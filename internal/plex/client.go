package plex

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const PageSize = 100

func sourceRevision(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

type Client struct {
	base, token, clientID, version string
	http                           *http.Client
}

func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" || strings.ContainsAny(u.Path, "\\\r\n") {
		return "", failure("INVALID", "Enter an HTTP or HTTPS server URL without credentials, a query, or fragment")
	}
	for _, s := range strings.Split(u.Path, "/") {
		if s == ".." || s == "." {
			return "", failure("INVALID", "Server URL must not contain relative path segments")
		}
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimRight(u.Path, "/")
	return strings.TrimRight(u.String(), "/"), nil
}

// plexDialContext handles resolvers that reject Plex's private-address DNS records.
// Only the canonical IPv4.<32 hex certificate id>.plex.direct format is eligible,
// and only after a DNS failure. HTTP Host, TLS SNI and certificate verification
// continue to use the original hostname; this does not permit insecure TLS.
func plexDialContext(dial func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dial(ctx, network, address)
		var dns *net.DNSError
		if err == nil || !errors.As(err, &dns) || ctx.Err() != nil {
			return conn, err
		}
		host, port, splitErr := net.SplitHostPort(address)
		portNumber, portErr := strconv.Atoi(port)
		labels := strings.Split(strings.ToLower(host), ".")
		if splitErr != nil || portErr != nil || portNumber < 1 || portNumber > 65535 || len(labels) != 4 || labels[2] != "plex" || labels[3] != "direct" || len(labels[1]) != 32 {
			return nil, err
		}
		for _, c := range labels[1] {
			if !strings.ContainsRune("0123456789abcdef", c) {
				return nil, err
			}
		}
		ip, parseErr := netip.ParseAddr(strings.ReplaceAll(labels[0], "-", "."))
		if parseErr != nil || !ip.Is4() || ip.IsUnspecified() || ip.IsMulticast() {
			return nil, err
		}
		return dial(ctx, network, net.JoinHostPort(ip.String(), port))
	}
}

func NewClient(base, token, clientID, version string) *Client {
	return &Client{base: base, token: token, clientID: clientID, version: version, http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: plexDialContext((&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext), TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 5 * time.Second, ResponseHeaderTimeout: 15 * time.Second, IdleConnTimeout: 60 * time.Second, MaxIdleConns: 4, MaxIdleConnsPerHost: 2}}}
}
func (c *Client) get(ctx context.Context, path string, query url.Values, etag, lastModified string) ([]byte, http.Header, int, error) {
	target := c.base + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
		if err != nil {
			return nil, nil, 0, failure("INVALID", "Invalid Plex request")
		}
		req.Header.Set("X-Plex-Token", c.token)
		req.Header.Set("X-Plex-Product", "Afterglow TV")
		req.Header.Set("X-Plex-Version", c.version)
		req.Header.Set("X-Plex-Client-Identifier", c.clientID)
		req.Header.Set("X-Plex-Platform", "Afterglow Server")
		req.Header.Set("Accept", "application/xml")
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if lastModified != "" {
			req.Header.Set("If-Modified-Since", lastModified)
		}
		response, err := c.http.Do(req)
		if err != nil {
			safe := transportError(err)
			if ctx.Err() != nil {
				return nil, nil, 0, ctx.Err()
			}
			if attempt == 0 && (ErrorCode(safe) == "OFFLINE" || ErrorCode(safe) == "TIMEOUT") {
				if err = backoff(ctx); err == nil {
					continue
				}
			}
			return nil, nil, 0, safe
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 16*1024*1024+1))
		response.Body.Close()
		status := response.StatusCode
		if status == 401 || status == 403 {
			return nil, nil, status, failure("UNAUTHORIZED", "Plex rejected the token. Check access and update the credential.")
		}
		if status >= 500 && attempt == 0 {
			if err = backoff(ctx); err != nil {
				return nil, nil, status, err
			}
			continue
		}
		if status != 200 && status != 304 {
			if status == 404 {
				return nil, nil, status, failure("NOT_FOUND", "The requested Plex endpoint or media is unavailable")
			}
			return nil, nil, status, failure("UPSTREAM", "Plex endpoint is unavailable or redirected. Check the server URL.")
		}
		if readErr != nil || len(data) > 16*1024*1024 {
			return nil, nil, status, failure("RESPONSE", "Plex returned an incomplete or oversized response")
		}
		return data, response.Header, status, nil
	}
	return nil, nil, 0, failure("OFFLINE", "Plex is unavailable")
}
func backoff(ctx context.Context) error {
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func transportError(err error) error {
	var dns *net.DNSError
	var cert *tls.CertificateVerificationError
	var unknown x509.UnknownAuthorityError
	var network net.Error
	switch {
	case errors.As(err, &cert) || errors.As(err, &unknown):
		return failure("TLS", "Plex TLS certificate validation failed. Use a trusted certificate and matching hostname.")
	case errors.As(err, &dns):
		return failure("DNS", "Plex hostname could not be resolved")
	case errors.As(err, &network) && network.Timeout():
		return failure("TIMEOUT", "Plex did not respond before the timeout")
	default:
		return failure("OFFLINE", "Cannot reach Plex. Check its address, network, and server status.")
	}
}
func (c *Client) container(ctx context.Context, path string, q url.Values) (xmlContainer, error) {
	var container xmlContainer
	data, _, _, err := c.get(ctx, path, q, "", "")
	if err != nil {
		return container, err
	}
	// Upstream fields are untrusted. Never propagate an echoed credential into local records.
	if c.token != "" {
		data = bytes.ReplaceAll(data, []byte(c.token), []byte("[redacted]"))
	}
	if err = xml.Unmarshal(data, &container); err != nil || container.XMLName.Local != "MediaContainer" {
		return container, failure("RESPONSE", "Plex returned an unexpected response instead of library metadata")
	}
	return container, nil
}
func (c *Client) Identity(ctx context.Context) (Identity, error) {
	root, err := c.container(ctx, "/", nil)
	if err != nil {
		return Identity{}, err
	}
	if root.MachineIdentifier == "" {
		return Identity{}, failure("RESPONSE", "The endpoint did not identify itself as a Plex Media Server")
	}
	return Identity{MachineIdentifier: root.MachineIdentifier, FriendlyName: root.FriendlyName, Product: root.Product, Version: root.Version, Platform: root.Platform, PlatformVersion: root.PlatformVersion, Owned: root.Owned, SecureConnections: root.Secure}, nil
}
func (c *Client) Libraries(ctx context.Context) ([]Library, error) {
	root, err := c.container(ctx, "/library/sections", nil)
	if err != nil {
		return nil, err
	}
	if root.Size == nil || *root.Size != len(root.Directories) {
		return nil, failure("RESPONSE", "Plex library discovery was incomplete")
	}
	out := make([]Library, 0, len(root.Directories))
	seen := map[string]bool{}
	for _, d := range root.Directories {
		if d.Key == "" || d.Title == "" || d.Type == "" || seen[d.Key] {
			return nil, failure("RESPONSE", "Plex returned invalid library identities")
		}
		seen[d.Key] = true
		out = append(out, Library{Key: d.Key, Title: d.Title, Type: d.Type, Agent: d.Agent, Scanner: d.Scanner, Language: d.Language, UUID: d.UUID, Revision: sourceRevision(d.Updated) + ":" + d.ScannedAt, Supported: d.Type == "movie" || d.Type == "show", Present: true})
	}
	return out, nil
}
func (c *Client) Page(ctx context.Context, section string, kind, offset int) (Page, error) {
	q := url.Values{"type": {strconv.Itoa(kind)}, "X-Plex-Container-Start": {strconv.Itoa(offset)}, "X-Plex-Container-Size": {strconv.Itoa(PageSize)}, "sort": {"titleSort:asc"}, "includeGuids": {"1"}}
	root, err := c.container(ctx, "/library/sections/"+url.PathEscape(section)+"/all", q)
	if err != nil {
		return Page{}, err
	}
	nodes := append(root.Videos, root.Directories...)
	if root.Size == nil || *root.Size != len(nodes) || len(nodes) > PageSize || (root.Offset != nil && *root.Offset != offset) {
		return Page{}, failure("RESPONSE", "Plex pagination was inconsistent; inventory reconciliation was skipped")
	}
	p := Page{Total: root.TotalSize, Offset: offset, Items: make([]MediaItem, 0, len(nodes))}
	for _, n := range nodes {
		p.Items = append(p.Items, n.model())
	}
	return p, nil
}
func (c *Client) Metadata(ctx context.Context, key string) (MediaItem, error) {
	root, err := c.container(ctx, "/library/metadata/"+url.PathEscape(key), url.Values{"includeGuids": {"1"}, "includeExtras": {"0"}})
	if err != nil {
		return MediaItem{}, err
	}
	nodes := append(root.Videos, root.Directories...)
	if len(nodes) != 1 || nodes[0].RatingKey != key {
		return MediaItem{}, failure("RESPONSE", "Plex item identity changed during synchronization")
	}
	return nodes[0].model(), nil
}
func (c *Client) Artwork(ctx context.Context, path, etag, modified string) (Artwork, error) {
	if !safeArtworkPath(path) {
		return Artwork{}, failure("NOT_FOUND", "Artwork is not available for this item")
	}
	data, headers, status, err := c.get(ctx, path, nil, etag, modified)
	if err != nil {
		return Artwork{}, err
	}
	if status == 304 {
		return Artwork{NotModified: true}, nil
	}
	content := http.DetectContentType(data)
	if content != "image/jpeg" && content != "image/png" && content != "image/webp" && content != "image/gif" {
		return Artwork{}, failure("RESPONSE", "Plex did not return supported image artwork")
	}
	if bytes.Contains(data, []byte(c.token)) && c.token != "" {
		return Artwork{}, failure("RESPONSE", "Invalid artwork response")
	}
	return Artwork{Data: data, ContentType: content, ETag: headers.Get("ETag"), LastModified: headers.Get("Last-Modified")}, nil
}
func safeArtworkPath(path string) bool {
	u, err := url.Parse(path)
	return err == nil && strings.HasPrefix(path, "/library/") && !strings.HasPrefix(path, "//") && !u.IsAbs() && u.Host == "" && u.RawQuery == "" && u.Fragment == "" && !strings.Contains(u.Path, "..") && !strings.ContainsAny(u.Path, "\\\r\n")
}
