package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

func TestRecentDistributionCoverageAndContinuation(t *testing.T) {
	items := []Item{}
	lookup := map[string]Item{}
	for j, count := range []int{20, 8, 4} {
		for i := 0; i < count; i++ {
			m := Item{ID: fmt.Sprintf("%d-%d", j, i), Show: fmt.Sprint(j), Type: "episode", Duration: 1800000, AddedAt: 2000000000 - int64(j)*86400}
			items = append(items, m)
			lookup[m.ID] = m
		}
	}
	r := Request{Settings: Settings{Mode: "SMART_RANDOM", Seed: "recent", EpisodeDays: 30, RecencyWeighted: true, AvoidSameShow: true, MinOtherShows: 1}, Items: items, Through: 64 * 1800000}
	all, e := Generate(context.Background(), r)
	if e != nil {
		t.Fatal(e)
	}
	again, _ := Generate(context.Background(), r)
	if !reflect.DeepEqual(all, again) {
		t.Fatal("non deterministic")
	}
	seen := map[string]bool{}
	for i, p := range all[:32] {
		seen[p.MediaID] = true
		if i > 0 && i < 9 && lookup[p.MediaID].Show == lookup[all[i-1].MediaID].Show {
			t.Fatal("early clustering despite alternatives")
		}
	}
	if len(seen) != 32 {
		t.Fatal("eligible episodes starved", len(seen))
	}
	split := 17
	r.History = map[string]int64{}
	for _, p := range all[:split] {
		r.History[p.MediaID] = p.Start
		r.Shows = rememberShow(r.Shows, lookup[p.MediaID].Show)
	}
	prev := lookup[all[split-1].MediaID]
	r.Previous = &prev
	r.From = all[split-1].End
	r.Sequence = int64(split)
	rest, e := Generate(context.Background(), r)
	if e != nil || !reflect.DeepEqual(all[split:], rest) {
		t.Fatal("continuation reshuffled", e)
	}
}
func TestRecentRecencyAndRelaxation(t *testing.T) {
	s := Settings{Mode: "SMART_RANDOM", Seed: "s", MovieDays: 14, EpisodeDays: 30, RecencyWeighted: true, AvoidSameShow: true}
	items := []Item{{ID: "older", Type: "movie", Duration: 60000, AddedAt: 86400}, {ID: "new", Type: "movie", Duration: 60000, AddedAt: 172800}}
	m, _ := distributed(items, map[string]int64{}, nil, nil, dayMS, 0, s)
	if m.ID != "new" {
		t.Fatal("new item not preferred")
	}
	m, _ = distributed(items, map[string]int64{"new": dayMS}, nil, nil, dayMS+60000, 1, s)
	if m.ID != "older" {
		t.Fatal("repeat window ignored")
	}
	for _, n := range []int{1, 2} {
		out, e := Generate(context.Background(), Request{Settings: s, Items: items[:n], Through: 600000})
		if e != nil || len(out) != 10 {
			t.Fatal("relaxation deadlock", e)
		}
		for i := 1; i < len(out); i++ {
			if n > 1 && out[i].MediaID == out[i-1].MediaID {
				t.Fatal("avoidable repeat")
			}
		}
	}
}
