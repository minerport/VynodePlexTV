package playback

import (
	"context"
	"testing"
)

func TestNativeReconfigurePreservesViewerAndPosition(t *testing.T) {
	s, a := playbackFixture(t)
	ctx := context.Background()
	r := tuneRequest(t, s)
	r.Capabilities.ClientType = "android-tv"
	first, e := s.Tune(ctx, "u", "auth", r)
	if e != nil {
		t.Fatal(e)
	}
	caps := r.Capabilities
	caps.Quality = "720p"
	changed, e := s.Reconfigure(ctx, "u", "auth", first.ID, caps, 123456)
	if e != nil {
		t.Fatal(e)
	}
	if changed.ID != first.ID || changed.Program.ID != first.Program.ID || changed.Position != 123456 {
		t.Fatal("options changed viewer/program/position")
	}
	if !a.sources[0].closed {
		t.Fatal("previous source not closed")
	}
	if _, e = s.Reconfigure(ctx, "v", "other", first.ID, caps, 0); e == nil {
		t.Fatal("other user changed viewer")
	}
	s.Stop(ctx, "u", first.ID)
}
