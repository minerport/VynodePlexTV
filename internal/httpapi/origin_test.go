package httpapi

import (
	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAutomaticOrigin(t *testing.T) {
	for _, tc := range []struct {
		origin, host string
		want         bool
	}{
		{"http://10.0.0.86:8475", "10.0.0.86:8475", true},
		{"http://tower:8475", "tower:8475", true},
		{"http://evil.test", "10.0.0.86:8475", false},
		{"null", "10.0.0.86:8475", false},
		{"", "10.0.0.86:8475", false},
		{"https://tower:8475", "tower:8475", false},
	} {
		r := httptest.NewRequest("POST", "http://"+tc.host+"/api/v1/auth/setup", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("X-Forwarded-Host", "evil.test")
		if got := allowedOrigin(r, "auto"); got != tc.want {
			t.Errorf("%q: got %v", tc.origin, got)
		}
	}
}

func TestAutomaticOriginOwnerSetup(t *testing.T) {
	db, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(db, config.Config{Origin: "auto"}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	for _, tc := range []struct {
		path, origin string
		status       int
	}{
		{"setup", "http://evil.test", 403},
		{"setup", "http://10.0.0.86:8475", 201},
		{"login", "http://10.0.0.86:8475", 200},
	} {
		r := httptest.NewRequest("POST", "http://10.0.0.86:8475/api/v1/auth/"+tc.path, strings.NewReader(`{"username":"fixture","password":"synthetic-test-password"}`))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
}
