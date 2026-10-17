package scheduler

import (
	"afterglow.local/tv/internal/channels"
	"afterglow.local/tv/internal/database"
	"context"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

func serviceFixture(t *testing.T) (*Service, *channels.Service, string) {
	t.Helper()
	dir := t.TempDir()
	db, e := database.Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	q := func(v string, args ...any) {
		t.Helper()
		if _, e := db.Exec(v, args...); e != nil {
			t.Fatal(e)
		}
	}
	q(`INSERT INTO users VALUES('owner','owner','unused','OWNER',1)`)
	q(`INSERT INTO plex_servers(id,name,base_url,machine_identifier,token_encrypted,token_key_version,created_at,updated_at) VALUES('s','Fixture','http://unreachable.invalid','s',x'00',1,1,1)`)
	q(`INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,selected,created_at,updated_at) VALUES('l','s','1','Movies','movie',1,1,1)`)
	for i, id := range []string{"a", "b", "c", "d"} {
		q(`INSERT INTO plex_media_items(id,plex_server_id,plex_library_id,plex_rating_key,media_type,title,duration_ms,created_at,updated_at) VALUES(?,'s','l',?,'movie',?,?,1,1)`, id, id, id, 3600000+int64(i)*1234)
	}
	members := channels.New(db, dir)
	c, e := members.Save(context.Background(), "", channels.Definition{Number: "101", Name: "Test", Enabled: true, Visibility: "VISIBLE", Logo: channels.Appearance{Source: "AUTO"}, Background: channels.Appearance{Source: "AUTO"}, Source: channels.Source{Type: "STATIC", Static: &channels.StaticConfig{MediaIDs: []string{"c", "a", "b"}}}}, "owner")
	if e != nil {
		t.Fatal(e)
	}
	s := New(db, members, slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.Now = func() time.Time { return time.Date(2026, 10, 17, 12, 30, 0, 0, time.UTC) }
	v, e := s.Settings(context.Background(), c.ID)
	if e != nil {
		t.Fatal(e)
	}
	v.Anchor = s.Now().Add(-24 * time.Hour).UnixMilli()
	v.PastDays = 7
	v.FutureDays = 14
	v.Seed = "stable"
	if _, e = s.SaveSettings(context.Background(), c.ID, v, "owner"); e != nil {
		t.Fatal(e)
	}
	return s, members, c.ID
}
func all(t *testing.T, s *Service, id string) []Entry {
	t.Helper()
	v, e := s.Programs(context.Background(), id, s.Now().Add(-7*24*time.Hour).UnixMilli(), s.Now().Add(15*24*time.Hour).UnixMilli(), 0, 500)
	if e != nil {
		t.Fatal(e)
	}
	return v.Items
}
func checkIntegrity(t *testing.T, s *Service, id string) {
	t.Helper()
	d, e := s.Diagnostics(context.Background(), id)
	if e != nil || len(d) != 0 {
		t.Fatal(d, e)
	}
}
func TestRecentMembershipRepairProtectsCurrent(t *testing.T) {
	s, members, _ := serviceFixture(t)
	ctx := context.Background()
	// Membership uses the real inventory clock; scheduling clock is fixed for repeatability.
	now := time.Now().Unix()
	if _, e := s.db.Exec("UPDATE plex_media_items SET added_at=?", now); e != nil {
		t.Fatal(e)
	}
	c, e := members.CreateRecent(ctx, "MOVIES", "owner")
	if e != nil {
		t.Fatal(e)
	}
	v, e := s.Settings(ctx, c.ID)
	if e != nil {
		t.Fatal(e)
	}
	v.Anchor = s.Now().Add(-24 * time.Hour).UnixMilli()
	if _, e = s.SaveSettings(ctx, c.ID, v, "owner"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Repair(ctx, c.ID, ""); e != nil {
		t.Fatal(e)
	}
	before := all(t, s, c.ID)
	if _, e = s.db.Exec("UPDATE plex_media_items SET added_at=? WHERE id='d'", now-31*86400); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Repair(ctx, c.ID, ""); e != nil {
		t.Fatal(e)
	}
	after := all(t, s, c.ID)
	for i, p := range after {
		if p.Start <= s.Now().UnixMilli() {
			if !reflect.DeepEqual(p, before[i]) {
				t.Fatal("history changed")
			}
		} else if p.MediaID == "d" {
			t.Fatal("aged-out item in future")
		}
	}
	if _, e = s.db.Exec("UPDATE plex_media_items SET added_at=? WHERE id='d'", now); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Repair(ctx, c.ID, ""); e != nil {
		t.Fatal(e)
	}
	found := false
	for _, p := range all(t, s, c.ID) {
		if p.Start > s.Now().UnixMilli() && p.MediaID == "d" {
			found = true
		}
	}
	if !found {
		t.Fatal("new eligible item starved")
	}
	checkIntegrity(t, s, c.ID)
}
func TestServicePersistenceBoundariesAndExtension(t *testing.T) {
	ctx := context.Background()
	s, _, id := serviceFixture(t)
	r, e := s.EnsureScheduleCoverage(ctx, id, s.Now().Add(24*time.Hour))
	if e != nil || r.Generated == 0 {
		t.Fatal(r, e)
	}
	before := all(t, s, id)
	if before[0].MediaID != "c" || before[1].MediaID != "a" {
		t.Fatal("static order")
	}
	restarted := New(s.db, s.members, s.log)
	restarted.Now = s.Now
	r, e = restarted.EnsureScheduleCoverage(ctx, id, s.Now().Add(24*time.Hour))
	if e != nil || r.Generated != 0 || r.Repaired != 0 {
		t.Fatal(r, e)
	}
	if !reflect.DeepEqual(before, all(t, s, id)) {
		t.Fatal("restart changed timeline")
	}
	for _, p := range before[:3] {
		at, e := s.GetProgramAt(ctx, id, time.UnixMilli(p.Start))
		if e != nil || at.ID != p.ID {
			t.Fatal("start boundary", e)
		}
		next, e := s.GetProgramAt(ctx, id, time.UnixMilli(p.End))
		if e != nil || next.Start != p.End {
			t.Fatal("end boundary", e)
		}
		prev, e := s.Previous(ctx, id, time.UnixMilli(p.End))
		if e != nil || prev.ID != p.ID {
			t.Fatal("previous", e)
		}
		n, e := s.Next(ctx, id, time.UnixMilli(p.Start))
		if e != nil || n.Start != p.End {
			t.Fatal("next", e)
		}
	}
	if _, e = s.EnsureScheduleCoverage(ctx, id, s.Now().Add(14*24*time.Hour)); e != nil {
		t.Fatal(e)
	}
	after := all(t, s, id)
	if !reflect.DeepEqual(before, after[:len(before)]) {
		t.Fatal("extension changed prior rows")
	}
	checkIntegrity(t, s, id)
}
func TestFutureRepairProtectsHistoryAndCurrent(t *testing.T) {
	for _, kind := range []string{"unavailable", "duration", "remove", "add", "gap", "overlap", "reorder"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			s, m, id := serviceFixture(t)
			if _, e := s.Repair(ctx, id, ""); e != nil {
				t.Fatal(e)
			}
			before := all(t, s, id)
			current, e := s.GetProgramAt(ctx, id, s.Now())
			if e != nil || current == nil {
				t.Fatal(e)
			}
			switch kind {
			case "unavailable":
				_, e = s.db.Exec("UPDATE plex_media_items SET available=0 WHERE id='a'")
			case "duration":
				_, e = s.db.Exec("UPDATE plex_media_items SET duration_ms=duration_ms+50000 WHERE id='a'")
			case "gap", "overlap":
				delta := int64(10)
				if kind == "overlap" {
					delta = -10
				}
				_, e = s.db.Exec("UPDATE channel_programs SET start_time_utc=start_time_utc+?,end_time_utc=end_time_utc+? WHERE channel_id=? AND sequence_number=?", delta, delta, id, current.Sequence+1)
			default:
				c, err := m.Get(ctx, id)
				if err != nil {
					t.Fatal(err)
				}
				switch kind {
				case "remove":
					c.Source.Static.MediaIDs = []string{"c", "b"}
				case "add":
					c.Source.Static.MediaIDs = append(c.Source.Static.MediaIDs, "d")
				case "reorder":
					c.Source.Static.MediaIDs = []string{"b", "a", "c"}
				}
				_, e = m.Save(ctx, id, c.Definition, "owner")
			}
			if e != nil {
				t.Fatal(e)
			}
			r, e := s.Repair(ctx, id, "owner")
			if e != nil || r.Repaired == 0 {
				t.Fatal(r, e)
			}
			after := all(t, s, id)
			for i, p := range before {
				if p.Start <= s.Now().UnixMilli() && (p.ID != after[i].ID || p.Start != after[i].Start || p.End != after[i].End || p.Sequence != after[i].Sequence) {
					t.Fatal("protected entry changed")
				}
			}
			checkIntegrity(t, s, id)
			found := false
			for _, p := range after {
				if p.Start > current.End {
					if (kind == "unavailable" || kind == "remove") && p.MediaID == "a" {
						t.Fatal("removed item retained")
					}
					if p.MediaID == "d" {
						found = true
					}
				}
			}
			if kind == "add" && !found {
				t.Fatal("new item missing")
			}
		})
	}
}
func TestEmptyDisabledInvalidDurationAndRollback(t *testing.T) {
	ctx := context.Background()
	s, _, id := serviceFixture(t)
	s.db.Exec("UPDATE plex_media_items SET duration_ms=NULL")
	r, e := s.Repair(ctx, id, "")
	if e != nil || r.Status != "EMPTY" {
		t.Fatal(r, e)
	}
	st, e := s.Status(ctx, id)
	if e != nil || st.Excluded != 3 {
		t.Fatal(st, e)
	}
	s.db.Exec("UPDATE channels SET enabled=0")
	r, e = s.Repair(ctx, id, "")
	if e != nil || r.Status != "DISABLED" {
		t.Fatal(r, e)
	}
	s.db.Exec("UPDATE channels SET enabled=1")
	s.db.Exec("UPDATE plex_media_items SET duration_ms=3600000")
	s.Repair(ctx, id, "")
	before := all(t, s, id)
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = s.RegenerateFuture(ctx, id, ""); e == nil {
		t.Fatal("cancel not reported")
	}
	if !reflect.DeepEqual(before, all(t, s, id)) {
		t.Fatal("rollback lost rows")
	}
}
func TestJobsDuplicationAndCascade(t *testing.T) {
	ctx := context.Background()
	s, m, id := serviceFixture(t)
	job, e := s.Enqueue(ctx, id, "SCHEDULE_GENERATE", "owner")
	if e != nil {
		t.Fatal(e)
	}
	duplicate, e := s.Enqueue(ctx, id, "SCHEDULE_REPAIR", "owner")
	if e != nil || job != duplicate {
		t.Fatal("duplicate active job", e)
	}
	worked, e := s.RunNext(ctx)
	if e != nil || !worked {
		t.Fatal(e)
	}
	jobs, e := s.Jobs(ctx, id)
	if e != nil || jobs[0].Status != "SUCCEEDED" || jobs[0].Processed == 0 {
		t.Fatal(jobs, e)
	}
	copy, e := m.Duplicate(ctx, id, "owner")
	if e != nil {
		t.Fatal(e)
	}
	a, _ := s.Settings(ctx, id)
	b, _ := s.Settings(ctx, copy.ID)
	if a.Seed == b.Seed || a.Anchor != b.Anchor || a.Mode != b.Mode {
		t.Fatal("duplicate settings")
	}
	if len(all(t, s, copy.ID)) != 0 {
		t.Fatal("copied rows")
	}
	if e = m.Delete(ctx, id, "owner"); e != nil {
		t.Fatal(e)
	}
	var n int
	s.db.QueryRow("SELECT COUNT(*) FROM channel_programs WHERE channel_id=?", id).Scan(&n)
	if n != 0 {
		t.Fatal("cascade")
	}
	s.db.QueryRow("SELECT COUNT(*) FROM plex_media_items").Scan(&n)
	if n != 4 {
		t.Fatal("inventory deleted")
	}
}

func TestMetadataOnlyUpdatesDoNotRegenerate(t *testing.T) {
	ctx := context.Background()
	s, _, id := serviceFixture(t)
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	before := all(t, s, id)
	if _, e := s.db.Exec("UPDATE plex_media_items SET title='New title',thumb_path='/new-art'"); e != nil {
		t.Fatal(e)
	}
	r, e := s.Repair(ctx, id, "")
	if e != nil || r.Generated != 0 || r.Repaired != 0 {
		t.Fatal(r, e)
	}
	after := all(t, s, id)
	for i, p := range before {
		if p.ID != after[i].ID || p.Start != after[i].Start || p.End != after[i].End {
			t.Fatal("cosmetic metadata shifted programs")
		}
	}
}
func TestSmartHistorySurvivesPruningAndServiceRestart(t *testing.T) {
	ctx := context.Background()
	s, _, id := serviceFixture(t)
	v, _ := s.Settings(ctx, id)
	v.Mode = "SMART_RANDOM"
	v.PastDays = 0
	v.MovieDays = 30
	if _, e := s.SaveSettings(ctx, id, v, ""); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	var n int
	if e := s.db.QueryRow("SELECT count(*) FROM channel_schedule_history WHERE channel_id=?", id).Scan(&n); e != nil || n != 3 {
		t.Fatal(n, e)
	}
	before := all(t, s, id)
	oldNow := s.Now()
	s.Now = func() time.Time { return oldNow.Add(24 * time.Hour) }
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	after := all(t, s, id)
	persisted := map[string]Entry{}
	for _, p := range after {
		persisted[p.ID] = p
	}
	for _, p := range before {
		if p.End >= s.Now().UnixMilli() {
			if got, ok := persisted[p.ID]; !ok || got.Start != p.Start || got.End != p.End {
				t.Fatal("pruning changed retained rows")
			}
		}
	}
	checkIntegrity(t, s, id)
}
func TestFailedGenerationLeavesFutureIntact(t *testing.T) {
	ctx := context.Background()
	s, _, id := serviceFixture(t)
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	before := all(t, s, id)
	s.db.Exec("UPDATE plex_media_items SET duration_ms=1")
	if _, e := s.RegenerateFuture(ctx, id, ""); e == nil {
		t.Fatal("expected bounded generation failure")
	}
	after := all(t, s, id)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed transaction changed timeline")
	}
}

func TestWorkerRecoveryAndQueueIsolation(t *testing.T) {
	ctx := context.Background()
	s, _, id := serviceFixture(t)
	job, e := s.Enqueue(ctx, id, "SCHEDULE_GENERATE", "")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Enqueue(ctx, id, "SCHEDULE_REGENERATE", ""); e == nil {
		t.Fatal("regeneration silently coalesced into extension")
	}
	s.db.Exec("UPDATE jobs SET status='RUNNING' WHERE id=?", job)
	s.db.Exec("INSERT INTO jobs(id,type,status,created_at) VALUES('plex-running','PLEX_HEALTH_CHECK','RUNNING',1)")
	s.db.Exec("UPDATE channels SET enabled=0 WHERE id=?", id)
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var status string
		s.db.QueryRow("SELECT status FROM jobs WHERE id=?", job).Scan(&status)
		if status == "CANCELLED" {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("restart did not cancel interrupted job")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	var status string
	s.db.QueryRow("SELECT status FROM jobs WHERE id='plex-running'").Scan(&status)
	if status != "RUNNING" {
		t.Fatal("scheduler changed Plex job")
	}
	s.db.Exec("UPDATE jobs SET status='QUEUED' WHERE id='plex-running'")
	worked, e := s.RunNext(context.Background())
	if e != nil || worked {
		t.Fatal("scheduler claimed Plex job", e)
	}
}
