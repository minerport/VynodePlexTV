package channels

import (
	"context"
	"strings"
)

func (s *Service) Metadata(ctx context.Context, field, search, parent string, offset, limit int) (Options, error) {
	if limit < 1 || limit > 50 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	if len(search) > 200 {
		return Options{}, invalid("q", "Search is too long")
	}
	out := Options{Items: []Option{}}
	if field == "media_type" {
		out.Items = []Option{{ID: "movie", Name: "Movie"}, {ID: "episode", Name: "Episode"}}
		return out, nil
	}
	pattern := "%" + strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(search) + "%"
	args := []any{pattern}
	query := ""
	switch field {
	case "server":
		query = `SELECT id,name,'Plex server',0,'',0,0 FROM plex_servers WHERE name LIKE ? ESCAPE '\' ORDER BY name,id`
	case "library":
		query = `SELECT l.id,l.title,s.name,(SELECT count(*) FROM plex_media_items m WHERE m.plex_library_id=l.id),'',0,0 FROM plex_libraries l JOIN plex_servers s ON s.id=l.plex_server_id WHERE l.title LIKE ? ESCAPE '\' ORDER BY l.title,l.id`
	case "show", "season":
		query = `SELECT m.id,m.title,s.name||' / '||l.title,(SELECT count(*) FROM plex_media_items ep WHERE ep.plex_server_id=m.plex_server_id AND (ep.grandparent_rating_key=m.plex_rating_key OR ep.parent_rating_key=m.plex_rating_key) AND ep.media_type='episode'),m.id,(SELECT count(*) FROM plex_media_items se WHERE se.plex_server_id=m.plex_server_id AND se.parent_rating_key=m.plex_rating_key AND se.media_type='season'),(SELECT count(*) FROM plex_media_items ep WHERE ep.plex_server_id=m.plex_server_id AND (ep.grandparent_rating_key=m.plex_rating_key OR ep.parent_rating_key=m.plex_rating_key) AND ep.media_type='episode')` + fromMedia + ` WHERE m.media_type=? AND m.title LIKE ? ESCAPE '\'`
		args = []any{field, pattern}
		if parent != "" {
			query += ` AND EXISTS(SELECT 1 FROM plex_media_items p WHERE p.id=? AND p.plex_server_id=m.plex_server_id AND p.plex_rating_key=m.parent_rating_key)`
			args = append(args, parent)
		}
		query += ` ORDER BY COALESCE(m.index_number,0),m.title,m.id`
	case "studio", "content_rating":
		column := "m.studio"
		if field == "content_rating" {
			column = "m.content_rating"
		}
		query = "SELECT " + column + "," + column + ",'Metadata',count(*),min(m.id),0,0" + fromMedia + " WHERE " + column + "<>'' AND " + column + " LIKE ? ESCAPE '\\' GROUP BY " + column + " ORDER BY " + column
	case "genre", "actor", "director", "writer", "collection", "label":
		kind := field
		if field == "actor" || field == "director" || field == "writer" {
			kind = "person"
		}
		id := "CAST(e.id AS TEXT)"
		group := "e.id"
		context := "s.name||' / '||group_concat(DISTINCT l.title)"
		if field == "collection" {
			id += "||'@'||l.id"
			group += ",l.id"
			context = "s.name||' / '||l.title"
		}
		query = "SELECT " + id + ",e.name," + context + ",count(DISTINCT m.id),min(m.id),0,0 FROM metadata_entities e JOIN media_item_entities r ON r.entity_id=e.id JOIN plex_media_items m ON m.id=r.media_item_id JOIN plex_libraries l ON l.id=m.plex_library_id JOIN plex_servers s ON s.id=m.plex_server_id WHERE e.kind=? AND r.relationship=? AND e.name LIKE ? ESCAPE '\\' GROUP BY " + group + " ORDER BY e.name,e.id"
		args = []any{kind, field, pattern}
	default:
		return out, invalid("field", "Unsupported metadata field")
	}
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit+1, offset)
	rows, e := s.db.QueryContext(ctx, query, args...)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var o Option
		if e = rows.Scan(&o.ID, &o.Name, &o.Context, &o.Count, &o.ArtworkID, &o.Seasons, &o.Episodes); e != nil {
			return out, e
		}
		out.Items = append(out.Items, o)
	}
	if len(out.Items) > limit {
		out.More = true
		out.Items = out.Items[:limit]
	}
	return out, rows.Err()
}

func (s *Service) Lookup(ctx context.Context, field, id string) (Option, error) {
	if e := (RuleCompiler{DB: s.db}).reference(ctx, field, id); e != nil {
		return Option{}, e
	}
	o := Option{ID: id}
	var q string
	args := []any{id}
	switch field {
	case "server":
		q = "SELECT name,'Plex server' FROM plex_servers WHERE id=?"
	case "library":
		q = "SELECT l.title,s.name FROM plex_libraries l JOIN plex_servers s ON s.id=l.plex_server_id WHERE l.id=?"
	case "show", "season":
		q = "SELECT m.title,s.name||' / '||l.title" + fromMedia + " WHERE m.id=?"
		o.ArtworkID = id
	case "media_type", "content_rating":
		o.Name = id
		return o, nil
	default:
		if field == "collection" {
			entity, library, _ := collectionParts(id)
			args = []any{library, entity}
			q = "SELECT e.name,s.name||' / '||l.title FROM metadata_entities e JOIN plex_servers s ON s.id=e.plex_server_id JOIN plex_libraries l ON l.id=? WHERE e.id=?"
		} else {
			q = "SELECT e.name,s.name FROM metadata_entities e JOIN plex_servers s ON s.id=e.plex_server_id WHERE e.id=?"
		}
	}
	e := s.db.QueryRowContext(ctx, q, args...).Scan(&o.Name, &o.Context)
	return o, e
}
