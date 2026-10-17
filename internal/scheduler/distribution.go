package scheduler

func rememberShow(shows []string, show string) []string {
	if show == "" {
		return shows
	}
	out := []string{}
	for _, v := range shows {
		if v != show {
			out = append(out, v)
		}
	}
	out = append(out, show)
	if len(out) > 10 {
		out = out[len(out)-10:]
	}
	return out
}

// Repeat eligibility is strongest. Among equally eligible items, prefer shows outside
// the distinct-show cooldown, then unseen/oldest media. Recency ranks unseen items
// by UTC added-day, keeping a large import from starving previously unplayed media.
func distributed(items []Item, history map[string]int64, previous *Item, shows []string, at, seq int64, s Settings) (Item, string) {
	type rank struct {
		tier, show  int
		seen        bool
		last, added int64
		tie         string
	}
	best := items[0]
	var br rank
	less := func(a, b rank) bool {
		if a.tier != b.tier {
			return a.tier < b.tier
		}
		if a.show != b.show {
			return a.show < b.show
		}
		if a.seen != b.seen {
			return !a.seen
		}
		if a.seen && a.last != b.last {
			return a.last < b.last
		}
		if a.added != b.added {
			return a.added > b.added
		}
		return a.tie < b.tie
	}
	n := s.MinOtherShows
	if s.AvoidSameShow && n < 1 {
		n = 1
	}
	for i, m := range items {
		last, seen := history[m.ID]
		r := rank{seen: seen, last: last, tie: hash([]any{s.Seed, seq, m.ID})}
		days := s.MovieDays
		if m.Type == "episode" {
			days = s.EpisodeDays
		}
		if seen && at-last < int64(days)*dayMS {
			r.tier = 1
		}
		if previous != nil && previous.ID == m.ID && len(items) > 1 {
			r.tier = 2
		}
		if m.Show != "" && n > 0 {
			for j := len(shows) - 1; j >= 0 && j >= len(shows)-n; j-- {
				if shows[j] == m.Show {
					r.show = n - (len(shows) - 1 - j)
					break
				}
			}
		}
		if s.RecencyWeighted {
			r.added = m.AddedAt / 86400
		}
		if i == 0 || less(r, br) {
			best = m
			br = r
		}
	}
	reason := "recency and show distribution"
	if br.tier > 0 {
		reason += "; repeat window relaxed"
	}
	if br.show > 0 {
		reason += "; show separation relaxed"
	}
	return best, reason
}
