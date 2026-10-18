package plex

import (
	"context"
	"testing"
)

func TestSuccessfulSyncQueuesDeduplicatedChannelRefresh(t *testing.T) {
	s, f := newTestService(t)
	_, libs := connected(t, s, f)
	l := libraryOf(t, libs, "movie")
	for _, id := range []string{"enabled", "disabled"} {
		_, e := s.db.Exec(`INSERT INTO channels(id,channel_number,name,channel_type,enabled,created_at,updated_at) VALUES(?,?,?,'RULE',?,0,0)`, id, id, id, id == "enabled")
		if e != nil {
			t.Fatal(e)
		}
		_, e = s.db.Exec(`INSERT INTO channel_schedule_settings(channel_id,mode,timezone,anchor_ms,seed,created_at,updated_at) VALUES(?,'SMART_RANDOM','UTC',0,'test',0,0)`, id)
		if e != nil {
			t.Fatal(e)
		}
	}
	for i := 0; i < 2; i++ {
		if j := syncTest(t, s, l, "incremental"); j.Status != "SUCCEEDED" {
			t.Fatal(j.Error)
		}
	}
	var count int
	s.db.QueryRow(`SELECT count(*) FROM jobs WHERE type='SCHEDULE_REPAIR' AND channel_id='enabled' AND status='QUEUED'`).Scan(&count)
	if count != 1 {
		t.Fatalf("queued %d repairs", count)
	}
	s.db.QueryRow(`SELECT count(*) FROM jobs WHERE type='SCHEDULE_REPAIR' AND channel_id='disabled'`).Scan(&count)
	if count != 0 {
		t.Fatal("disabled channel queued")
	}
	s.db.Exec(`DELETE FROM jobs WHERE type='SCHEDULE_REPAIR'`)
	f.mu.Lock()
	f.offline = true
	f.mu.Unlock()
	if j := syncTest(t, s, l, "incremental"); j.Status != "FAILED" {
		t.Fatal("expected outage")
	}
	s.db.QueryRow(`SELECT count(*) FROM jobs WHERE type='SCHEDULE_REPAIR'`).Scan(&count)
	if count != 0 {
		t.Fatal("failed scan queued repair")
	}
	options, e := s.Settings(context.Background())
	if e != nil || options.SyncIntervalSeconds != 900 {
		t.Fatal("refresh default", options, e)
	}
}
