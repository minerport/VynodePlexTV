package scheduler

import (
	"afterglow.local/tv/internal/interstitials"
	"context"
	"sort"
	"strconv"
)

type Request struct {
	Presentation            interstitials.Snapshot
	AssetHistory            map[string]int64
	PreviousStart           int64
	Settings                Settings
	Items                   []Item
	From, Through, Sequence int64
	History                 map[string]int64
	Previous                *Item
	Shows                   []string
}

// Generate is pure apart from cancellation. History contains persisted air starts before From.
func Generate(ctx context.Context, r Request) ([]Entry, error) {
	out := []Entry{}
	if len(r.Items) == 0 {
		return out, nil
	}
	for _, m := range r.Items {
		if m.Duration <= 0 || m.Duration > 7*dayMS {
			return nil, invalid("Unusable media duration")
		}
	}
	if r.Settings.Mode != "SEQUENTIAL" && r.Settings.Mode != "RANDOM" && r.Settings.Mode != "SMART_RANDOM" {
		return nil, invalid("Unknown scheduling mode")
	}
	history := map[string]int64{}
	for id, t := range r.History {
		history[id] = t
	}
	order := append([]Item{}, r.Items...)
	// A seeded permutation repeats as complete cycles. Last and first are distinct for N>1.
	if r.Settings.Mode == "RANDOM" {
		sort.Slice(order, func(i, j int) bool {
			return hash([]string{r.Settings.Seed, order[i].ID}) < hash([]string{r.Settings.Seed, order[j].ID})
		})
	}
	previous := r.Previous
	shows := append([]string{}, r.Shows...)
	assetHistory := map[string]int64{}
	for id, t := range r.AssetHistory {
		assetHistory[id] = t
	}
	previousStart := r.PreviousStart
	at := r.From
	seq := r.Sequence
	for at < r.Through {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		if len(out) >= maxEntries {
			return nil, invalid("Schedule exceeds 100,000 entries; shorten horizon or correct very short media")
		}
		m := order[seq%int64(len(order))]
		reason := "sequential order"
		if r.Settings.Mode == "RANDOM" {
			reason = "seeded complete cycle"
			if previous != nil && m.ID == previous.ID && len(order) > 1 {
				m = order[(seq+1)%int64(len(order))]
			}
		}
		if r.Settings.Mode == "SMART_RANDOM" {
			m, reason = smart(r.Items, history, previous, at, seq, r.Settings)
			if r.Settings.RecencyWeighted || r.Settings.AvoidSameShow || r.Settings.MinOtherShows > 0 {
				m, reason = distributed(r.Items, history, previous, shows, at, seq, r.Settings)
			}
		}
		clips := r.Presentation.Select(r.Settings.Seed, m.Type, m.ID, seq, at, previousStart, r.Settings.Timezone, assetHistory)
		if len(clips) > 0 {
			at = clips[len(clips)-1].End
		}
		previousStart = at
		out = append(out, Entry{Presentation: clips, MediaID: m.ID, Start: at, End: at + m.Duration, Duration: m.Duration, Sequence: seq, Reason: reason})
		history[m.ID] = at
		shows = rememberShow(shows, m.Show)
		previous = &m
		at += m.Duration
		seq++
	}
	return out, nil
}
func smart(items []Item, history map[string]int64, previous *Item, at, seq int64, s Settings) (Item, string) {
	best := items[0]
	bestTier := 99
	bestLast := int64(0)
	bestCluster := 99
	bestTie := ""
	for _, m := range items {
		last, seen := history[m.ID]
		window := s.MovieDays
		if m.Type == "episode" {
			window = s.EpisodeDays
		}
		tier := 0
		if seen && at-last < int64(window)*dayMS {
			tier = 1
		}
		if previous != nil && previous.ID == m.ID && len(items) > 1 {
			tier = 2
		}
		if !seen {
			last = -1 << 62
		}
		cluster := 0
		if previous != nil && m.Show != "" && m.Show == previous.Show {
			cluster = 1
			if m.Season == previous.Season {
				cluster = 2
			}
		}
		tie := hash([]string{s.Seed, strconv.FormatInt(seq, 10), m.ID})
		if tier < bestTier || tier == bestTier && (last < bestLast || last == bestLast && (cluster < bestCluster || cluster == bestCluster && (bestTie == "" || tie < bestTie))) {
			best = m
			bestTier = tier
			bestLast = last
			bestCluster = cluster
			bestTie = tie
		}
	}
	reason := "outside repeat window; oldest first"
	if bestTier == 1 {
		reason = "repeat window relaxed; oldest non-immediate item"
	}
	if len(items) == 1 {
		reason = "single item; repeats unavoidable"
	}
	return best, reason
}
