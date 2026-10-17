package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"afterglow.local/tv/internal/plex"
)

func TestPlexAuthorizationAndTokenProtection(t *testing.T) {
	dir := t.TempDir()
	db, err := database.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service, err := plex.NewService(db, dir, "test", logger)
	if err != nil {
		t.Fatal(err)
	}
	a := New(db, config.Config{Origin: "http://localhost:8080"}, logger, "test")
	a.MountPlex(service)
	handler := a.Handler()
	secret, err := randomToken()
	if err != nil {
		t.Fatal(err)
	}
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != secret {
			w.WriteHeader(401)
			return
		}
		switch r.URL.Path {
		case "/":
			io.WriteString(w, `<MediaContainer machineIdentifier="api-fixture" friendlyName="API Fixture"/>`)
		case "/library/sections":
			io.WriteString(w, `<MediaContainer size="1"><Directory key="1" title="Movies" type="movie"/></MediaContainer>`)
		default:
			w.Write([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 100)))
		}
	}))
	defer remote.Close()
	call := func(method, path, body, token string, csrf bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:8080")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: "afterglow_session", Value: token})
		}
		if csrf {
			r.Header.Set("X-CSRF-Token", "test-csrf")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	adminToken, _ := randomToken()
	userToken, _ := randomToken()
	for _, u := range []struct{ id, role, token string }{{"owner", "OWNER", adminToken}, {"viewer", "USER", userToken}} {
		if _, err = db.Exec("INSERT INTO users VALUES(?,?,? ,?,?)", u.id, u.id, "unused", u.role, time.Now().Unix()); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec("INSERT INTO sessions VALUES(?,?,?,?)", digest(u.token), u.id, "test-csrf", time.Now().Add(time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	routes := []struct{ method, path string }{{"GET", "/api/v1/plex/servers"}, {"POST", "/api/v1/plex/servers"}, {"PATCH", "/api/v1/plex/servers/missing"}, {"DELETE", "/api/v1/plex/servers/missing"}, {"GET", "/api/v1/jobs"}, {"GET", "/api/v1/media"}, {"GET", "/api/v1/media/item/artwork/poster"}, {"POST", "/api/v1/plex/libraries/library/sync"}, {"PUT", "/api/v1/plex/settings"}}
	for _, route := range routes {
		if w := call(route.method, route.path, "{}", "", true); w.Code != 401 {
			t.Fatalf("anonymous route %s: %d", route.path, w.Code)
		}
		if w := call(route.method, route.path, "{}", userToken, true); w.Code != 403 {
			t.Fatalf("viewer route %s: %d", route.path, w.Code)
		}
	}
	body, _ := json.Marshal(plex.Connection{BaseURL: remote.URL, Token: secret})
	if w := call("POST", "/api/v1/plex/servers", string(body), adminToken, false); w.Code != 403 {
		t.Fatal("missing CSRF accepted")
	}
	w := call("POST", "/api/v1/plex/servers", string(body), adminToken, true)
	if w.Code != 201 {
		t.Fatalf("server add: %d %s", w.Code, w.Body.String())
	}
	if bytes.Contains(w.Body.Bytes(), []byte(secret)) {
		t.Fatal("secret in create response")
	}
	var server plex.Server
	if err = json.Unmarshal(w.Body.Bytes(), &server); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/plex/servers", "/api/v1/plex/servers/" + server.ID} {
		w = call("GET", path, "", adminToken, false)
		if w.Code != 200 || strings.Contains(w.Body.String(), secret) || strings.Contains(w.Body.String(), "token_encrypted") {
			t.Fatal("server response leaked secret")
		}
	}
	if _, err = db.Exec(`INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,created_at,updated_at) VALUES('lib',?,'1','Movies','movie',1,1)`, server.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,thumb_path,art_path,created_at,updated_at) VALUES('item',?,'lib','1','movie','Movie','/library/metadata/1/thumb/1','/library/metadata/1/art/1',1,1)`, server.ID); err != nil {
		t.Fatal(err)
	}
	w = call("GET", "/api/v1/media/item/artwork/poster", "", adminToken, false)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || strings.Contains(w.Body.String(), secret) {
		t.Fatal("artwork proxy failed")
	}
	if strings.Contains(w.Header().Get("ETag"), secret) {
		t.Fatal("secret in artwork header")
	}
	if _, err = service.Item(context.Background(), "item"); err != nil {
		t.Fatal(err)
	}
}
