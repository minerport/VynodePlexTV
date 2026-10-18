package interstitials

import (
	"afterglow.local/tv/internal/database"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSelectionPolicies(t *testing.T) {
	at := time.Date(2026, 10, 17, 20, 0, 0, 0, time.UTC).UnixMilli()
	base := Snapshot{Branding: Branding{Enabled: true}, Rules: []Rule{{ID: "r", Enabled: true, Kind: "BUMPER", Placement: "BEFORE", Every: 1, RepeatMinutes: 120, MaxDuration: 30000}}, Assets: []Asset{{ID: "a", Kind: "BUMPER", Enabled: true, Available: true, Duration: 10000, Tags: []string{"action"}}, {ID: "b", Kind: "BUMPER", Enabled: true, Available: true, Duration: 10000}}}
	for _, tc := range []struct {
		name   string
		change func(*Snapshot)
		seq    int64
		want   int
	}{{"before movie", func(s *Snapshot) { s.Rules[0].MediaType = "movie" }, 0, 1}, {"episode filter", func(s *Snapshot) { s.Rules[0].MediaType = "episode" }, 0, 0}, {"between first", func(s *Snapshot) { s.Rules[0].Placement = "BETWEEN" }, 0, 0}, {"between episodes", func(s *Snapshot) { s.Rules[0].Placement = "BETWEEN" }, 1, 1}, {"every n", func(s *Snapshot) { s.Rules[0].Every = 3 }, 2, 0}, {"tag", func(s *Snapshot) { s.Rules[0].Tag = "kids" }, 0, 0}, {"fit", func(s *Snapshot) { s.Rules[0].MaxDuration = 9000 }, 0, 0}, {"season", func(s *Snapshot) { s.Assets[0].ActiveFrom = "2026-12-01"; s.Assets[1].Enabled = false }, 0, 0}, {"disabled", func(s *Snapshot) { s.Branding.Enabled = false }, 0, 0}, {"hour crossing", func(s *Snapshot) { s.Rules[0].Placement = "HOURLY" }, 1, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			s := base
			s.Rules = append([]Rule{}, base.Rules...)
			s.Assets = append([]Asset{}, base.Assets...)
			tc.change(&s)
			out := s.Select("seed", "movie", "m", tc.seq, at, at-3600000, "America/New_York", map[string]int64{})
			if len(out) != tc.want {
				t.Fatal(out)
			}
		})
	}
	x := base.Select("seed", "movie", "m", 1, at, 0, "UTC", map[string]int64{})
	y := base.Select("seed", "movie", "m", 1, at, 0, "UTC", map[string]int64{})
	if !reflect.DeepEqual(x, y) {
		t.Fatal("unstable")
	}
	h := map[string]int64{x[0].AssetID: at - 1000}
	z := base.Select("seed", "movie", "m", 2, at, 0, "UTC", h)
	if z[0].AssetID == x[0].AssetID {
		t.Fatal("repeat")
	}
	base.Assets = base.Assets[:1]
	z = base.Select("seed", "movie", "m", 2, at, 0, "UTC", map[string]int64{"a": at - 1000})
	if len(z) != 1 || z[0].Reason != "repeat window relaxed; oldest asset" {
		t.Fatal(z)
	}
}
func TestLocalRangesAndPathSafety(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "clip.mp4")
	if e := os.WriteFile(p, []byte("0123456789"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		r, w string
		code int
	}{{"", "0123456789", 200}, {"bytes=2-4", "234", 206}, {"bytes=-3", "789", 206}, {"bytes=99-100", "", 416}, {"bytes=0-1,4-5", "", 416}} {
		r, e := (LocalSource{p}).Fetch(context.Background(), "main", tc.r)
		if e != nil {
			t.Fatal(e)
		}
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if string(b) != tc.w || r.StatusCode != tc.code {
			t.Fatal(tc, string(b), r.StatusCode)
		}
	}
	s := New(nil, dir)
	if _, e := s.Path("../secret"); e == nil {
		t.Fatal("traversal")
	}
	if _, e := s.Path("a"); e == nil {
		t.Fatal("invalid id")
	}
}
func TestInheritanceAndUploadRejection(t *testing.T) {
	db, e := database.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	s := New(db, t.TempDir())
	if _, e = db.Exec(`INSERT INTO users VALUES('u','u','unused','OWNER',1);INSERT INTO channels(id,channel_number,name,channel_type,created_at,updated_at) VALUES('c','1','C','STATIC',1,1)`); e != nil {
		t.Fatal(e)
	}
	b := DefaultBranding("c")
	b.Enabled = true
	if e = s.SaveBranding(ctx, b); e != nil {
		t.Fatal(e)
	}
	a, e := s.SaveAsset(ctx, Asset{Name: "Next", Kind: "COMING_UP_NEXT", Source: "GENERATED_METADATA", Duration: 10000, Enabled: true}, "u")
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.SaveRule(ctx, Rule{Name: "Global", Enabled: true, Kind: a.Kind, Placement: "BEFORE", Every: 1, MaxDuration: 30000})
	if e != nil {
		t.Fatal(e)
	}
	snap, e := s.Snapshot(ctx, "c")
	if e != nil || len(snap.Rules) != 1 {
		t.Fatal(snap, e)
	}
	if _, e = db.Exec("INSERT INTO channel_interstitial_rules VALUES('c',?,0)", r.ID); e != nil {
		t.Fatal(e)
	}
	snap, e = s.Snapshot(ctx, "c")
	if e != nil || len(snap.Rules) != 0 {
		t.Fatal(snap, e)
	}
	if _, e = s.Upload(ctx, "bad", "BUMPER", "evil.html", "u", nil, 1024); e == nil {
		t.Fatal("unsafe extension")
	}
	if e = s.DeleteAsset(ctx, a.ID); e != nil {
		t.Fatal(e)
	}
}
func TestPresentationSeedAndHourlyStability(t *testing.T) {
	s := Snapshot{Branding: Branding{Enabled: true}, Rules: []Rule{{ID: "r", Kind: "IDENT", Enabled: true, Placement: "HOURLY", Every: 1, MaxDuration: 20000}}, Assets: []Asset{}}
	for _, id := range []string{"a", "b", "c", "d", "e"} {
		s.Assets = append(s.Assets, Asset{ID: id, Kind: "IDENT", Enabled: true, Available: true, Duration: 1000})
	}
	at := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC).UnixMilli()
	if len(s.Select("a", "movie", "m", 1, at, at-60000, "America/New_York", map[string]int64{})) != 1 {
		t.Fatal("DST hour crossing ignored")
	}
	if len(s.Select("a", "movie", "m", 1, at+30000, at, "America/New_York", map[string]int64{})) != 0 {
		t.Fatal("same hour repeated")
	}
	different := false
	for i := int64(0); i < 20; i++ {
		a := s.Select("a", "movie", "m", i, at, 0, "UTC", map[string]int64{})
		b := s.Select("b", "movie", "m", i, at, 0, "UTC", map[string]int64{})
		different = different || a[0].AssetID != b[0].AssetID
	}
	if !different {
		t.Fatal("seed does not affect ties")
	}
}
