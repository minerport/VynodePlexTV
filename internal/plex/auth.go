package plex

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LinkStart struct {
	ID        string `json:"id"`
	AuthURL   string `json:"authUrl"`
	ExpiresAt int64  `json:"expiresAt"`
}
type LinkConnection struct {
	URI      string `json:"uri"`
	Local    bool   `json:"local"`
	Relay    bool   `json:"relay"`
	Protocol string `json:"protocol"`
}
type LinkServer struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Owned       bool             `json:"owned"`
	Connections []LinkConnection `json:"connections"`
}
type LinkStatus struct {
	Status  string       `json:"status"`
	Servers []LinkServer `json:"servers"`
}
type linkState struct {
	userID    string
	pinID     int64
	code      string
	expires   int64
	lastPoll  int64
	token     string
	resources []accountResource
}
type accountResource struct {
	Name             string           `json:"name"`
	ClientIdentifier string           `json:"clientIdentifier"`
	Provides         string           `json:"provides"`
	AccessToken      string           `json:"accessToken"`
	Owned            bool             `json:"owned"`
	Connections      []LinkConnection `json:"connections"`
}
type pin struct {
	ID        int64  `json:"id"`
	Code      string `json:"code"`
	AuthToken string `json:"authToken"`
	ExpiresIn int64  `json:"expiresIn"`
}

// Account tokens and unclaimed PINs remain in bounded, expiring memory only.
// Only the chosen server's token is persisted through the encrypted server service.
func (s *Service) accountRequest(ctx context.Context, method, path, token string, form url.Values, target any) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, s.accountURL+path, body)
	if err != nil {
		return failure("INVALID", "Unable to start Plex sign-in")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Product", "Afterglow TV")
	req.Header.Set("X-Plex-Version", s.version)
	req.Header.Set("X-Plex-Platform", "Afterglow Server")
	req.Header.Set("X-Plex-Client-Identifier", s.vault.ClientID)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.Header.Set("X-Plex-Token", token)
	}
	response, err := s.accountHTTP.Do(req)
	if err != nil {
		return transportError(err)
	}
	defer response.Body.Close()
	if response.StatusCode == 401 || response.StatusCode == 403 {
		return failure("UNAUTHORIZED", "Plex account authorization was rejected. Start sign-in again.")
	}
	if response.StatusCode != 200 && response.StatusCode != 201 {
		return failure("UPSTREAM", "Plex account sign-in is temporarily unavailable. Try again.")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024+1))
	if err != nil || len(data) > 2*1024*1024 || json.Unmarshal(data, target) != nil {
		return failure("RESPONSE", "Plex account service returned an unexpected response")
	}
	return nil
}
func (s *Service) pruneLinksLocked() {
	for id, v := range s.links {
		if v.expires <= now() {
			delete(s.links, id)
		}
	}
}
func (s *Service) StartLink(ctx context.Context, userID string) (LinkStart, error) {
	s.linkMu.Lock()
	defer s.linkMu.Unlock()
	s.pruneLinksLocked()
	if len(s.links) >= 64 {
		return LinkStart{}, failure("CONFLICT", "Too many pending Plex sign-ins. Try again later.")
	}
	// One pending flow per administrator. Starting again invalidates the previous local flow.
	for id, v := range s.links {
		if v.userID == userID {
			delete(s.links, id)
		}
	}
	var p pin
	if err := s.accountRequest(ctx, http.MethodPost, "/api/v2/pins", "", url.Values{"strong": {"true"}}, &p); err != nil {
		return LinkStart{}, err
	}
	if p.ID <= 0 || p.Code == "" {
		return LinkStart{}, failure("RESPONSE", "Plex did not return a sign-in code")
	}
	ttl := p.ExpiresIn
	if ttl <= 0 || ttl > 600 {
		ttl = 600
	}
	id := newID()
	expires := now() + ttl
	s.links[id] = &linkState{userID: userID, pinID: p.ID, code: p.Code, expires: expires}
	authURL := "https://app.plex.tv/auth#?" + url.Values{"clientID": {s.vault.ClientID}, "code": {p.Code}, "context[device][product]": {"Afterglow TV"}}.Encode()
	return LinkStart{ID: id, AuthURL: authURL, ExpiresAt: expires}, nil
}
func (s *Service) PollLink(ctx context.Context, id, userID string) (LinkStatus, error) {
	s.linkMu.Lock()
	defer s.linkMu.Unlock()
	s.pruneLinksLocked()
	state := s.links[id]
	if state == nil || state.userID != userID {
		return LinkStatus{}, failure("NOT_FOUND", "Plex sign-in expired or belongs to another account. Start again.")
	}
	if state.resources != nil {
		return publicLinkStatus(state.resources), nil
	}
	if now()-state.lastPoll < 2 {
		return LinkStatus{Status: "WAITING", Servers: []LinkServer{}}, nil
	}
	state.lastPoll = now()
	if state.token == "" {
		var p pin
		path := "/api/v2/pins/" + strconv.FormatInt(state.pinID, 10) + "?" + url.Values{"code": {state.code}}.Encode()
		if err := s.accountRequest(ctx, http.MethodGet, path, "", nil, &p); err != nil {
			return LinkStatus{}, err
		}
		state.token = p.AuthToken
		if state.token == "" {
			return LinkStatus{Status: "WAITING", Servers: []LinkServer{}}, nil
		}
	}
	var resources []accountResource
	if err := s.accountRequest(ctx, http.MethodGet, "/api/v2/resources?includeHttps=1&includeRelay=1&includeIPv6=1", state.token, nil, &resources); err != nil {
		return LinkStatus{}, err
	}
	state.resources = make([]accountResource, 0, len(resources))
	for _, resource := range resources {
		if !strings.Contains(","+resource.Provides+",", ",server,") || resource.AccessToken == "" || resource.ClientIdentifier == "" {
			continue
		}
		valid := []LinkConnection{}
		for _, connection := range resource.Connections {
			base, err := NormalizeURL(connection.URI)
			if err != nil {
				continue
			}
			connection.URI = base
			valid = append(valid, connection)
		}
		sort.SliceStable(valid, func(i, j int) bool { return connectionRank(valid[i]) < connectionRank(valid[j]) })
		resource.Connections = valid
		state.resources = append(state.resources, resource)
	}
	state.token = ""
	state.code = ""
	return publicLinkStatus(state.resources), nil
}
func connectionRank(c LinkConnection) int {
	rank := 0
	if !c.Local {
		rank += 2
	}
	if c.Protocol != "https" {
		rank++
	}
	if c.Relay {
		rank += 4
	}
	return rank
}
func publicLinkStatus(resources []accountResource) LinkStatus {
	out := LinkStatus{Status: "AUTHORIZED", Servers: []LinkServer{}}
	for _, r := range resources {
		out.Servers = append(out.Servers, LinkServer{ID: r.ClientIdentifier, Name: r.Name, Owned: r.Owned, Connections: r.Connections})
	}
	return out
}
func (s *Service) CancelLink(id, userID string) error {
	s.linkMu.Lock()
	defer s.linkMu.Unlock()
	if state := s.links[id]; state != nil && state.userID == userID {
		delete(s.links, id)
		return nil
	}
	return failure("NOT_FOUND", "Plex sign-in is no longer active")
}
func (s *Service) ConnectLinkedServer(ctx context.Context, id, userID, resourceID, uri string) (Server, error) {
	s.linkMu.Lock()
	s.pruneLinksLocked()
	state := s.links[id]
	if state == nil || state.userID != userID {
		s.linkMu.Unlock()
		return Server{}, failure("NOT_FOUND", "Plex sign-in expired. Start again.")
	}
	var selected *accountResource
	for _, r := range state.resources {
		if r.ClientIdentifier == resourceID {
			copy := r
			selected = &copy
			break
		}
	}
	valid := false
	if selected != nil {
		for _, c := range selected.Connections {
			if c.URI == uri {
				valid = true
			}
		}
	}
	s.linkMu.Unlock()
	if !valid {
		return Server{}, failure("INVALID", "Choose one of the connections returned by Plex")
	}
	connection := Connection{BaseURL: uri, Token: selected.AccessToken, Name: selected.Name}
	identity, err := s.TestConnection(ctx, connection)
	if err != nil {
		return Server{}, err
	}
	if identity.MachineIdentifier != resourceID {
		return Server{}, failure("IDENTITY", "This connection responded as a different Plex server; nothing was saved")
	}
	servers, err := s.Servers(ctx)
	if err != nil {
		return Server{}, err
	}
	existingID := ""
	for _, server := range servers {
		if server.MachineIdentifier == resourceID {
			existingID = server.ID
		}
	}
	saved, err := s.SaveServer(ctx, existingID, connection, userID)
	if err != nil {
		return Server{}, err
	}
	s.linkMu.Lock()
	delete(s.links, id)
	s.linkMu.Unlock()
	return saved, nil
}

func newAccountHTTP() *http.Client {
	return &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}
