package httpapi

import (
	"afterglow.local/tv/internal/config"
	"afterglow.local/tv/internal/database"
	"afterglow.local/tv/internal/playback"
	"afterglow.local/tv/internal/plex"
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
)

type testPlaybackAdapter struct{}
type testPlaybackStream struct{}

func (testPlaybackStream) Fetch(context.Context, string, string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"video/mp4"}}, Body: io.NopCloser(strings.NewReader("fixture-video"))}, nil
}
func (testPlaybackStream) Close(context.Context) error { return nil }
func (testPlaybackAdapter) ResolvePlayback(context.Context, string, plex.Capabilities, string) (plex.PlaybackResolution, error) {
	return plex.PlaybackResolution{Source: testPlaybackStream{}, Duration: 3600000, ServerID: "s", Mode: "DIRECT_PLAY", Mime: "video/mp4"}, nil
}
func (testPlaybackAdapter) StopPlayback(context.Context, string, string, string) error { return nil }
func TestPlaybackHTTPAuthorizationAndLogout(t *testing.T) {
	dir := t.TempDir()
	db, e := database.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a := New(db, config.Config{Dir: dir, Origin: "http://localhost:8080", WebDir: t.TempDir()}, logger, "test")
	a.playback = playback.New(db, a.scheduler, testPlaybackAdapter{}, logger)
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
	for _, p := range []string{"/on-now", "/playback/active", "/playback/id/stream/main", "/watch/channels/id/artwork/logo", "/guide/on-now", "/users/me/favorites"} {
		call("GET", p, "", 401)
	}
	creds := `{"username":"owner","password":"test-long-password"}`
	call("POST", "/auth/setup", creds, 201)
	w := call("POST", "/auth/login", creds, 200)
	cookie = w.Result().Cookies()[0]
	var auth Session
	json.Unmarshal(w.Body.Bytes(), &auth)
	csrf = auth.CSRF
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO plex_servers(id,name,base_url,machine_identifier,token_encrypted,token_key_version,created_at,updated_at) VALUES('s','Fixture','http://unused.invalid','s',x'00',1,1,1)`)
	exec(`INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,selected,created_at,updated_at) VALUES('l','s','1','Movies','movie',1,1,1)`)
	exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,duration_ms,created_at,updated_at) VALUES('m','s','l','1','movie','Movie',3600000,1,1)`)
	w = call("POST", "/channels", `{"number":"101","name":"Test","enabled":true,"visibility":"VISIBLE","logo":{"source":"AUTO"},"background":{"source":"AUTO"},"source":{"type":"STATIC","static":{"mediaIds":["m"]}}}`, 201)
	var channel struct{ ID string }
	json.Unmarshal(w.Body.Bytes(), &channel)
	start := time.Now().Add(-10 * time.Minute).UnixMilli()
	exec(`INSERT INTO channel_programs VALUES('p',?,'m','m',?,?,3600000,0,'g',1,'test',1)`, channel.ID, start, start+3600000)
	body := `{"channelId":"` + channel.ID + `","mode":"LIVE","clientId":"browser","capabilities":{"clientType":"browser"}}`
	exec("UPDATE users SET role='USER'")
	call("GET", "/playback/active", "", 403)
	call("GET", "/on-now", "", 200)
	call("GET", "/guide/on-now", "", 200)
	call("GET", "/guide/programs/p", "", 200)
	call("POST", "/users/me/favorites/"+channel.ID, "{}", 200)
	call("GET", "/users/me/favorites", "", 200)
	call("DELETE", "/users/me/favorites/"+channel.ID, "{}", 200)
	saved := csrf
	csrf = ""
	call("POST", "/playback/tune", body, 403)
	call("POST", "/users/me/favorites/"+channel.ID, "{}", 403)
	csrf = saved
	w = call("POST", "/playback/tune", body, 201)
	var tune playback.Tune
	json.Unmarshal(w.Body.Bytes(), &tune)
	call("GET", "/playback/"+tune.ID+"/stream/main", "", 200)
	call("POST", "/playback/"+tune.ID+"/heartbeat", `{"positionMs":600000,"state":"PLAYING","ended":false}`, 200)
	call("GET", "/watch/channels/missing/artwork/logo", "", 404)
	exec("UPDATE users SET role='ADMIN'")
	call("GET", "/playback/active", "", 200)
	exec("UPDATE channels SET enabled=0")
	call("POST", "/playback/tune", body, 404)
	call("GET", "/guide/programs/p", "", 404)
	call("GET", "/playback/"+tune.ID+"/stream/main", "", 404)
	exec("UPDATE channels SET enabled=1")
	call("POST", "/auth/logout", "{}", 200)
	call("GET", "/playback/"+tune.ID+"/stream/main", "", 401)
	var state string
	db.QueryRow("SELECT state FROM viewing_sessions WHERE id=?", tune.ID).Scan(&state)
	if state != "STOPPED" {
		t.Fatal("logout leaked session")
	}
}
