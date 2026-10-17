package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"afterglow.local/tv/internal/database"
)

type fixture struct {
	mu                                     sync.Mutex
	token                                  string
	offline, unauthorized, partial, repeat bool
	movies                                 []string
	updated                                int
	title                                  string
	metadataCalls, artCalls                int
	hook                                   func()
	server                                 *httptest.Server
}

func newFixture(t *testing.T) *fixture {
	f := &fixture{token: newID(), movies: []string{"10", "11"}, updated: 100, title: "Night Movie"}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	return f
}
func (f *fixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Header.Get("X-Plex-Token") != f.token || f.unauthorized {
		w.WriteHeader(401)
		fmt.Fprint(w, f.token)
		return
	}
	if r.URL.Query().Get("X-Plex-Token") != "" || r.Header.Get("X-Plex-Client-Identifier") == "" || r.Header.Get("X-Plex-Product") == "" {
		w.WriteHeader(400)
		return
	}
	if f.offline {
		w.WriteHeader(503)
		fmt.Fprint(w, f.token)
		return
	}
	w.Header().Set("Content-Type", "application/xml")
	switch r.URL.Path {
	case "/":
		fmt.Fprint(w, `<MediaContainer machineIdentifier="fixture-server" friendlyName="Cinema" product="Plex Media Server" version="1.2" platform="Linux" platformVersion="6" owned="1" secure="0"/>`)
	case "/library/sections":
		fmt.Fprint(w, `<MediaContainer size="3"><Directory key="1" title="Movies" type="movie" uuid="movies" updatedAt="1" scanner="Plex Movie"/><Directory key="2" title="TV" type="show" uuid="tv" updatedAt="1"/><Directory key="3" title="Music" type="artist" uuid="music"/></MediaContainer>`)
	case "/library/sections/1/all", "/library/sections/2/all":
		if r.URL.Query().Get("sort") != "titleSort:asc" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		offset, _ := strconv.Atoi(r.URL.Query().Get("X-Plex-Container-Start"))
		kind := r.URL.Query().Get("type")
		keys := f.movies
		typ := "movie"
		if kind == "2" {
			keys = []string{"20"}
			typ = "show"
		}
		if kind == "3" {
			keys = []string{"21"}
			typ = "season"
		}
		if kind == "4" {
			keys = []string{"22"}
			typ = "episode"
		}
		if f.partial && offset > 0 {
			w.WriteHeader(503)
			return
		}
		count := 0
		if offset < len(keys) {
			count = 1
		}
		fmt.Fprintf(w, `<MediaContainer size="%d" totalSize="%d" offset="%d">`, count, len(keys), offset)
		if count > 0 {
			idx := offset
			if f.repeat {
				idx = 0
			}
			node := "Video"
			if typ == "show" || typ == "season" {
				node = "Directory"
			}
			fmt.Fprintf(w, `<%s ratingKey="%s" type="%s" title="Listing" updatedAt="%d"/>`, node, keys[idx], typ, f.updated)
		}
		fmt.Fprint(w, `</MediaContainer>`)
	default:
		if strings.Contains(r.URL.Path, "/thumb/") || strings.Contains(r.URL.Path, "/art/") {
			f.artCalls++
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("ETag", `"fixture-art"`)
			w.Write([]byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 100)))
			return
		}
		key := strings.TrimPrefix(r.URL.Path, "/library/metadata/")
		if key == r.URL.Path {
			w.WriteHeader(404)
			return
		}
		f.metadataCalls++
		if f.hook != nil {
			f.hook()
		}
		typ := "movie"
		attrs := ""
		node := "Video"
		switch key {
		case "20":
			typ = "show"
			node = "Directory"
		case "21":
			typ = "season"
			node = "Directory"
			attrs = ` parentRatingKey="20" index="1"`
		case "22":
			typ = "episode"
			attrs = ` parentRatingKey="21" grandparentRatingKey="20" index="2" parentIndex="1"`
		}
		fmt.Fprintf(w, `<MediaContainer size="1"><%s ratingKey="%s" type="%s" title="%s" titleSort="Night" summary="A fixture summary" year="2024" rating="8.2" audienceRating="8.5" contentRating="PG" duration="3600000" addedAt="50" updatedAt="%d" originallyAvailableAt="2024-01-01" studio="Fixture studio" tagline="Fixture tagline" guid="plex://movie/fixture" thumb="/library/metadata/%s/thumb/1" art="/library/metadata/%s/art/1"%s><Genre id="1" tag="Drama"/><Role id="2" tag="An Actor" role="Detective"/><Director id="3" tag="A Director"/><Writer id="4" tag="A Writer"/><Collection id="5" tag="A Collection"/><Label id="6" tag="Family"/><Guid id="imdb://tt0000001"/><Guid id="tmdb://1"/><Media id="100" container="mkv" videoCodec="h264" audioCodec="aac" videoResolution="1080" width="1920" height="1080" bitrate="4000" duration="3600000"><Part id="101" file="/private/fixture.mkv" size="10000"><Stream id="1" streamType="1" codec="h264"/><Stream id="2" streamType="2" codec="aac" language="English" languageCode="eng" default="1" channels="2"/><Stream id="3" streamType="3" codec="srt" forced="0" hearingImpaired="1"/></Part></Media></%s></MediaContainer>`, node, key, typ, f.title, f.updated, key, key, attrs, node)
	}
}
func newTestService(t *testing.T) (*Service, *fixture) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	s, err := NewService(db, dir, "test", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	return s, newFixture(t)
}
func connected(t *testing.T, s *Service, f *fixture) (Server, []Library) {
	t.Helper()
	server, err := s.SaveServer(context.Background(), "", Connection{BaseURL: f.server.URL, Token: f.token}, "")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.discover(context.Background(), server.ID); err != nil {
		t.Fatal(err)
	}
	libs, err := s.Libraries(context.Background(), server.ID)
	if err != nil {
		t.Fatal(err)
	}
	return server, libs
}
func libraryOf(t *testing.T, libs []Library, kind string) Library {
	t.Helper()
	for _, l := range libs {
		if l.Type == kind {
			return l
		}
	}
	t.Fatal("missing library")
	return Library{}
}
func syncTest(t *testing.T, s *Service, l Library, mode string) Job {
	t.Helper()
	ctx := context.Background()
	if err := s.SelectLibrary(ctx, l.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	j, err := s.Enqueue(ctx, "PLEX_LIBRARY_SYNC", l.ServerID, l.ID, mode, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.runNext(ctx); err != nil {
		t.Fatal(err)
	}
	j, err = s.Job(ctx, j.ID)
	if err != nil {
		t.Fatal(err)
	}
	return j
}
func inventory(t *testing.T, s *Service) MediaResult {
	t.Helper()
	v, err := s.Media(context.Background(), MediaQuery{})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestVaultPersistenceTamperAndMissingKey(t *testing.T) {
	dir := t.TempDir()
	v, err := OpenVault(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	secret := newID()
	enc, err := v.Encrypt("server", secret)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(enc), secret) {
		t.Fatal("plaintext ciphertext")
	}
	reopened, err := OpenVault(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.ClientID != v.ClientID {
		t.Fatal("client id changed")
	}
	got, err := reopened.Decrypt("server", 1, enc)
	if err != nil || got != secret {
		t.Fatal("credential roundtrip failed")
	}
	if _, err = v.Decrypt("other", 1, enc); err == nil {
		t.Fatal("AAD ignored")
	}
	enc[len(enc)-1] ^= 1
	if _, err = v.Decrypt("server", 1, enc); err == nil {
		t.Fatal("tamper accepted")
	}
	if _, err = v.Decrypt("server", 2, enc); err == nil {
		t.Fatal("unknown key accepted")
	}
	if err = os.Remove(filepath.Join(dir, "secrets", "plex-key-v1")); err != nil {
		t.Fatal(err)
	}
	if _, err = OpenVault(dir, true); err == nil {
		t.Fatal("lost key silently replaced")
	}
}
func TestClientIdentityUnauthorizedTimeoutTLSAndRedaction(t *testing.T) {
	f := newFixture(t)
	c := NewClient(f.server.URL, f.token, "stable-client", "test")
	identity, err := c.Identity(context.Background())
	if err != nil || identity.MachineIdentifier != "fixture-server" || identity.FriendlyName != "Cinema" {
		t.Fatalf("identity: %v", err)
	}
	f.mu.Lock()
	f.unauthorized = true
	f.mu.Unlock()
	_, err = c.Identity(context.Background())
	if ErrorCode(err) != "UNAUTHORIZED" || strings.Contains(SafeMessage(err), f.token) {
		t.Fatal("authorization/redaction failed")
	}
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	c = NewClient(slow.URL, newID(), "client", "test")
	c.http.Timeout = 10 * time.Millisecond
	_, err = c.Identity(context.Background())
	if ErrorCode(err) != "TIMEOUT" {
		t.Fatalf("timeout: %v", err)
	}
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer tlsServer.Close()
	c = NewClient(tlsServer.URL, newID(), "client", "test")
	_, err = c.Identity(context.Background())
	if ErrorCode(err) != "TLS" {
		t.Fatalf("TLS: %v", err)
	}
	if strings.Contains(SafeMessage(fmt.Errorf("private %s", f.token)), f.token) {
		t.Fatal("raw error leaked")
	}
}
func TestURLAndArtworkBoundary(t *testing.T) {
	for _, bad := range []string{"file:///tmp/a", "http://user:password@host", "http://host?X-Plex-Token=value", "http://host/../secret"} {
		if _, err := NormalizeURL(bad); err == nil {
			t.Fatal("invalid URL accepted")
		}
	}
	for _, good := range []string{"http://192.168.1.10:32400", "https://plex.example.com"} {
		v, err := NormalizeURL(good + "/")
		if err != nil || v != good {
			t.Fatal("valid URL normalization failed")
		}
	}
	for _, bad := range []string{"https://other.test/a", "//other.test/a", "/library/../secret", "/library/thumb?X-Plex-Token=value"} {
		if safeArtworkPath(bad) {
			t.Fatal("unsafe artwork URL accepted")
		}
	}
}
func TestConnectionsDiscoverySelectionAndEncryptedResponse(t *testing.T) {
	s, f := newTestService(t)
	server, libs := connected(t, s, f)
	if len(libs) != 3 {
		t.Fatal("library discovery failed")
	}
	raw, _ := json.Marshal(server)
	if strings.Contains(string(raw), f.token) || strings.Contains(string(raw), "token") {
		t.Fatal("secret exposed")
	}
	var ciphertext []byte
	if err := s.db.QueryRow("SELECT token_encrypted FROM plex_servers WHERE id=?", server.ID).Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ciphertext), f.token) {
		t.Fatal("token stored in plaintext")
	}
	if _, err := s.SaveServer(context.Background(), "", Connection{BaseURL: f.server.URL, Token: f.token}, ""); ErrorCode(err) != "CONFLICT" {
		t.Fatal("duplicate accepted")
	}
	music := libraryOf(t, libs, "artist")
	if err := s.SelectLibrary(context.Background(), music.ID, true, ""); ErrorCode(err) != "INVALID" {
		t.Fatal("unsupported library selected")
	}
	movie := libraryOf(t, libs, "movie")
	if err := s.SelectLibrary(context.Background(), movie.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if err := s.SelectLibrary(context.Background(), movie.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveServer(context.Background(), server.ID, Connection{BaseURL: f.server.URL, Name: "Renamed"}, ""); err != nil {
		t.Fatal(err)
	}
}
func TestMovieTVMetadataAndArtwork(t *testing.T) {
	s, f := newTestService(t)
	_, libs := connected(t, s, f)
	for _, kind := range []string{"movie", "show"} {
		j := syncTest(t, s, libraryOf(t, libs, kind), "full")
		if j.Status != "SUCCEEDED" {
			t.Fatalf("sync %s: %s", kind, j.Error)
		}
	}
	all := inventory(t, s)
	if all.Total != 5 {
		t.Fatalf("media count %d", all.Total)
	}
	types := map[string]bool{}
	for _, entry := range all.Items {
		m, err := s.Item(context.Background(), entry.ID)
		if err != nil {
			t.Fatal(err)
		}
		types[m.Type] = true
		if len(m.Tags) != 6 || len(m.ExternalIDs) != 3 || len(m.Versions) != 1 || len(m.Versions[0].Streams) != 3 {
			t.Fatalf("metadata relationships missing for %s", m.Type)
		}
		if m.Type == "episode" && (m.ParentKey != "21" || m.GrandparentKey != "20" || *m.Index != 2) {
			t.Fatal("episode hierarchy lost")
		}
		b, _ := json.Marshal(m)
		if strings.Contains(string(b), "/private/") || strings.Contains(string(b), f.token) {
			t.Fatal("private data leaked")
		}
	}
	if len(types) != 4 {
		t.Fatal("media types missing")
	}
	m := all.Items[0]
	a, err := s.Artwork(context.Background(), m.ID, "poster")
	if err != nil || a.ContentType != "image/png" {
		t.Fatalf("artwork: %v", err)
	}
	if _, err = s.Artwork(context.Background(), m.ID, "poster"); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	calls := f.artCalls
	f.mu.Unlock()
	if calls != 1 {
		t.Fatal("artwork cache miss")
	}
	if _, err = s.Artwork(context.Background(), m.ID, "background"); err != nil {
		t.Fatal(err)
	}
}
func TestSyncRerunChangeRemoveReappearAndIncremental(t *testing.T) {
	s, f := newTestService(t)
	_, libs := connected(t, s, f)
	l := libraryOf(t, libs, "movie")
	if j := syncTest(t, s, l, "full"); j.Status != "SUCCEEDED" {
		t.Fatal(j.Error)
	}
	initial := inventory(t, s)
	ids := map[string]string{}
	for _, m := range initial.Items {
		ids[m.RatingKey] = m.ID
	}
	f.mu.Lock()
	calls := f.metadataCalls
	f.mu.Unlock()
	if j := syncTest(t, s, l, "incremental"); j.Status != "SUCCEEDED" {
		t.Fatal(j.Error)
	}
	f.mu.Lock()
	afterCalls := f.metadataCalls
	f.mu.Unlock()
	if calls != afterCalls {
		t.Fatal("unchanged metadata refetched")
	}
	f.mu.Lock()
	f.updated = 200
	f.title = "Changed title"
	f.movies = []string{"10"}
	f.mu.Unlock()
	if j := syncTest(t, s, l, "incremental"); j.Status != "SUCCEEDED" {
		t.Fatal(j.Error)
	}
	all := inventory(t, s)
	if all.Total != 2 {
		t.Fatal("deleted row lost")
	}
	for _, m := range all.Items {
		if ids[m.RatingKey] != m.ID {
			t.Fatal("ID changed")
		}
		if m.RatingKey == "11" && m.Available {
			t.Fatal("removed media remained available")
		}
		if m.RatingKey == "10" && m.Title != "Changed title" {
			t.Fatal("metadata not updated")
		}
	}
	f.mu.Lock()
	f.movies = []string{"10", "11"}
	f.mu.Unlock()
	if j := syncTest(t, s, l, "full"); j.Status != "SUCCEEDED" {
		t.Fatal(j.Error)
	}
	for _, m := range inventory(t, s).Items {
		if !m.Available || m.ID != ids[m.RatingKey] {
			t.Fatal("reappearing item identity lost")
		}
	}
}
func TestOutagePartialFailureAndRepeatedPaginationPreserveInventory(t *testing.T) {
	for _, mode := range []string{"offline", "partial", "repeat"} {
		t.Run(mode, func(t *testing.T) {
			s, f := newTestService(t)
			server, libs := connected(t, s, f)
			l := libraryOf(t, libs, "movie")
			if j := syncTest(t, s, l, "full"); j.Status != "SUCCEEDED" {
				t.Fatal(j.Error)
			}
			f.mu.Lock()
			f.offline = mode == "offline"
			f.partial = mode == "partial"
			f.repeat = mode == "repeat"
			f.mu.Unlock()
			j := syncTest(t, s, l, "full")
			if j.Status != "FAILED" {
				t.Fatalf("expected failure: %s", j.Status)
			}
			for _, m := range inventory(t, s).Items {
				if !m.Available {
					t.Fatal("failure marked inventory missing")
				}
			}
			f.mu.Lock()
			f.offline = false
			f.partial = false
			f.repeat = false
			f.mu.Unlock()
			if err := s.health(context.Background(), server.ID); err != nil {
				t.Fatal(err)
			}
			if j = syncTest(t, s, l, "full"); j.Status != "SUCCEEDED" {
				t.Fatal(j.Error)
			}
		})
	}
}
func TestJobsCancellationAndLifecycle(t *testing.T) {
	s, f := newTestService(t)
	_, libs := connected(t, s, f)
	l := libraryOf(t, libs, "movie")
	ctx := context.Background()
	if j := syncTest(t, s, l, "full"); j.Status != "SUCCEEDED" {
		t.Fatal(j.Error)
	}
	j, err := s.Enqueue(ctx, "PLEX_LIBRARY_SYNC", l.ServerID, l.ID, "full", "")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.Enqueue(ctx, "PLEX_LIBRARY_SYNC", l.ServerID, l.ID, "full", "")
	if err != nil || again.ID != j.ID {
		t.Fatal("duplicate job queued")
	}
	if err = s.Cancel(ctx, j.ID, ""); err != nil {
		t.Fatal(err)
	}
	done, _ := s.Job(ctx, j.ID)
	if done.Status != "CANCELLED" {
		t.Fatal("queued cancellation failed")
	}
	j, err = s.Enqueue(ctx, "PLEX_LIBRARY_SYNC", l.ServerID, l.ID, "full", "")
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.hook = func() {
		if e := s.Cancel(context.Background(), j.ID, ""); e != nil {
			t.Error(e)
		}
	}
	f.mu.Unlock()
	if _, err = s.runNext(ctx); err != nil {
		t.Fatal(err)
	}
	done, _ = s.Job(ctx, j.ID)
	if done.Status != "CANCELLED" {
		t.Fatalf("running cancellation status %s", done.Status)
	}
	for _, m := range inventory(t, s).Items {
		if !m.Available {
			t.Fatal("cancel marked media missing")
		}
	}
}
func TestSearchHierarchyAndSelectionRetention(t *testing.T) {
	s, f := newTestService(t)
	_, libs := connected(t, s, f)
	l := libraryOf(t, libs, "show")
	if j := syncTest(t, s, l, "full"); j.Status != "SUCCEEDED" {
		t.Fatal(j.Error)
	}
	result, err := s.Media(context.Background(), MediaQuery{ParentKey: "21", Title: "Night"})
	if err != nil || result.Total != 1 || result.Items[0].Type != "episode" {
		t.Fatalf("hierarchy search: %v", err)
	}
	if err = s.SelectLibrary(context.Background(), l.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	if inventory(t, s).Total != 3 {
		t.Fatal("disabled library inventory removed")
	}
	root, err := s.Media(context.Background(), MediaQuery{TopLevel: true})
	if err != nil || root.Total != 1 || root.Items[0].Type != "show" {
		t.Fatal("top-level browser must exclude seasons and episodes")
	}
	for _, number := range []int{10, 2} {
		_, err = s.db.Exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,parent_rating_key,media_type,title,index_number,created_at,updated_at) VALUES(?,?,?,?,?,'season',?,?,1,1)`, fmt.Sprint(number), l.ServerID, l.ID, fmt.Sprint(number), "20", fmt.Sprintf("Season %d", number), number)
		if err != nil {
			t.Fatal(err)
		}
	}
	seasons, err := s.Media(context.Background(), MediaQuery{ServerID: l.ServerID, ParentKey: "20"})
	if err != nil || seasons.Total != 3 || seasons.Items[1].Title != "Season 2" || seasons.Items[2].Title != "Season 10" {
		t.Fatal("seasons must use numeric order")
	}
}
