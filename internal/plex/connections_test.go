package plex

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestSecureConnectionDiscovery(t *testing.T) {
	s, f := newTestService(t)
	secure := httptest.NewTLSServer(f.server.Config.Handler)
	defer secure.Close()
	s.factory = func(base, token string) Provider {
		c := NewClient(base, token, "test", "test")
		c.http = secure.Client() // Trust only this test server's certificate.
		return c
	}
	resource := accountResource{ClientIdentifier: "fixture-server", AccessToken: f.token, Connections: []LinkConnection{{URI: "https://127.0.0.1:1"}, {URI: secure.URL}}}
	uri, err := s.findSecureConnection(context.Background(), resource)
	if err != nil || uri != secure.URL {
		t.Fatalf("discovery failed: %v", err)
	}
	resource.ClientIdentifier = "different-server"
	if _, err = s.findSecureConnection(context.Background(), resource); err == nil {
		t.Fatal("identity mismatch accepted")
	}
	resource.ClientIdentifier = "fixture-server"
	resource.AccessToken = "invalid-fixture"
	if _, err = s.findSecureConnection(context.Background(), resource); ErrorCode(err) != "UNAUTHORIZED" {
		t.Fatalf("access rejection lost: %v", err)
	}
	resource.Connections = []LinkConnection{{URI: f.server.URL, Protocol: "https"}}
	if _, err = s.findSecureConnection(context.Background(), resource); ErrorCode(err) != "CONNECTION" {
		t.Fatal("automatic HTTP downgrade accepted")
	}
}

func TestAutomaticLinkKeepsIdentityAndUserBoundary(t *testing.T) {
	s, f := newTestService(t)
	secure := httptest.NewTLSServer(f.server.Config.Handler)
	defer secure.Close()
	s.factory = func(base, token string) Provider {
		c := NewClient(base, token, "test", "test")
		c.http = secure.Client()
		return c
	}
	if _, err := s.db.Exec("INSERT INTO users VALUES('owner','owner','unused','OWNER',1)"); err != nil {
		t.Fatal(err)
	}
	s.links["flow"] = &linkState{userID: "owner", expires: now() + 60, resources: []accountResource{{Name: "Fixture", ClientIdentifier: "fixture-server", AccessToken: f.token, Connections: []LinkConnection{{URI: secure.URL}}}}}
	if _, err := s.ConnectLinkedServer(context.Background(), "flow", "other", "fixture-server", ""); ErrorCode(err) != "NOT_FOUND" {
		t.Fatal("cross-user auto connect allowed")
	}
	saved, err := s.ConnectLinkedServer(context.Background(), "flow", "owner", "fixture-server", "")
	if err != nil {
		t.Fatal(err)
	}
	if saved.BaseURL != secure.URL {
		t.Fatal("wrong endpoint persisted")
	}
	if _, exists := s.links["flow"]; exists {
		t.Fatal("completed flow retained")
	}
}

func TestAutomaticLocalHTTPFallback(t *testing.T) {
	s, f := newTestService(t)
	resource := accountResource{ClientIdentifier: "fixture-server", AccessToken: f.token, Connections: []LinkConnection{{URI: "https://127.0.0.1:1"}, {URI: f.server.URL, Local: true}}}
	uri, err := s.findSecureConnection(context.Background(), resource)
	if err != nil || uri != f.server.URL {
		t.Fatalf("local fallback: %v", err)
	}
	for _, c := range []LinkConnection{{URI: "http://8.8.8.8:32400", Local: true}, {URI: "http://plex.example.com", Local: true}, {URI: f.server.URL}, {URI: f.server.URL, Local: true, Relay: true}} {
		if localHTTP(c) {
			t.Fatal("unsafe automatic HTTP candidate accepted")
		}
	}
}
