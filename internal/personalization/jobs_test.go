package personalization

import (
	"context"
	"testing"
	"time"
)

func TestRebuildJobLifecycleAndDeduplication(t *testing.T) {
	s := personalFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id, e := s.Enqueue(ctx, "a")
	if e != nil {
		t.Fatal(e)
	}
	again, e := s.Enqueue(ctx, "a")
	if e != nil || again != id {
		t.Fatal(again, e)
	}
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	var state string
	for time.Now().Before(deadline) {
		if e = s.db.QueryRow("SELECT status FROM jobs WHERE id=?", id).Scan(&state); e != nil {
			t.Fatal(e)
		}
		if state == "SUCCEEDED" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	if state != "SUCCEEDED" {
		t.Fatal(state)
	}
	var users int
	if e = s.db.QueryRow("SELECT COUNT(*) FROM personalization_state WHERE user_id='b'").Scan(&users); e != nil || users != 0 {
		t.Fatal("rebuild crossed user boundary", users, e)
	}
}
