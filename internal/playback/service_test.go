package playback

import (
	"afterglow.local/tv/internal/channels"
	"afterglow.local/tv/internal/database"
	"afterglow.local/tv/internal/plex"
	"afterglow.local/tv/internal/scheduler"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

type fakeSource struct{ closed bool }

func (f *fakeSource) Fetch(context.Context, string, string) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"video/mp4"}}, Body: io.NopCloser(strings.NewReader("video"))}, nil
}
func (f *fakeSource) Close(context.Context) error { f.closed = true; return nil }

type fakeAdapter struct {
	err     error
	sources []*fakeSource
}

func (f *fakeAdapter) ResolvePlayback(_ context.Context, _ string, _ plex.Capabilities, _ string) (plex.PlaybackResolution, error) {
	if f.err != nil {
		return plex.PlaybackResolution{}, f.err
	}
	src := &fakeSource{}
	f.sources = append(f.sources, src)
	return plex.PlaybackResolution{Source: src, Duration: 3600000, ServerID: "server", Mode: "DIRECT_PLAY", Mime: "video/mp4"}, nil
}
func (f *fakeAdapter) StopPlayback(context.Context, string, string, string) error { return nil }
func playbackFixture(t *testing.T) (*Service, *fakeAdapter) {
	t.Helper()
	dir := t.TempDir()
	db, e := database.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec("INSERT INTO users VALUES('u','viewer','unused','USER',1),('v','other','unused','USER',1)")
	exec("INSERT INTO sessions VALUES('auth','u','csrf',9999999999)")
	exec(`INSERT INTO plex_servers(id,name,base_url,machine_identifier,token_encrypted,token_key_version,created_at,updated_at) VALUES('server','Fixture','http://offline.invalid','server',x'00',1,1,1)`)
	exec(`INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,selected,created_at,updated_at) VALUES('l','server','1','Movies','movie',1,1,1)`)
	exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,duration_ms,created_at,updated_at) VALUES('m','server','l','1','movie','Movie',3600000,1,1)`)
	members := channels.New(db, dir)
	c, e := members.Save(context.Background(), "", channels.Definition{Number: "101", Name: "Cinema", Enabled: true, Visibility: "VISIBLE", Logo: channels.Appearance{Source: "AUTO"}, Background: channels.Appearance{Source: "AUTO"}, Source: channels.Source{Type: "STATIC", Static: &channels.StaticConfig{MediaIDs: []string{"m"}}}}, "u")
	if e != nil {
		t.Fatal(e)
	}
	exec("UPDATE channels SET id=id WHERE id=?", c.ID)
	base := int64(2000000000000)
	for i := 0; i < 3; i++ {
		exec(`INSERT INTO channel_programs VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, string(rune('a'+i)), c.ID, "m", "m", base+int64(i)*3600000, base+int64(i+1)*3600000, 3600000, i, "g", 1, "test", 1)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	adapter := &fakeAdapter{}
	s := New(db, scheduler.New(db, members, log), adapter, log)
	s.Now = func() time.Time { return time.UnixMilli(base + 660000) }
	return s, adapter
}
func tuneRequest(t *testing.T, s *Service) TuneRequest {
	v, e := s.OnNow(context.Background())
	if e != nil || len(v) != 1 {
		t.Fatal(v, e)
	}
	return TuneRequest{ChannelID: v[0].Channel.ID, Mode: "LIVE", ClientID: "client", Capabilities: plex.Capabilities{ClientType: "browser"}}
}
func TestOffsets(t *testing.T) {
	for _, c := range []struct{ now, start, duration, want int64 }{{100, 100, 1000, 0}, {1100, 100, 1000, 999}, {50, 100, 1000, 0}, {760000, 100000, 3600000, 660000}, {100, 0, 0, 0}} {
		if got := Offset(c.now, c.start, c.duration); got != c.want {
			t.Fatal(c, got)
		}
	}
}
func TestRestartAndAbandonedSessionRecovery(t *testing.T) {
	s, a := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	v, e := s.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	restarted := New(s.db, s.schedule, a, s.log)
	restarted.Now = s.Now
	if e = restarted.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	var state string
	if e = s.db.QueryRow("SELECT state FROM viewing_sessions WHERE id=?", v.ID).Scan(&state); e != nil || state != "STOPPED" {
		t.Fatal(state, e)
	}
	if _, e = restarted.Get("u", v.ID); e == nil {
		t.Fatal("stale descriptor survived restart")
	}
	s.Stop(ctx, "u", v.ID)
	fresh, e := restarted.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	if fresh.Program.ID != v.Program.ID || fresh.Position != v.Position {
		t.Fatal("restart shifted schedule")
	}
	restarted.Stop(ctx, "u", fresh.ID)
}
func TestTuneBeginningGoLiveAndBoundary(t *testing.T) {
	s, a := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	live, e := s.Tune(ctx, "u", "auth", r)
	if e != nil || live.Position != 660000 || live.Program.ID != "a" || live.Next.ID != "b" {
		t.Fatal(live, e)
	}
	r.Mode = "FROM_BEGINNING"
	begin, e := s.Tune(ctx, "u", "auth", r)
	if e != nil || begin.Position != 0 {
		t.Fatal(begin, e)
	}
	back, e := s.GoLive(ctx, "u", "auth", begin.ID)
	if e != nil || back.Position != 660000 || !a.sources[1].closed {
		t.Fatal(back, e)
	}
	u, e := s.Heartbeat(ctx, "u", live.ID, Heartbeat{Position: 660000, State: "PAUSED"})
	if e != nil || u.Mode != "BEHIND_LIVE" {
		t.Fatal(u, e)
	}
	s.Now = func() time.Time { return time.UnixMilli(live.Program.End) }
	u, e = s.Heartbeat(ctx, "u", back.ID, Heartbeat{Position: 3599000, State: "PLAYING"})
	if e != nil || !u.Transition {
		t.Fatal(u, e)
	}
	next, e := s.GoLive(ctx, "u", "auth", back.ID)
	if e != nil || next.Program.ID != "b" || next.Position != 0 {
		t.Fatal(next, e)
	}
	s.Stop(ctx, "u", next.ID)
	s.Stop(ctx, "u", live.ID)
}
func TestIsolationCleanupAndFailureSafety(t *testing.T) {
	s, a := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	one, e := s.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	two, e := s.Tune(ctx, "v", "other-auth", r)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get("v", one.ID); e == nil {
		t.Fatal("cross user read")
	}
	if e = s.Stop(ctx, "v", one.ID); e == nil {
		t.Fatal("cross user stop")
	}
	if _, e = s.Fetch(ctx, "v", one.ID, "main", ""); e == nil {
		t.Fatal("cross user stream")
	}
	if e = s.StopAuth(ctx, "auth"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get("v", two.ID); e != nil {
		t.Fatal("other session stopped")
	}
	s.Now = func() time.Time { return time.UnixMilli(two.ServerTime + 91000) }
	if e = s.Cleanup(ctx); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get("v", two.ID); e == nil {
		t.Fatal("stale retained")
	}
	for _, kind := range []string{"offline", "unauthorized", "missing"} {
		t.Run(kind, func(t *testing.T) {
			a.err = errors.New("upstream-secret-credential")
			if _, e = s.Tune(ctx, "u", "auth", r); e == nil || strings.Contains(SafeError(e), "secret") {
				t.Fatal("unsafe failure")
			}
			var n int
			s.db.QueryRow("SELECT count(*) FROM plex_media_items WHERE available=1").Scan(&n)
			if n != 1 {
				t.Fatal("inventory mutated")
			}
		})
	}
}
func TestDisabledMissingAndInvalidRequests(t *testing.T) {
	s, _ := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	s.db.Exec("UPDATE channels SET enabled=0")
	if _, e := s.Tune(ctx, "u", "auth", r); e == nil {
		t.Fatal("disabled accepted")
	}
	s.db.Exec("UPDATE channels SET enabled=1")
	s.db.Exec("UPDATE plex_media_items SET available=0")
	if _, e := s.Tune(ctx, "u", "auth", r); e == nil {
		t.Fatal("missing accepted")
	}
	s.db.Exec("UPDATE plex_media_items SET available=1")
	r.Mode = "invented"
	if _, e := s.Tune(ctx, "u", "auth", r); e == nil {
		t.Fatal("invalid mode")
	}
}

func TestCatchUpIdentityAndHistorySafety(t *testing.T) {
	s, _ := playbackFixture(t)
	ctx := context.Background()
	s.Now = func() time.Time { return time.UnixMilli(2000000000000 + 4000000) }
	r := tuneRequest(t, s)
	r.ProgramID = "a"
	r.Mode = "FROM_BEGINNING"
	v, e := s.Tune(ctx, "u", "auth", r)
	if e != nil || v.Position != 0 || v.Origin != "CATCH_UP" || v.Program.ID != "a" {
		t.Fatal(v, e)
	}
	u, e := s.Heartbeat(ctx, "u", v.ID, Heartbeat{Position: 100, State: "PLAYING"})
	if e != nil || u.Transition || u.Mode == "LIVE" {
		t.Fatal(u, e)
	}
	live, e := s.GoLive(ctx, "u", "auth", v.ID)
	if e != nil || live.Program.ID != "b" || live.Mode != "LIVE" {
		t.Fatal(live, e)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM channel_programs").Scan(&count)
	if count != 3 {
		t.Fatal(count)
	}
	var last string
	s.db.QueryRow("SELECT last_channel_id FROM viewer_preferences WHERE user_id='u'").Scan(&last)
	if last != r.ChannelID {
		t.Fatal(last)
	}
	r.ProgramID = "c"
	if _, e = s.Tune(ctx, "u", "auth", r); e == nil {
		t.Fatal("future program accepted")
	}
	r.ProgramID = "expired"
	if _, e = s.Tune(ctx, "u", "auth", r); e == nil {
		t.Fatal("expired program accepted")
	}
	r.ProgramID = "a"
	s.db.Exec("UPDATE plex_media_items SET available=0")
	if _, e = s.Tune(ctx, "u", "auth", r); e == nil {
		t.Fatal("missing catchup accepted")
	}
}
