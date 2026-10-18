package playback

import (
	"afterglow.local/tv/internal/interstitials"
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPresentationContinuityAndFallback(t *testing.T) {
	s, a := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	tune, e := s.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.schedule.Presentation.SaveAsset(ctx, interstitials.Asset{Name: "Next", Kind: "COMING_UP_NEXT", Source: "GENERATED_METADATA", Duration: 10000, Enabled: true}, "u")
	if e != nil {
		t.Fatal(e)
	}
	end := tune.Program.End
	if _, e = s.db.Exec("UPDATE channel_programs SET start_time_utc=start_time_utc+10000,end_time_utc=end_time_utc+10000 WHERE id IN('b','c')"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec(`INSERT INTO presentation_entries VALUES('break',?,'b',?,NULL,?,?,10000,0,'test')`, r.ChannelID, b.ID, end, end+10000); e != nil {
		t.Fatal(e)
	}
	s.Now = func() time.Time { return time.UnixMilli(end + 500) }
	station, e := s.Advance(ctx, "u", "auth", tune.ID, false)
	if e != nil || station.ID != tune.ID || station.Delivery != "METADATA_PRESENTATION" {
		t.Fatal(station, e)
	}
	if !a.sources[0].closed {
		t.Fatal("old stream left open")
	}
	s.Now = func() time.Time { return time.UnixMilli(end + 10050) }
	next, e := s.Advance(ctx, "u", "auth", tune.ID, false)
	if e != nil || next.ID != tune.ID || next.Program.ID != "b" {
		t.Fatal(next, e)
	}
	active, e := s.Active(ctx)
	if e != nil || len(active) != 1 {
		t.Fatal(active, e)
	}
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM viewing_sessions").Scan(&count)
	if count != 1 {
		t.Fatal("created extra viewing rows", count)
	}
	if _, e = s.Advance(ctx, "v", "auth", tune.ID, false); e == nil {
		t.Fatal("cross-user transition")
	}
}
func TestEndPauseRaceStillTransitions(t *testing.T) {
	s, _ := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	v, e := s.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	end := v.Program.End
	s.Now = func() time.Time { return time.UnixMilli(end - 10) }
	if _, e = s.Heartbeat(ctx, "u", v.ID, Heartbeat{Position: v.Duration - 10, State: "PAUSED"}); e != nil {
		t.Fatal(e)
	}
	s.Now = func() time.Time { return time.UnixMilli(end + 50) }
	u, e := s.Heartbeat(ctx, "u", v.ID, Heartbeat{Position: v.Duration, State: "PLAYING", Ended: true})
	if e != nil || !u.Transition {
		t.Fatal(u, e)
	}
}
func TestLocalPresentationSurvivesPlexOutageAndMissingFile(t *testing.T) {
	s, a := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	s.schedule.Presentation = interstitials.New(s.db, t.TempDir())
	asset := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if e := os.MkdirAll(s.schedule.Presentation.Dir, 0700); e != nil {
		t.Fatal(e)
	}
	path, _ := s.schedule.Presentation.Path(asset)
	if e := os.WriteFile(path, []byte("local clip"), 0600); e != nil {
		t.Fatal(e)
	}
	start := s.Now().UnixMilli() - 1000
	if _, e := s.db.Exec(`INSERT INTO interstitial_assets(id,name,kind,source,duration_ms,created_at,updated_at) VALUES(?,'Ident','IDENT','UPLOAD',4000,1,1)`, asset); e != nil {
		t.Fatal(e)
	}
	if _, e := s.db.Exec(`UPDATE channel_programs SET start_time_utc=?,end_time_utc=? WHERE id='a'`, start+4000, start+4000+3600000); e != nil {
		t.Fatal(e)
	}
	if _, e := s.db.Exec(`INSERT INTO presentation_entries VALUES('local',?,'a',?,NULL,?,?,4000,0,'test')`, r.ChannelID, asset, start, start+4000); e != nil {
		t.Fatal(e)
	}
	a.err = errors.New("offline")
	v, e := s.Tune(ctx, "u", "auth", r)
	if e != nil || v.Delivery != "LOCAL_INTERSTITIAL" {
		t.Fatal(v, e)
	}
	response, e := s.Fetch(ctx, "u", v.ID, strings.TrimPrefix(v.StreamURL, "/api/v1/playback/"+v.ID+"/stream/"), "bytes=0-4")
	if e != nil {
		t.Fatal(e)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(data) != "local" {
		t.Fatal(string(data))
	}
	if e = os.Remove(path); e != nil {
		t.Fatal(e)
	}
	fallback, e := s.Advance(ctx, "u", "auth", v.ID, false)
	if e != nil || fallback.ID != v.ID || fallback.Delivery != "METADATA_PRESENTATION" {
		t.Fatal(fallback, e)
	}
	var available bool
	s.db.QueryRow("SELECT available FROM interstitial_assets WHERE id=?", asset).Scan(&available)
	if available {
		t.Fatal("missing asset not flagged")
	}
}
