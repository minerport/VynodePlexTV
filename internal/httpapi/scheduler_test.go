package httpapi

import (
	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSchedulerAPIAndRoles(t *testing.T) {
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
		r := httptest.NewRequest(method, "/api/v1"+path, bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:8080")
		r.Header.Set("X-CSRF-Token", csrf)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	call("GET", "/scheduler", "", 401)
	creds := `{"username":"owner","password":"test-only-long-password"}`
	call("POST", "/auth/setup", creds, 201)
	w := call("POST", "/auth/login", creds, 200)
	cookie = w.Result().Cookies()[0]
	var session Session
	json.Unmarshal(w.Body.Bytes(), &session)
	csrf = session.CSRF
	w = call("POST", "/channels", `{"number":"101","name":"Test","enabled":true,"visibility":"VISIBLE","logo":{"source":"AUTO"},"background":{"source":"AUTO"},"source":{"type":"STATIC","static":{"mediaIds":[]}}}`, 201)
	var c struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &c)
	base := "/channels/" + c.ID + "/schedule"
	settings := call("GET", base+"/settings", "", 200).Body.String()
	for _, role := range []string{"USER", "ADMIN", "OWNER"} {
		db.Exec("UPDATE users SET role=?", role)
		call("GET", base+"?day=2026-10-17", "", 200)
		call("GET", base+"/at?time=2026-10-17T12:00:00Z", "", 200)
		want := 202
		save := 200
		if role == "USER" {
			want = 403
			save = 403
		}
		call("PATCH", base+"/settings", settings, save)
		for _, action := range []string{"generate", "repair", "regenerate"} {
			call("POST", base+"/"+action, "{}", want)
			if role != "USER" {
				if _, e = a.Scheduler().RunNext(context.Background()); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	call("GET", "/scheduler", "", 200)
	call("GET", "/scheduler/jobs", "", 200)
	call("GET", base+"/status", "", 200)
	call("GET", base+"?day=invalid", "", 422)
	call("GET", base+"/at?time=invalid", "", 422)
	csrf = ""
	call("POST", base+"/generate", "{}", 403)
	db.Exec("UPDATE channels SET enabled=0")
	db.Exec("UPDATE users SET role='USER'")
	call("GET", base+"?day=2026-10-17", "", 404)
}
