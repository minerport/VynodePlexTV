package scheduler

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestSeasonalWindowStableRepairAndExpiry(t *testing.T) {
	s, _, id := serviceFixture(t)
	ctx := context.Background()
	start := s.Now().Add(-time.Hour).UnixMilli()
	end := s.Now().Add(2 * time.Hour).UnixMilli()
	if _, e := s.db.Exec(`INSERT INTO channel_activation_windows VALUES(?,'2026-10-17','2026-10-17','UTC',?,?)`, id, start, end); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	before := all(t, s, id)
	if len(before) == 0 || before[0].Start != start {
		t.Fatal(before)
	}
	for _, p := range before {
		if p.Start >= end {
			t.Fatal("program started after expiration", p)
		}
	}
	checkIntegrity(t, s, id)
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, all(t, s, id)) {
		t.Fatal("repair changed stable window")
	}
	s.Now = func() time.Time { return time.UnixMilli(end + 7200000) }
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(before, all(t, s, id)) {
		t.Fatal("expiry rewrote history")
	}
	status, e := s.Status(ctx, id)
	if e != nil || status.Status != "DORMANT" {
		t.Fatal(status, e)
	}
}

func TestFutureActivationCanMoveIntoHorizon(t *testing.T) {
	s, _, id := serviceFixture(t)
	ctx := context.Background()
	far := s.Now().AddDate(1, 0, 0).UnixMilli()
	if _, e := s.db.Exec(`INSERT INTO channel_activation_windows VALUES(?,'2027-10-17','2027-10-18','UTC',?,?)`, id, far, far+86400000); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	if len(all(t, s, id)) != 0 {
		t.Fatal("generated beyond horizon")
	}
	start := s.Now().Add(time.Hour).UnixMilli()
	if _, e := s.db.Exec("UPDATE channel_activation_windows SET start_ms=?,end_ms=? WHERE channel_id=?", start, start+86400000, id); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Repair(ctx, id, ""); e != nil {
		t.Fatal(e)
	}
	items := all(t, s, id)
	if len(items) == 0 || items[0].Start != start {
		t.Fatal("stale coverage prevented activation", items)
	}
	checkIntegrity(t, s, id)
}
