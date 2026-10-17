package channels

import (
	"context"
	"strings"
)

func (s *Service) sourcePredicate(ctx context.Context, source Source, include bool) (Predicate, string, error) {
	if source.Recent != nil {
		r := RecentRule(*source.Recent)
		source.Rule = &r
		include = false
	}
	if e := s.ValidateSource(ctx, source); e != nil {
		return Predicate{}, "", e
	}
	p := Predicate{SQL: playableSQL}
	order := "m.title COLLATE NOCASE,COALESCE(m.year,0),m.id"
	if source.Recent != nil {
		order = "m.added_at DESC,m.id"
	}
	if source.Show != nil && source.Show.ExcludeUnavailable != nil && !*source.Show.ExcludeUnavailable {
		include = true
	}
	if !include {
		p.SQL += " AND " + eligibleSQL
	}
	var filter Predicate
	switch source.Type {
	case "STATIC":
		ids := source.Static.MediaIDs
		if len(ids) == 0 {
			p.SQL += " AND 0"
			return p, order, nil
		}
		filter.SQL = "m.id IN (" + placeholders(len(ids)) + ")"
		for _, id := range ids {
			filter.Args = append(filter.Args, id)
		}
		order = "" // order is appended with bound parameters below
	case "SHOW":
		filter = relationship("show", []string{source.Show.ShowID})
		if !source.Show.IncludeSpecials {
			filter.SQL += " AND COALESCE(m.parent_index_number,-1)<>0"
		}
		if source.Show.EpisodeScope == "SELECTED_SEASONS" {
			seasons := relationship("season", source.Show.SeasonIDs)
			filter.SQL += " AND " + seasons.SQL
			filter.Args = append(filter.Args, seasons.Args...)
		}
		order = "COALESCE(m.parent_index_number,2147483647),COALESCE(m.index_number,2147483647),m.id"
	case "COLLECTION":
		filter = relationship("collection", []string{source.Collection.CollectionID})
	case "RULE":
		var e error
		filter, e = (RuleCompiler{DB: s.db}).Compile(ctx, *source.Rule)
		if e != nil {
			return p, order, e
		}
	}
	p.SQL += " AND (" + filter.SQL + ")"
	p.Args = append(p.Args, filter.Args...)
	return p, order, nil
}

const fromMedia = ` FROM plex_media_items m JOIN plex_libraries l ON l.id=m.plex_library_id JOIN plex_servers s ON s.id=m.plex_server_id `
const mediaColumns = `m.id,m.title,m.media_type,m.year,COALESCE(show.title,''),m.parent_index_number,m.index_number,m.duration_ms,l.title,m.available,(` + eligibleSQL + `)`
const showJoin = ` LEFT JOIN plex_media_items show ON m.media_type='episode' AND show.plex_server_id=m.plex_server_id AND show.plex_rating_key=m.grandparent_rating_key `

func (s *Service) CountSource(ctx context.Context, source Source, include bool) (int, error) {
	p, _, e := s.sourcePredicate(ctx, source, include)
	if e != nil {
		return 0, e
	}
	var count int
	e = s.db.QueryRowContext(ctx, "SELECT count(*)"+fromMedia+" WHERE "+p.SQL, p.Args...).Scan(&count)
	return count, e
}
func (s *Service) CountResolvedMedia(ctx context.Context, id string) (int, error) {
	c, e := s.raw(ctx, id)
	if e != nil {
		return 0, e
	}
	return s.CountSource(ctx, c.Source, false)
}
func (s *Service) ResolveChannelMedia(ctx context.Context, id string, include bool, offset, limit int) (Preview, error) {
	c, e := s.raw(ctx, id)
	if e != nil {
		return Preview{}, e
	}
	return s.Preview(ctx, PreviewRequest{Source: c.Source, IncludeUnavailable: include, Offset: offset, Limit: limit})
}
func (s *Service) Preview(ctx context.Context, request PreviewRequest) (Preview, error) {
	p, order, e := s.sourcePredicate(ctx, request.Source, request.IncludeUnavailable)
	if e != nil {
		return Preview{}, e
	}
	if request.Limit < 1 || request.Limit > 100 {
		request.Limit = 24
	}
	if request.Offset < 0 {
		request.Offset = 0
	}
	out := Preview{Items: []Media{}, Warnings: []string{}, Offset: request.Offset, Limit: request.Limit}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	if e = tx.QueryRowContext(ctx, "SELECT count(*)"+fromMedia+" WHERE "+p.SQL, p.Args...).Scan(&out.Count); e != nil {
		return out, e
	}
	if request.Source.Type == "STATIC" && len(request.Source.Static.MediaIDs) > 0 {
		parts := []string{}
		for i, id := range request.Source.Static.MediaIDs {
			parts = append(parts, "WHEN ? THEN "+itoa(i))
			p.Args = append(p.Args, id)
		}
		order = "CASE m.id " + strings.Join(parts, " ") + " END"
	}
	if order == "" {
		order = "m.id"
	}
	args := append(p.Args, request.Limit, request.Offset)
	rows, e := tx.QueryContext(ctx, "SELECT "+mediaColumns+fromMedia+showJoin+" WHERE "+p.SQL+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var m Media
		if e = rows.Scan(&m.ID, &m.Title, &m.Type, &m.Year, &m.ShowTitle, &m.Season, &m.Episode, &m.DurationMS, &m.Library, &m.Available, &m.Eligible); e != nil {
			rows.Close()
			return out, e
		}
		out.Items = append(out.Items, m)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	if e = tx.Commit(); e != nil {
		return out, e
	}
	if out.Count == 0 {
		out.Warnings = append(out.Warnings, "No eligible media currently matches. This valid channel can still be saved.")
	}
	if request.Source.Static != nil && out.Count < len(request.Source.Static.MediaIDs) {
		out.Warnings = append(out.Warnings, "Unavailable or inactive-library selections remain configured; include unavailable to inspect them.")
	}
	return out, nil
}
