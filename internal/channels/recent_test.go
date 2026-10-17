package channels

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRecentWindowsAndIsolation(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for _, days := range []int{7, 14, 30, 60, 90, 43} {
		for _, kind := range []string{"MOVIES", "TV"} {
			t.Run(fmt.Sprintf("%s/%d", kind, days), func(t *testing.T) {
				boundary := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()
				if _, e := s.db.Exec("UPDATE plex_media_items SET added_at=?", now.Unix()); e != nil {
					t.Fatal(e)
				}
				target := "a"
				if kind == "TV" {
					target = "c"
				}
				rule := RecentRule(RecentConfig{Kind: kind, Days: days})
				p, e := (RuleCompiler{DB: s.db, Now: func() time.Time { return now }}).Compile(ctx, rule)
				if e != nil {
					t.Fatal(e)
				}
				for _, tc := range []struct {
					at        int64
					available int
					want      int
				}{{boundary - 1, 1, 0}, {boundary, 1, 1}, {now.Unix(), 1, 1}, {now.Unix() + 1, 1, 0}, {now.Unix(), 0, 0}} {
					if _, e = s.db.Exec("UPDATE plex_media_items SET added_at=?,available=? WHERE id=?", tc.at, tc.available, target); e != nil {
						t.Fatal(e)
					}
					var n int
					if e = s.db.QueryRow("SELECT count(*) FROM plex_media_items m WHERE m.id=? AND ("+p.SQL+")", append([]any{target}, p.Args...)...).Scan(&n); e != nil {
						t.Fatal(e)
					}
					if n != tc.want {
						t.Fatalf("boundary %+v got %d", tc, n)
					}
				}
				var wrong int
				wantType := "movie"
				if kind == "TV" {
					wantType = "episode"
				}
				if e = s.db.QueryRow("SELECT count(*) FROM plex_media_items m WHERE m.media_type<>? AND ("+p.SQL+")", append([]any{wantType}, p.Args...)...).Scan(&wrong); e != nil || wrong != 0 {
					t.Fatalf("crossover %d %v", wrong, e)
				}
			})
		}
	}
}
func TestRecentTemplateCanonicalAndEditable(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	c, e := s.CreateRecent(ctx, "TV", "owner")
	if e != nil {
		t.Fatal(e)
	}
	if c.Source.Recent == nil || c.Source.Recent.Days != 30 {
		t.Fatal(c)
	}
	var weighted, avoid, others int
	if e = s.db.QueryRow("SELECT recency_weighted,avoid_same_show,min_other_shows FROM channel_schedule_settings WHERE channel_id=?", c.ID).Scan(&weighted, &avoid, &others); e != nil || weighted != 1 || avoid != 1 || others != 1 {
		t.Fatal(e, weighted, avoid, others)
	}
	// A tampered rule or unavailable override cannot weaken the template's type boundary.
	c.Source.Rule = &Rule{Version: 1, Root: condition("media_type", "is", "movie")}
	p, e := s.Preview(ctx, PreviewRequest{Source: c.Source, IncludeUnavailable: true})
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range p.Items {
		if m.Type != "episode" {
			t.Fatal(m)
		}
	}
	kinds, e := s.RecentKinds(ctx)
	if e != nil || len(kinds) != 1 || kinds[0] != "TV" {
		t.Fatal(kinds, e)
	}
	for _, days := range []int{0, -1, 3651} {
		c.Source.Recent.Days = days
		if s.ValidateSource(ctx, c.Source) == nil {
			t.Fatal("invalid lookback accepted")
		}
	}
}
