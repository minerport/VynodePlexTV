package plex

import (
	"context"
	"net/url"
	"time"
)

// Probe only account-advertised HTTPS endpoints. Credentials never follow a
// redirect or an implicit downgrade. Limit concurrency and total discovery time.
func (s *Service) findSecureConnection(ctx context.Context, resource accountResource) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 22*time.Second)
	defer cancel()
	candidates := make(chan string, len(resource.Connections))
	seen := map[string]bool{}
	// Try a remote route early so stale local interfaces cannot starve it.
	connections := append([]LinkConnection(nil), resource.Connections...)
	for i, c := range connections {
		if !c.Local && !c.Relay && i > 1 {
			copy(connections[2:i+1], connections[1:i])
			connections[1] = c
			break
		}
	}
	for _, c := range connections {
		u, err := url.Parse(c.URI)
		if err == nil && u.Scheme == "https" && !seen[c.URI] {
			seen[c.URI] = true
			candidates <- c.URI
		}
	}
	close(candidates)
	count := len(candidates)
	if count == 0 {
		return "", failure("CONNECTION", "Plex advertised no secure connections. Enable secure connections in Plex, or explicitly choose an HTTP address on a trusted network.")
	}
	type result struct {
		uri  string
		code string
	}
	results := make(chan result, count)
	workers := min(4, count)
	for i := 0; i < workers; i++ {
		go func() {
			for uri := range candidates {
				if ctx.Err() != nil {
					return
				}
				probe, stop := context.WithTimeout(ctx, 6*time.Second)
				identity, err := s.TestConnection(probe, Connection{BaseURL: uri, Token: resource.AccessToken})
				stop()
				code := ""
				if err != nil {
					code = ErrorCode(err)
				}
				if err == nil && identity.MachineIdentifier != resource.ClientIdentifier {
					code = "IDENTITY"
				}
				if code == "" {
					results <- result{uri: uri}
					return
				}
				results <- result{code: code}
			}
		}()
	}
	unauthorized := false
	for i := 0; i < count; i++ {
		select {
		case r := <-results:
			if r.uri != "" {
				return r.uri, nil
			}
			unauthorized = unauthorized || r.code == "UNAUTHORIZED"
		case <-ctx.Done():
			if ctx.Err() != nil {
				return "", failure("CONNECTION", "Secure connection discovery timed out. Check that Plex is running and reachable from this Afterglow device, or choose a connection manually.")
			}
		}
	}
	if unauthorized {
		return "", failure("UNAUTHORIZED", "Plex rejected account access. Check library sharing permissions and sign in again.")
	}
	return "", failure("CONNECTION", "No advertised secure Plex connection was reachable and verified. Check Plex remote access, firewall or VPN settings, or choose a connection manually. No credentials were saved.")
}
