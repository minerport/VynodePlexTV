package plex

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaybackCapabilities(t *testing.T) {
	v := MediaVersion{Container: "mp4", VideoCodec: "h264", AudioCodec: "aac", PartCount: 1}
	c := Capabilities{Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}}
	if !DirectCompatible(v, c) {
		t.Fatal("supported mp4 rejected")
	}
	for _, kind := range []string{"mkv", "hevc", "dts", "multipart", "force"} {
		vv, cc := v, c
		switch kind {
		case "mkv":
			vv.Container = "mkv"
		case "hevc":
			vv.VideoCodec = "hevc"
		case "dts":
			vv.AudioCodec = "dts"
		case "multipart":
			vv.PartCount = 2
		case "force":
			cc.ForceTranscode = true
		}
		if DirectCompatible(vv, cc) {
			t.Fatal(kind)
		}
	}
}
func TestPlaybackRequiresSessionDecision(t *testing.T) {
	s, f := newTestService(t)
	decisions := map[string]bool{}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session := r.URL.Query().Get("session")
		if strings.Contains(r.URL.Path, "/transcode/universal/") {
			if r.Header.Get("X-Plex-Token") != f.token {
				t.Error("missing authentication")
			}
			switch {
			case strings.HasSuffix(r.URL.Path, "/decision"):
				decisions[session] = true
				fmt.Fprint(w, `<MediaContainer generalDecisionCode="1001" transcodeDecisionCode="1001"><Video><Media videoDecision="copy" audioDecision="transcode"/></Video></MediaContainer>`)
			case strings.HasSuffix(r.URL.Path, "/start.m3u8"):
				if !decisions[session] {
					w.WriteHeader(400)
					return
				}
				fmt.Fprintf(w, "#EXTM3U\nsession/%s/base/index.m3u8\n", session)
			default:
				w.WriteHeader(200)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/library/metadata/") {
			// A hosted extra advertises browser-compatible codecs, but its Part
			// location must never be fetched directly by this application.
			rec := httptest.NewRecorder()
			f.serve(rec, r)
			body := strings.ReplaceAll(rec.Body.String(), `container="mkv"`, `container="mp4"`)
			body = strings.ReplaceAll(body, `<Part id="101"`, `<Part key="https://external.invalid/trailer.mp4" id="101"`)
			w.WriteHeader(rec.Code)
			fmt.Fprint(w, body)
			return
		}
		f.serve(w, r)
	}))
	defer up.Close()
	server, e := s.SaveServer(context.Background(), "", Connection{BaseURL: up.URL, Token: f.token}, "")
	if e != nil {
		t.Fatal(e)
	}
	if e = s.discover(context.Background(), server.ID); e != nil {
		t.Fatal(e)
	}
	libs, _ := s.Libraries(context.Background(), server.ID)
	syncTest(t, s, libraryOf(t, libs, "movie"), "full")
	var id string
	if e = s.db.QueryRow("SELECT id FROM plex_media_items WHERE media_type='movie' LIMIT 1").Scan(&id); e != nil {
		t.Fatal(e)
	}
	for _, session := range []string{"first", "second"} {
		r, e := s.ResolvePlayback(context.Background(), id, Capabilities{ClientType: "browser", HLS: true, Containers: []string{"mp4"}, VideoCodecs: []string{"h264"}, AudioCodecs: []string{"aac"}}, session)
		if e != nil {
			t.Fatal(e)
		}
		if r.Mode != "TRANSCODE" {
			t.Fatal("decision mode", r.Mode)
		}
		response, e := r.Source.Fetch(context.Background(), "main", "")
		if e != nil {
			t.Fatal(e)
		}
		response.Body.Close()
		r.Source.Close(context.Background())
	}
}
func TestPlaylistProxyTokenAndPathIsolation(t *testing.T) {
	token := newID()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != token || r.URL.Query().Get("X-Plex-Token") != "" {
			t.Error("token header isolation")
		}
		if strings.HasSuffix(r.URL.Path, "start.m3u8") {
			fmt.Fprintf(w, "#EXTM3U\n/video/:/transcode/universal/session/session/base/index.m3u8?X-Plex-Token=%s\n", token)
		} else {
			fmt.Fprint(w, "#EXTM3U\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:10,\n1.ts\n")
		}
	}))
	defer up.Close()
	src := &streamSource{client: NewClient(up.URL, token, "client", "test"), session: "session", prefix: "/safe/", resources: map[string]string{"main": "/video/:/transcode/universal/start.m3u8"}}
	r, e := src.Fetch(context.Background(), "main", "")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if strings.Contains(string(b), token) || strings.Contains(string(b), "plex") || !strings.Contains(string(b), "/safe/") {
		t.Fatal("unsafe playlist")
	}
	child := strings.TrimPrefix(strings.Split(string(b), "\n")[1], "/safe/")
	r, e = src.Fetch(context.Background(), child, "")
	if e != nil {
		t.Fatal(e)
	}
	b, _ = io.ReadAll(r.Body)
	r.Body.Close()
	if !strings.Contains(string(b), `URI="/safe/`) {
		t.Fatal("map not rewritten")
	}
	for _, bad := range []string{"https://evil.test/a", "//evil.test/a", "/library/parts/other", "../../../../secret", "/video/:/transcode/universal/session/other/a"} {
		if _, e = src.resource(src.root, bad); e == nil {
			t.Fatal("unsafe path", bad)
		}
	}
	if _, e = src.Fetch(context.Background(), "unregistered", ""); e == nil {
		t.Fatal("unregistered path")
	}
}
func TestPlaybackProxyErrorsAndRanges(t *testing.T) {
	for _, code := range []int{401, 403, 404, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code); fmt.Fprint(w, "secret-token") }))
			defer up.Close()
			s := &streamSource{client: NewClient(up.URL, "secret-token", "c", "v"), resources: map[string]string{"main": "/library/parts/1/file.mp4"}}
			if _, e := s.Fetch(context.Background(), "main", ""); e == nil || strings.Contains(SafeMessage(e), "secret-token") {
				t.Fatal("unsafe response")
			}
		})
	}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=10-20" {
			t.Error("range missing")
		}
		w.Header().Set("Content-Range", "bytes 10-20/100")
		w.WriteHeader(206)
		fmt.Fprint(w, "media bytes")
	}))
	defer up.Close()
	s := &streamSource{client: NewClient(up.URL, "token", "c", "v"), resources: map[string]string{"main": "/library/parts/1/file.mp4"}}
	r, e := s.Fetch(context.Background(), "main", "bytes=10-20")
	if e != nil || r.StatusCode != 206 {
		t.Fatal(e)
	}
	r.Body.Close()
	if _, e = s.Fetch(context.Background(), "main", "bytes=0-1,2-3"); e == nil {
		t.Fatal("multiple ranges accepted")
	}
}
