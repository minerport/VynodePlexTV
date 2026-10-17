package httpapi

import (
	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChannelAuthorizationValidationAndLifecycle(t *testing.T) {
	dir := t.TempDir()
	db, e := database.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	a := New(db, config.Config{Dir: dir, Origin: "http://localhost:8080", WebDir: t.TempDir()}, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	h := a.Handler()
	var cookie *http.Cookie
	csrf := ""
	call := func(method, path, body string, want int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, w.Code, want, w.Body.String())
		}
		return w
	}
	for _, path := range []string{"/channels", "/channel-groups", "/channel-metadata/show", "/channel-artwork/missing", "/channels/missing/artwork/logo"} {
		call("GET", "/api/v1"+path, "", 401)
	}
	creds := `{"username":"test-owner","password":"test-only-long-password"}`
	call("POST", "/api/v1/auth/setup", creds, 201)
	w := call("POST", "/api/v1/auth/login", creds, 200)
	cookie = w.Result().Cookies()[0]
	var session Session
	if e = json.Unmarshal(w.Body.Bytes(), &session); e != nil {
		t.Fatal(e)
	}
	csrf = session.CSRF
	body := `{"number":"101","name":"Test channel","enabled":true,"visibility":"VISIBLE","logo":{"source":"AUTO"},"background":{"source":"AUTO"},"source":{"type":"STATIC","static":{"mediaIds":[]}}}`
	for _, role := range []string{"USER", "ADMIN", "OWNER"} {
		if _, e = db.Exec("UPDATE users SET role=?", role); e != nil {
			t.Fatal(e)
		}
		if role == "USER" {
			call("POST", "/api/v1/channels", body, 403)
			for _, path := range []string{"/channels", "/channel-groups", "/channel-metadata/show", "/channel-artwork/missing"} {
				call("GET", "/api/v1"+path, "", 403)
			}
			continue
		}
		call("GET", "/api/v1/channels", "", 200)
	}
	saved := csrf
	csrf = ""
	call("POST", "/api/v1/channels", body, 403)
	csrf = saved
	w = call("POST", "/api/v1/channels", body, 201)
	var c struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &c)
	if c.ID == "" {
		t.Fatal("no id")
	}
	call("POST", "/api/v1/channels", body, 409)
	call("GET", "/api/v1/channels/"+c.ID+"/media", "", 200)
	call("PATCH", "/api/v1/channels/"+c.ID+"/enabled", `{"enabled":false}`, 200)
	call("POST", "/api/v1/channels/"+c.ID+"/duplicate", "", 201)
	call("POST", "/api/v1/channel-rules/validate", `{"version":999,"root":{"type":"condition","field":"year","operator":"equals","value":2000}}`, 422)
	call("POST", "/api/v1/channel-preview", `{"source":{"type":"RULE","rule":{"version":1,"root":{"type":"condition","field":"year;DROP TABLE users","operator":"equals","value":2000}}}}`, 422)
	call("POST", "/api/v1/channels", `{"unknown":"value"}`, 400)
	call("GET", "/api/v1/channels/"+c.ID+"/artwork/logo", "", 200)
	call("DELETE", "/api/v1/channels/"+c.ID, "", 200)
	call("GET", "/api/v1/channels/"+c.ID, "", 404)
	var count int
	if e = db.QueryRow("SELECT count(*) FROM audit_log WHERE target_id<>'' AND user_id IS NOT NULL").Scan(&count); e != nil || count < 4 {
		t.Fatal("missing audit", count, e)
	}
}
