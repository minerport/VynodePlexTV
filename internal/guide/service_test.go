package guide

import (
	"afterglow.local/tv/internal/database"
	"context"
	"fmt"
	"testing"
	"time"
)

func fixture(t *testing.T) *Service {
	t.Helper()
	db, e := database.Open(t.TempDir())
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
	exec("INSERT INTO users VALUES('u','one','unused','USER',1),('v','two','unused','USER',1)")
	exec(`INSERT INTO plex_servers(id,name,base_url,machine_identifier,token_encrypted,token_key_version,created_at,updated_at) VALUES('s','Fixture','http://offline.invalid','s',x'00',1,1,1)`)
	exec(`INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,created_at,updated_at) VALUES('l','s','1','TV','show',1,1)`)
	exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,duration_ms,created_at,updated_at) VALUES('m','s','l','1','movie','Film',3600000,1,1)`)
	for i := 0; i < 4; i++ {
		id := fmt.Sprint(i)
		exec(`INSERT INTO channels(id,channel_number,name,channel_type,group_id,enabled,created_at,updated_at) VALUES(?,?,?,'STATIC','movies',?,1,1)`, id, fmt.Sprint(101+i), "Cinema "+id, i != 2)
		exec(`INSERT INTO channel_sources VALUES(?,'STATIC','{}',1,1)`, id)
		if i == 3 {
			continue
		}
		for n := -1; n < 3; n++ {
			start := int64(2000000000000) + int64(n)*3600000
			exec(`INSERT INTO channel_programs VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, fmt.Sprintf("%s-%d", id, n), id, "m", "m", start, start+3600000, 3600000, n+1, "g", 1, "test", 1)
		}
	}
	s := New(db)
	s.Now = func() time.Time { return time.UnixMilli(2000000000000 + 1800000) }
	return s
}
func TestWindowIntersectionAndClassification(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	base := int64(2000000000000)
	v, e := s.Window(ctx, "u", Query{Start: base, End: base + 3600000})
	if e != nil || len(v.Channels) != 2 {
		t.Fatal(v, e)
	}
	for _, c := range v.Channels {
		if len(c.Programs) != 1 || c.Programs[0].State != "CURRENT" || c.Programs[0].Progress != 50 {
			t.Fatal(c)
		}
	}
	for _, tc := range []struct {
		at    int64
		state string
	}{{base - 1, "PAST"}, {base, "CURRENT"}, {base + 3600000, "FUTURE"}} {
		v, e = s.Window(ctx, "u", Query{Start: tc.at, End: tc.at + 1})
		if e != nil || v.Channels[0].Programs[0].State != tc.state {
			t.Fatal(tc, v, e)
		}
	}
}
func TestGuideFiltersFavoritesAndIsolation(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	if e := s.Favorite(ctx, "u", "0", true); e != nil {
		t.Fatal(e)
	}
	if e := s.Favorite(ctx, "u", "0", true); e != nil {
		t.Fatal(e)
	}
	q := Query{Kind: "now", Favorites: true}
	v, e := s.Window(ctx, "u", q)
	if e != nil || len(v.Channels) != 1 || !v.Channels[0].Favorite {
		t.Fatal(v, e)
	}
	v, e = s.Window(ctx, "v", q)
	if e != nil || len(v.Channels) != 0 {
		t.Fatal(v, e)
	}
	q.Favorites = false
	q.Group = "tv"
	v, e = s.Window(ctx, "u", q)
	if e != nil || len(v.Channels) != 0 {
		t.Fatal(v, e)
	}
	if e = s.Favorite(ctx, "u", "2", true); e == nil {
		t.Fatal("disabled favorite accepted")
	}
	if e = s.Favorite(ctx, "u", "0", false); e != nil {
		t.Fatal(e)
	}
	ids, e := s.Favorites(ctx, "u")
	if e != nil || len(ids) != 0 {
		t.Fatal(ids, e)
	}
}
func TestGuideDetailsCatchupAndAvailability(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		id, state string
		restart   bool
	}{{"0--1", "PAST", true}, {"0-0", "CURRENT", true}, {"0-1", "FUTURE", false}} {
		v, e := s.Detail(ctx, "u", tc.id)
		if e != nil || v.State != tc.state || v.CanRestart != tc.restart {
			t.Fatal(v, e)
		}
	}
	if _, e := s.Detail(ctx, "u", "2-0"); e == nil {
		t.Fatal("disabled exposed")
	}
	s.db.Exec("UPDATE plex_media_items SET available=0")
	v, e := s.Detail(ctx, "u", "0--1")
	if e != nil || v.CanRestart {
		t.Fatal(v, e)
	}
}
func TestGuideNowSoonPaginationAndEmpty(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	v, e := s.Window(ctx, "u", Query{Kind: "now", Limit: 1})
	if e != nil || len(v.Channels) != 1 || !v.More || len(v.Channels[0].Programs) != 2 {
		t.Fatal(v, e)
	}
	v, e = s.Window(ctx, "u", Query{Kind: "soon"})
	if e != nil || len(v.Channels[0].Programs) != 0 {
		t.Fatal(v, e)
	}
	s.Now = func() time.Time { return time.UnixMilli(2000000000000 + 1800001) }
	v, e = s.Window(ctx, "u", Query{Kind: "soon"})
	if e != nil || len(v.Channels[0].Programs) != 1 || v.Channels[0].Programs[0].State != "FUTURE" {
		t.Fatal(v, e)
	}
	v, e = s.Window(ctx, "u", Query{Kind: "now", Limit: 1, Offset: 1})
	if e != nil || v.Channels[0].ID != "1" || v.More {
		t.Fatal(v, e)
	}
}
func TestGuideLastChannelIsolation(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	s.db.Exec("INSERT INTO viewer_preferences VALUES('u','0',1)")
	id, e := s.Last(ctx, "u")
	if e != nil || id != "0" {
		t.Fatal(id, e)
	}
	id, e = s.Last(ctx, "v")
	if e != nil || id != "" {
		t.Fatal(id, e)
	}
	s.db.Exec("UPDATE channels SET enabled=0 WHERE id='0'")
	id, e = s.Last(ctx, "u")
	if e != nil || id != "" {
		t.Fatal(id, e)
	}
}
func TestGuideDSTWindowsAndLimits(t *testing.T) {
	s := fixture(t)
	s.db.Exec("UPDATE app_settings SET timezone='America/New_York'")
	for _, date := range []string{"2026-03-08T06:00:00Z", "2026-11-01T05:00:00Z", "2026-10-18T03:30:00Z"} {
		at, _ := time.Parse(time.RFC3339, date)
		v, e := s.Window(context.Background(), "u", Query{Start: at.UnixMilli(), End: at.Add(3 * time.Hour).UnixMilli()})
		if e != nil || v.End-v.Start != 10800000 || v.Timezone != "America/New_York" {
			t.Fatal(v, e)
		}
	}
	for _, q := range []Query{{Start: 0, End: 86400001}, {Start: 10, End: 10}, {Start: 0, End: 1, Offset: -1}} {
		if _, e := s.Window(context.Background(), "u", q); e == nil {
			t.Fatal("invalid window accepted")
		}
	}
}

func TestGuideRejectsMalformedIntervals(t *testing.T) {
	s := fixture(t)
	s.db.Exec("PRAGMA ignore_check_constraints=ON")
	s.db.Exec("UPDATE channel_programs SET duration_ms=0 WHERE id='0-0'")
	v, e := s.Window(context.Background(), "u", Query{Kind: "now"})
	if e != nil || len(v.Warnings) != 1 {
		t.Fatal(v, e)
	}
	if _, e = s.Detail(context.Background(), "u", "0-0"); e == nil {
		t.Fatal("invalid detail timing accepted")
	}
}
