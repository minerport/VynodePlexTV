package scheduler

import (
	"afterglow.local/tv/internal/interstitials"
	"context"
	"reflect"
	"testing"
	"time"
)

func TestPersistedPresentationRepair(t *testing.T) {
	s, _, id := serviceFixture(t)
	ctx := context.Background()
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	current, _ := s.GetProgramAt(ctx, id, s.Now())
	a, e := s.Presentation.SaveAsset(ctx, interstitials.Asset{Name: "Next", Kind: "COMING_UP_NEXT", Source: "GENERATED_METADATA", Duration: 10000, Enabled: true}, "owner")
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Presentation.SaveRule(ctx, interstitials.Rule{Name: "Between", ChannelID: id, Enabled: true, Kind: a.Kind, Placement: "BEFORE", Every: 1, MaxDuration: 20000})
	if e != nil {
		t.Fatal(e)
	}
	b := interstitials.DefaultBranding(id)
	b.Enabled = true
	if e = s.Presentation.SaveBranding(ctx, b); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	after, _ := s.GetProgramAt(ctx, id, s.Now())
	if current.ID != after.ID {
		t.Fatal("current rewritten")
	}
	checkIntegrity(t, s, id)
	entries := all(t, s, id)
	var clip Entry
	for _, p := range entries {
		if p.Kind == "INTERSTITIAL" {
			clip = p
			break
		}
	}
	if clip.ID == "" {
		t.Fatal("no inserted clips")
	}
	if e = s.Presentation.DeleteAsset(ctx, a.ID); e == nil {
		t.Fatal("referenced asset deleted")
	}
	if _, e = s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(entries, all(t, s, id)) {
		t.Fatal("rerun changed timeline")
	}
	s.Now = func() time.Time { return time.UnixMilli(clip.Start + 1000) }
	a.Enabled = false
	if _, e = s.Presentation.SaveAsset(ctx, a, "owner"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	still, e := s.GetProgramAt(ctx, id, s.Now())
	if e != nil || still.ID != clip.ID {
		t.Fatal("current station break was removed", e)
	}
	checkIntegrity(t, s, id)
}
