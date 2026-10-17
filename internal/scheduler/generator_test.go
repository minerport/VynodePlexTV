package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

func fixtureItems() []Item {
	return []Item{{ID: "a", Duration: 60123, Type: "movie"}, {ID: "b", Duration: 125009, Type: "movie"}, {ID: "c", Duration: 42001, Type: "episode", Show: "show", Season: 1}}
}
func TestSequentialExactLoopAndContinuation(t *testing.T) {
	items := fixtureItems()
	r := Request{Settings: Settings{Mode: "SEQUENTIAL"}, Items: items, From: 1234, Through: 1000000}
	all, e := Generate(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	for i, p := range all {
		if p.MediaID != items[i%3].ID || p.End-p.Start != items[i%3].Duration {
			t.Fatal(p)
		}
		if i > 0 && p.Start != all[i-1].End {
			t.Fatal("gap")
		}
	}
	split := 3
	r.Through = all[split-1].End
	first, _ := Generate(context.Background(), r)
	r.From = first[len(first)-1].End
	r.Sequence = int64(split)
	r.Through = 1000000
	rest, _ := Generate(context.Background(), r)
	if !reflect.DeepEqual(all, append(first, rest...)) {
		t.Fatal("extension changed sequence")
	}
}
func TestRandomDeterminismCyclesAndRestart(t *testing.T) {
	items := fixtureItems()
	r := Request{Settings: Settings{Mode: "RANDOM", Seed: "one"}, Items: items, Through: 3000000}
	a, _ := Generate(context.Background(), r)
	b, _ := Generate(context.Background(), r)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("unstable")
	}
	for i := 0; i+3 <= len(a); i += 3 {
		seen := map[string]bool{}
		for _, p := range a[i : i+3] {
			seen[p.MediaID] = true
		}
		if len(seen) != 3 {
			t.Fatal("cycle incomplete")
		}
	}
	for i := 1; i < len(a); i++ {
		if a[i].MediaID == a[i-1].MediaID {
			t.Fatal("immediate repeat")
		}
	}
	different := false
	for i := 0; i < 10; i++ {
		r.Settings.Seed = fmt.Sprint(i)
		b, _ = Generate(context.Background(), r)
		if !reflect.DeepEqual(a, b) {
			different = true
		}
	}
	if !different {
		t.Fatal("seed ineffective")
	}
}
func TestSmartWindowsRelaxationAndClustering(t *testing.T) {
	s := Settings{Mode: "SMART_RANDOM", Seed: "test", MovieDays: 14, EpisodeDays: 30}
	items := fixtureItems()
	at := int64(60) * dayMS
	cases := []struct {
		name     string
		items    []Item
		history  map[string]int64
		previous *Item
		want     string
	}{{"outside movie window", items, map[string]int64{"a": at - 15*dayMS, "b": at - dayMS, "c": at - 20*dayMS}, nil, "a"}, {"oldest fallback", items, map[string]int64{"a": at - dayMS, "b": at - 2*dayMS, "c": at - 3*dayMS}, nil, "c"}, {"avoid immediate", items, map[string]int64{"a": 1, "b": 2, "c": 3}, &items[0], "b"}, {"single item", items[:1], map[string]int64{"a": at}, &items[0], "a"}, {"unseen preferred", items, map[string]int64{"a": 1, "b": 2}, nil, "c"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := smart(tc.items, tc.history, tc.previous, at, 7, s)
			if m.ID != tc.want {
				t.Fatalf("got %s want %s", m.ID, tc.want)
			}
		})
	}
	eps := []Item{{ID: "same", Show: "show", Season: 1}, {ID: "other", Show: "other", Season: 1}}
	m, _ := smart(eps, map[string]int64{}, &eps[0], at, 0, s)
	if m.ID != "other" {
		t.Fatal("clustering")
	}
}
func TestSmartPersistedHistoryContinuation(t *testing.T) {
	for _, n := range []int{1, 2, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			items := []Item{}
			for i := 0; i < n; i++ {
				items = append(items, Item{ID: fmt.Sprint(i), Type: "episode", Duration: 3600000})
			}
			r := Request{Settings: Settings{Mode: "SMART_RANDOM", Seed: "seed", EpisodeDays: 30}, Items: items, From: 1000, Through: 48 * 3600000}
			all, e := Generate(context.Background(), r)
			if e != nil {
				t.Fatal(e)
			}
			r.Through = all[23].End
			first, _ := Generate(context.Background(), r)
			r.From = first[23].End
			r.Sequence = 24
			r.Through = 48 * 3600000
			r.History = map[string]int64{}
			for _, p := range first {
				r.History[p.MediaID] = p.Start
			}
			for _, m := range items {
				if m.ID == first[23].MediaID {
					copy := m
					r.Previous = &copy
				}
			}
			rest, _ := Generate(context.Background(), r)
			if !reflect.DeepEqual(all, append(first, rest...)) {
				t.Fatal("history restart changed sequence")
			}
		})
	}
}
func TestDSTAndMidnight(t *testing.T) {
	for _, tc := range []struct {
		day   string
		hours int
	}{{"2026-03-08", 23}, {"2026-11-01", 25}, {"2026-10-17", 24}} {
		a, b, e := DayRange(tc.day, "America/New_York")
		if e != nil || int(b.Sub(a)/time.Hour) != tc.hours || a.Location() != time.UTC {
			t.Fatal(tc, e)
		}
		r := Request{Settings: Settings{Mode: "SEQUENTIAL"}, Items: []Item{{ID: "long", Duration: 9000001}}, From: a.UnixMilli(), Through: b.UnixMilli()}
		p, e := Generate(context.Background(), r)
		if e != nil {
			t.Fatal(e)
		}
		for i := 1; i < len(p); i++ {
			if p[i].Start != p[i-1].End {
				t.Fatal("DST gap")
			}
		}
	}
}
func TestInvalidEmptyAndCancelled(t *testing.T) {
	for _, d := range []int64{0, -1, 8 * dayMS} {
		_, e := Generate(context.Background(), Request{Settings: Settings{Mode: "SEQUENTIAL"}, Items: []Item{{ID: "bad", Duration: d}}, Through: 1})
		if e == nil {
			t.Fatal("accepted bad duration")
		}
	}
	p, e := Generate(context.Background(), Request{})
	if e != nil || len(p) != 0 {
		t.Fatal("empty")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = Generate(ctx, Request{Settings: Settings{Mode: "SEQUENTIAL"}, Items: fixtureItems(), Through: 1})
	if e != context.Canceled {
		t.Fatal(e)
	}
}
