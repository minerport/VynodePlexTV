package channels

import (
	"context"
	"strings"
)

// ScheduleItem is application metadata, independent of Plex transport structures.
type ScheduleItem struct {
	Media
	ShowID string
}

func (s *Service) ScheduleSnapshot(ctx context.Context, id string, include bool) (Channel, []ScheduleItem, error) {
	c, e := s.raw(ctx, id)
	if e != nil {
		return c, nil, e
	}
	p, order, e := s.sourcePredicate(ctx, c.Source, include)
	if e != nil {
		return c, nil, e
	}
	if !include {
		p.SQL += " AND " + eligibleSQL
	}
	if c.Source.Static != nil && len(c.Source.Static.MediaIDs) > 0 {
		parts := []string{}
		for i, id := range c.Source.Static.MediaIDs {
			parts = append(parts, "WHEN ? THEN "+itoa(i))
			p.Args = append(p.Args, id)
		}
		order = "CASE m.id " + strings.Join(parts, " ") + " END"
	}
	if order == "" {
		order = "m.id"
	}
	rows, e := s.db.QueryContext(ctx, "SELECT "+mediaColumns+",COALESCE(show.id,'')"+fromMedia+showJoin+" WHERE "+p.SQL+" ORDER BY "+order+" LIMIT 50001", p.Args...)
	if e != nil {
		return c, nil, e
	}
	defer rows.Close()
	out := []ScheduleItem{}
	for rows.Next() {
		var m ScheduleItem
		if e = rows.Scan(&m.ID, &m.Title, &m.Type, &m.Year, &m.ShowTitle, &m.Season, &m.Episode, &m.DurationMS, &m.Library, &m.Available, &m.Eligible, &m.ShowID); e != nil {
			return c, nil, e
		}
		out = append(out, m)
	}
	if len(out) > 50000 {
		return c, nil, invalid("source", "Scheduling supports up to 50,000 members per channel")
	}
	return c, out, rows.Err()
}
