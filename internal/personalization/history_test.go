package personalization

import (
	"afterglow.local/tv/internal/database"
	"context"
	"fmt"
	"testing"
	"time"
)

func personalFixture(t *testing.T) *Service {
	t.Helper()
	db, e := database.Open(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	_, e = db.Exec(`INSERT INTO users VALUES('a','alice','unused','USER',1),('b','bob','unused','USER',1);
 INSERT INTO plex_servers(id,name,base_url,machine_identifier,token_encrypted,token_key_version,created_at,updated_at) VALUES('s','Fixture','http://offline.invalid','s',x'00',1,1,1);
 INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,created_at,updated_at) VALUES('l','s','1','Movies','movie',1,1);
 INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,duration_ms,created_at,updated_at) VALUES('m','s','l','1','movie','Synthetic Film',3600000,1,1),('n','s','l','2','movie','Other Film',3600000,1,1);
 INSERT INTO metadata_entities(id,plex_server_id,kind,source_key,name) VALUES(1,'s','genre','action','Action'),(2,'s','person','actor','Synthetic Actor'),(3,'s','person','director','Synthetic Director');
 INSERT INTO media_item_entities(media_item_id,entity_id,relationship) VALUES('m',1,'genre'),('m',2,'actor'),('m',3,'director');
 INSERT INTO channels(id,channel_number,name,channel_type,created_at,updated_at) VALUES('one','101','Cinema','STATIC',1,1),('two','102','Other','STATIC',1,1);
 INSERT INTO channel_sources VALUES('one','STATIC','{}',1,1),('two','STATIC','{}',1,1);
 INSERT INTO channel_programs VALUES('p','one','m','m',2000000000000,2000003600000,3600000,0,'g',1,'test',1),('q','two','n','n',2000000000000,2000003600000,3600000,0,'g',1,'test',1);`)
	if e != nil {
		t.Fatal(e)
	}
	return New(db)
}
func TestHeartbeatCredit(t *testing.T) {
	for _, v := range []struct {
		name, state            string
		elapsed, advance, want int64
	}{{"playing", "PLAYING", 15000, 15000, 15000}, {"pause", "PAUSED", 10000, 10000, 10000}, {"duplicate", "PLAYING", 0, 0, 0}, {"seek", "PLAYING", 1000, 900000, 0}, {"disconnected", "PLAYING", 90000, 90000, 0}, {"backwards", "PLAYING", 10000, -30000, 0}, {"error", "ERROR", 10000, 10000, 0}} {
		t.Run(v.name, func(t *testing.T) {
			if got := Credit(100000, 100000+v.elapsed, 0, v.advance, "PLAYING", v.state); got != v.want {
				t.Fatal(got, v.want)
			}
		})
	}
}
func TestHistoryClearAndDuplicateSafety(t *testing.T) {
	s := personalFixture(t)
	ctx := context.Background()
	v := HistoryStart{ID: "h", UserID: "a", ChannelID: "one", ProgramID: "p", MediaID: "m", Origin: "CATCH_UP", ClientType: "browser", At: 1000, Duration: 3600000}
	if e := s.StartHistory(ctx, v); e != nil {
		t.Fatal(e)
	}
	for _, n := range []int64{2000, 17000, 17000, 32000} {
		if e := s.ObserveHistory(ctx, "a", "h", n, n-1000, "PLAYING"); e != nil {
			t.Fatal(e)
		}
	}
	h, e := s.History(ctx, "a", 0)
	if e != nil || len(h) != 1 || h[0].Watched != 30000 || h[0].Origin != "CATCH_UP" {
		t.Fatal(h, e)
	}
	other, e := s.History(ctx, "b", 0)
	if e != nil || len(other) != 0 {
		t.Fatal(other, e)
	}
	if e = s.ClearHistory(ctx, "a"); e != nil {
		t.Fatal(e)
	}
	if e = s.ObserveHistory(ctx, "a", "h", 47000, 46000, "PLAYING"); e != nil {
		t.Fatal(e)
	}
	h, e = s.History(ctx, "a", 0)
	if e != nil || len(h) != 0 {
		t.Fatal("late heartbeat restored cleared history", h, e)
	}
}
func TestRebuildAndGoldenRankingIsolation(t *testing.T) {
	s := personalFixture(t)
	ctx := context.Background()
	now := time.UnixMilli(2000000600000)
	for i := 0; i < 3; i++ {
		id := fmt.Sprint(i)
		e := s.StartHistory(ctx, HistoryStart{ID: id, UserID: "a", ChannelID: "one", ProgramID: "p", MediaID: "m", Origin: "LIVE", ClientType: "browser", At: now.Add(-time.Hour).UnixMilli(), Duration: 3600000})
		if e != nil {
			t.Fatal(e)
		}
		if _, e = s.db.Exec("UPDATE user_program_history SET watch_duration_ms=1800000,completion_ratio=0.5 WHERE id=?", id); e != nil {
			t.Fatal(e)
		}
	}
	if e := s.Rebuild(ctx, "a", now); e != nil {
		t.Fatal(e)
	}
	a, e := s.Affinities(ctx, "a")
	if e != nil || len(a) != 4 {
		t.Fatal(a, e)
	}
	for _, v := range a {
		if v.Sessions != 3 || v.Score != 0.6 {
			t.Fatal(v)
		}
	}
	ranked, e := s.Recommend(ctx, "a", now, true)
	if e != nil || len(ranked.Items) != 2 || ranked.Items[0].Channel.ID != "one" || ranked.ColdStart {
		t.Fatal(ranked, e)
	}
	if _, e = s.db.Exec("INSERT INTO user_channel_favorites VALUES('b','two',1)"); e != nil {
		t.Fatal(e)
	}
	other, e := s.Recommend(ctx, "b", now, true)
	if e != nil || other.Items[0].Channel.ID != "two" || !other.ColdStart || other.Items[0].Reasons[0] != "Favorite channel" {
		t.Fatal(other, e)
	}
	if e = s.Hide(ctx, "a", "one", true); e != nil {
		t.Fatal(e)
	}
	ranked, e = s.Recommend(ctx, "a", now, false)
	if e != nil || len(ranked.Items) != 1 || ranked.Items[0].Channel.ID != "two" || ranked.Items[0].Components != nil {
		t.Fatal(ranked, e)
	}
	if e = s.Rebuild(ctx, "a", now.AddDate(0, 0, 31)); e != nil {
		t.Fatal(e)
	}
	a, e = s.Affinities(ctx, "a")
	if e != nil || a[0].Score >= 0.6 {
		t.Fatal("decay", a, e)
	}
}

func TestShowAffinityAndReplaySuppression(t *testing.T) {
	s := personalFixture(t)
	ctx := context.Background()
	now := time.UnixMilli(2000000600000)
	_, e := s.db.Exec(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,created_at,updated_at) VALUES('show','s','l','show','show','Synthetic Show',1,1);UPDATE plex_media_items SET media_type='episode',grandparent_rating_key='show' WHERE id='m'`)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		id := fmt.Sprint(i)
		if e = s.StartHistory(ctx, HistoryStart{ID: id, UserID: "a", ChannelID: "one", ProgramID: "p", MediaID: "m", Origin: "LIVE", ClientType: "browser", At: now.Add(-time.Hour).UnixMilli(), Duration: 3600000}); e != nil {
			t.Fatal(e)
		}
		if _, e = s.db.Exec("UPDATE user_program_history SET watch_duration_ms=3600000,completion_ratio=1,ended_at=started_at+3600000 WHERE id=?", id); e != nil {
			t.Fatal(e)
		}
	}
	if e = s.Rebuild(ctx, "a", now); e != nil {
		t.Fatal(e)
	}
	ranked, e := s.Recommend(ctx, "a", now, true)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, r := range ranked.Items {
		if r.Channel.ID == "one" {
			found = true
			if r.Components["show"] <= 0 || r.Components["recent_completion"] != -12 {
				t.Fatal(r)
			}
		}
	}
	if !found {
		t.Fatal("episode missing")
	}
}
