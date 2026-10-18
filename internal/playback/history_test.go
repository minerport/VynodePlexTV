package playback

import (
	"afterglow.local/tv/internal/personalization"
	"context"
	"testing"
	"time"
)

func TestPlaybackHistoryMeaningfulWatchAndClear(t *testing.T) {
	s, _ := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	v, e := s.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	base := s.Now()
	for i := 0; i <= 10; i++ {
		at := base.Add(time.Duration(i) * 15 * time.Second)
		s.Now = func() time.Time { return at }
		if _, e = s.Heartbeat(ctx, "u", v.ID, Heartbeat{Position: v.Position + int64(i)*15000, State: "PLAYING"}); e != nil {
			t.Fatal(e)
		}
	}
	p := personalization.New(s.db)
	h, e := p.History(ctx, "u", 0)
	if e != nil || len(h) != 1 || h[0].Watched != 135000 {
		t.Fatal(h, e)
	}
	// Exactly repeated heartbeat contributes nothing.
	if _, e = s.Heartbeat(ctx, "u", v.ID, Heartbeat{Position: v.Position + 150000, State: "PLAYING"}); e != nil {
		t.Fatal(e)
	}
	h, e = p.History(ctx, "u", 0)
	if e != nil || h[0].Watched != 135000 {
		t.Fatal(h, e)
	}
	if e = p.ClearHistory(ctx, "u"); e != nil {
		t.Fatal(e)
	}
	s.Now = func() time.Time { return base.Add(165 * time.Second) }
	if _, e = s.Heartbeat(ctx, "u", v.ID, Heartbeat{Position: v.Position + 165000, State: "PLAYING"}); e != nil {
		t.Fatal(e)
	}
	if e = s.Stop(ctx, "u", v.ID); e != nil {
		t.Fatal(e)
	}
	h, e = p.History(ctx, "u", 0)
	if e != nil || len(h) != 0 {
		t.Fatal(h, e)
	}
}
