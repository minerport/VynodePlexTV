package plex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlexAccountLinkingAndSecretBoundary(t *testing.T) {
	s, f := newTestService(t)
	ctx := context.Background()
	if _, err := s.db.Exec("INSERT INTO users VALUES('admin','admin','unused','OWNER',1)"); err != nil {
		t.Fatal(err)
	}
	accountToken := newID()
	code := newID()
	claimed := false
	account := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("X-Plex-Client-Identifier") != s.vault.ClientID {
			t.Error("client identifier missing")
		}
		switch r.URL.Path {
		case "/api/v2/pins":
			if r.Method != "POST" || r.FormValue("strong") != "true" {
				t.Error("weak PIN requested")
			}
			json.NewEncoder(w).Encode(pin{ID: 55, Code: code, ExpiresIn: 600})
		case "/api/v2/pins/55":
			if r.URL.Query().Get("code") != code {
				t.Error("wrong PIN proof")
			}
			p := pin{ID: 55}
			if claimed {
				p.AuthToken = accountToken
			}
			json.NewEncoder(w).Encode(p)
		case "/api/v2/resources":
			if r.Header.Get("X-Plex-Token") != accountToken {
				t.Error("missing account credential")
			}
			json.NewEncoder(w).Encode([]accountResource{{Name: "Cinema", ClientIdentifier: "fixture-server", Provides: "server", AccessToken: f.token, Owned: true, Connections: []LinkConnection{{URI: f.server.URL, Local: true, Protocol: "http"}}}})
		default:
			w.WriteHeader(404)
		}
	}))
	defer account.Close()
	s.accountURL = account.URL
	start, err := s.StartLink(ctx, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(start.AuthURL, "https://app.plex.tv/auth#?") {
		t.Fatal("invalid auth URL")
	}
	if _, err = s.PollLink(ctx, start.ID, "other"); ErrorCode(err) != "NOT_FOUND" {
		t.Fatal("cross-user polling allowed")
	}
	status, err := s.PollLink(ctx, start.ID, "admin")
	if err != nil || status.Status != "WAITING" {
		t.Fatal("pending PIN failed")
	}
	claimed = true
	s.links[start.ID].lastPoll = 0
	status, err = s.PollLink(ctx, start.ID, "admin")
	if err != nil || status.Status != "AUTHORIZED" || len(status.Servers) != 1 {
		t.Fatalf("resource discovery failed: %v", err)
	}
	raw, _ := json.Marshal(status)
	if strings.Contains(string(raw), f.token) || strings.Contains(string(raw), accountToken) {
		t.Fatal("token in public linking response")
	}
	if s.links[start.ID].token != "" || s.links[start.ID].code != "" {
		t.Fatal("account credential retained after resource discovery")
	}
	if _, err = s.ConnectLinkedServer(ctx, start.ID, "admin", "fixture-server", "http://arbitrary.invalid"); ErrorCode(err) != "INVALID" {
		t.Fatal("unlisted connection allowed")
	}
	server, err := s.ConnectLinkedServer(ctx, start.ID, "admin", "fixture-server", f.server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if server.MachineIdentifier != "fixture-server" {
		t.Fatal("wrong server saved")
	}
	if len(s.links) != 0 {
		t.Fatal("link secrets retained after connection")
	}
	if err = s.health(ctx, server.ID); err != nil {
		t.Fatal("saved scoped token unusable")
	}
}
func TestLinkExpiryAndCancellation(t *testing.T) {
	s, _ := newTestService(t)
	s.links["expired"] = &linkState{userID: "admin", expires: now() - 1, token: newID()}
	if _, err := s.PollLink(context.Background(), "expired", "admin"); ErrorCode(err) != "NOT_FOUND" {
		t.Fatal("expired link accepted")
	}
	if len(s.links) != 0 {
		t.Fatal("expired secrets retained")
	}
	s.links["active"] = &linkState{userID: "admin", expires: now() + 60}
	if err := s.CancelLink("active", "other"); ErrorCode(err) != "NOT_FOUND" {
		t.Fatal("cross-user cancellation")
	}
	if err := s.CancelLink("active", "admin"); err != nil {
		t.Fatal(err)
	}
}
