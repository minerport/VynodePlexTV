package personalization

import (
	"afterglow.local/tv/internal/guide"
	"context"
	"sort"
	"strings"
	"time"
)

type Recommendation struct {
	Channel    guide.Channel      `json:"channel"`
	Program    guide.Program      `json:"program"`
	Reasons    []string           `json:"reasons"`
	Score      float64            `json:"score,omitempty"`
	Components map[string]float64 `json:"components,omitempty"`
}
type Recommendations struct {
	Items      []Recommendation `json:"items"`
	Version    int              `json:"rankingVersion"`
	ServerTime int64            `json:"serverTime"`
	ColdStart  bool             `json:"coldStart"`
}
type evidence struct {
	rating     float64
	added      int64
	completed  bool
	affinities map[string]Affinity
	brief      int
}

// Version 1 keeps explicit preferences dominant. All components are bounded.
func score(c guide.Channel, p guide.Program, e evidence, now int64) Recommendation {
	out := Recommendation{Channel: c, Program: p, Reasons: []string{}, Components: map[string]float64{}}
	out.Channel.Programs = nil
	add := func(key string, value float64, reason string) {
		out.Components[key] = value
		out.Score += value
		if reason != "" && value > 0 {
			out.Reasons = append(out.Reasons, reason)
		}
	}
	if c.Favorite {
		add("favorite", 45, "Favorite channel")
	}
	for _, kind := range []string{"channel", "show", "genre", "actor", "director"} {
		a, ok := e.affinities[kind]
		if !ok {
			continue
		}
		weight := map[string]float64{"channel": 12, "show": 12, "genre": 10, "actor": 6, "director": 6}[kind]
		reason := "Because you watch " + a.Name
		if kind == "channel" {
			reason = "A channel you return to"
		}
		add(kind, weight*a.Score, reason)
	}
	if e.added > 0 && now-e.added*1000 >= 0 && now-e.added*1000 < 7*86400000 {
		add("freshness", 5, "New this week")
	}
	add("rating", max(0, min(5, e.rating/2)), "")
	if p.Start > now {
		add("timing", 7, "Starting soon")
	} else {
		add("timing", 8-28*max(0, min(1, float64(now-p.Start)/float64(p.Duration))), "")
	}
	if e.completed {
		add("recent_completion", -12, "")
	}
	// Three independent brief visits are required; one surf never penalizes a channel.
	if e.brief >= 3 {
		add("brief_visits", -min(3, float64(e.brief)/3), "")
	}
	if len(out.Reasons) == 0 {
		if p.Start > now {
			out.Reasons = append(out.Reasons, "Starting soon")
		} else if p.Progress < 20 {
			out.Reasons = append(out.Reasons, "Just getting started")
		} else {
			out.Reasons = append(out.Reasons, "On now in your channels")
		}
	}
	return out
}
func (s *Service) Recommend(ctx context.Context, user string, now time.Time, debug bool) (Recommendations, error) {
	out := Recommendations{Items: []Recommendation{}, Version: RankingVersion, ServerTime: now.UnixMilli(), ColdStart: true}
	affinities, err := s.Affinities(ctx, user)
	if err != nil {
		return out, err
	}
	out.ColdStart = len(affinities) == 0
	g := guide.New(s.db)
	g.Now = func() time.Time { return now }
	for offset := 0; ; offset += 100 {
		w, err := g.Window(ctx, user, guide.Query{Kind: "now", Offset: offset, Limit: 100})
		if err != nil {
			return out, err
		}
		media := map[string]bool{}
		for _, c := range w.Channels {
			for _, p := range c.Programs {
				if p.Start <= now.Add(30*time.Minute).UnixMilli() && p.Available {
					media[p.MediaID] = true
				}
			}
		}
		data := map[string]*evidence{}
		marks := []string{}
		args := []any{user, now.AddDate(0, 0, -14).UnixMilli(), user, now.AddDate(0, 0, -7).UnixMilli()}
		for id := range media {
			marks = append(marks, "?")
			args = append(args, id)
			data[id] = &evidence{affinities: map[string]Affinity{}}
		}
		if len(marks) > 0 {
			rows, e := s.db.QueryContext(ctx, `SELECT m.id,COALESCE(m.rating,0),COALESCE(m.added_at,0),EXISTS(SELECT 1 FROM user_program_history h WHERE h.user_id=? AND h.media_item_id=m.id AND h.completion_ratio>=0.9 AND h.started_at>=?),(SELECT COUNT(*) FROM user_program_history h WHERE h.user_id=? AND h.media_item_id=m.id AND h.ended_at IS NOT NULL AND h.watch_duration_ms<15000 AND h.started_at>=?),COALESCE(sh.id,'') FROM plex_media_items m LEFT JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key WHERE m.id IN (`+strings.Join(marks, ",")+`)`, args...)
			if e != nil {
				return out, e
			}
			for rows.Next() {
				var id, show string
				var v evidence
				if e = rows.Scan(&id, &v.rating, &v.added, &v.completed, &v.brief, &show); e != nil {
					rows.Close()
					return out, e
				}
				v.affinities = map[string]Affinity{}
				for _, a := range affinities {
					if a.Kind == "show" && a.Identity == show {
						v.affinities["show"] = a
					}
				}
				data[id] = &v
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return out, e
			}
			args = []any{user}
			for id := range media {
				args = append(args, id)
			}
			rows, e = s.db.QueryContext(ctx, `SELECT DISTINCT m.id,a.kind,a.identity,a.name,a.score,a.engaged_sessions FROM plex_media_items m LEFT JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key JOIN media_item_entities r ON r.media_item_id=m.id OR r.media_item_id=sh.id JOIN user_content_affinities a ON a.identity=CAST(r.entity_id AS TEXT) AND a.kind=r.relationship AND a.user_id=? WHERE m.id IN (`+strings.Join(marks, ",")+`)`, args...)
			if e != nil {
				return out, e
			}
			for rows.Next() {
				var id string
				var a Affinity
				if e = rows.Scan(&id, &a.Kind, &a.Identity, &a.Name, &a.Score, &a.Sessions); e != nil {
					rows.Close()
					return out, e
				}
				old := data[id].affinities[a.Kind]
				if a.Score > old.Score || (a.Score == old.Score && a.Identity < old.Identity) {
					data[id].affinities[a.Kind] = a
				}
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return out, e
			}
		}
		for _, c := range w.Channels {
			for _, p := range c.Programs {
				v := data[p.MediaID]
				if v == nil || p.Start > now.Add(30*time.Minute).UnixMilli() {
					continue
				}
				copyEvidence := *v
				copyEvidence.affinities = map[string]Affinity{}
				for k, a := range v.affinities {
					copyEvidence.affinities[k] = a
				}
				for _, a := range affinities {
					if a.Kind == "channel" && a.Identity == c.ID {
						copyEvidence.affinities["channel"] = a
					}
				}
				out.Items = append(out.Items, score(c, p, copyEvidence, now.UnixMilli()))
			}
		}
		if !w.More {
			break
		}
	}
	sort.Slice(out.Items, func(i, j int) bool {
		a, b := out.Items[i], out.Items[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.Program.Start != b.Program.Start {
			return a.Program.Start < b.Program.Start
		}
		return a.Program.ID < b.Program.ID
	})
	if len(out.Items) > 24 {
		out.Items = out.Items[:24]
	}
	if !debug {
		for i := range out.Items {
			out.Items[i].Score = 0
			out.Items[i].Components = nil
		}
	}
	return out, nil
}
