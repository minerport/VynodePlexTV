package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
)

func TestAuthenticationAndPersistence(t *testing.T) {
	dir := t.TempDir()
	db, err := database.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := New(db, config.Config{Origin: "http://localhost:8080", WebDir: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	handler := a.Handler()
	call := func(method, path, body, origin, csrf string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	origin := "http://localhost:8080"
	creds := `{"username":"owner","password":"correct-horse-battery"}`
	assert := func(w *httptest.ResponseRecorder, want int) {
		t.Helper()
		if w.Code != want {
			t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
		}
	}
	assert(call("GET", "/api/v1/health", "", "", "", nil), 200)
	assert(call("GET", "/api/v1/ready", "", "", "", nil), 200)
	assert(call("GET", "/api/v1/settings", "", "", "", nil), 401)
	assert(call("POST", "/api/v1/auth/setup", creds, "http://evil.test", "", nil), 403)
	assert(call("POST", "/api/v1/auth/setup", `{"username":"owner","password":"short"}`, origin, "", nil), 400)
	assert(call("POST", "/api/v1/auth/setup", creds, origin, "", nil), 201)
	assert(call("POST", "/api/v1/auth/setup", creds, origin, "", nil), 409)
	assert(call("POST", "/api/v1/auth/login", `{"username":"owner","password":"incorrect"}`, origin, "", nil), 401)
	w := call("POST", "/api/v1/auth/login", creds, origin, "", nil)
	assert(w, 200)
	var session Session
	if err = json.Unmarshal(w.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 86400 {
		t.Fatal("insecure session cookie")
	}
	var stored string
	if err = db.QueryRow("SELECT token_hash FROM sessions").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == cookie.Value {
		t.Fatal("raw session token stored")
	}
	assert(call("PUT", "/api/v1/settings", `{"displayName":"Movie room","timezone":"America/New_York"}`, origin, "", cookie), 403)
	assert(call("PUT", "/api/v1/settings", `{"displayName":"Movie room","timezone":"invalid"}`, origin, session.CSRF, cookie), 400)
	assert(call("PUT", "/api/v1/settings", `{"displayName":"Movie room","timezone":"America/New_York"}`, origin, session.CSRF, cookie), 200)
	if _, err = db.Exec("UPDATE users SET role='USER'"); err != nil {
		t.Fatal(err)
	}
	assert(call("PUT", "/api/v1/settings", `{"displayName":"Denied","timezone":"UTC"}`, origin, session.CSRF, cookie), 403)
	if _, err = db.Exec("UPDATE users SET role='OWNER'"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = database.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a.db = db
	w = call("GET", "/api/v1/settings", "", "", "", cookie)
	assert(w, 200)
	if !bytes.Contains(w.Body.Bytes(), []byte("Movie room")) {
		t.Fatal("settings did not persist")
	}
	assert(call("POST", "/api/v1/auth/logout", "", origin, session.CSRF, cookie), 200)
	assert(call("GET", "/api/v1/auth/session", "", "", "", cookie), 401)
	w = call("POST", "/api/v1/auth/login", creds, origin, "", nil)
	assert(w, 200)
	cookie = w.Result().Cookies()[0]
	if _, err = db.Exec("UPDATE sessions SET expires_at=?", time.Now().Add(-time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	assert(call("GET", "/api/v1/auth/session", "", "", "", cookie), 401)
	var audits int
	if err = db.QueryRow("SELECT COUNT(*) FROM audit_log").Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if audits != 2 {
		t.Fatalf("audit count %d", audits)
	}
}

func TestConcurrentOwnerSetup(t *testing.T) {
	db, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(db, config.Config{Origin: "http://localhost:8080"}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("POST", "/api/v1/auth/setup", bytes.NewBufferString(`{"username":"owner","password":"correct-horse-battery"}`))
			r.Header.Set("Origin", "http://localhost:8080")
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			a.Handler().ServeHTTP(w, r)
			results <- w.Code
		}()
	}
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[201] != 1 || counts[409] != 1 {
		t.Fatalf("unexpected setup statuses: %v", counts)
	}
}

func TestRateLimitAndRequestValidation(t *testing.T) {
	db, err := database.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	a := New(db, config.Config{Origin: "http://localhost:8080"}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	for i := 0; i < 21; i++ {
		r := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(`{} {}`))
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		want := 400
		if i == 20 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("request %d: status %d", i, w.Code)
		}
	}
}
