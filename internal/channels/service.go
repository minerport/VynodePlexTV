package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Service struct {
	db       *sql.DB
	dir      string
	uploadMu sync.Mutex
}

func New(db *sql.DB, dir string) *Service { return &Service{db: db, dir: dir} }
func optional(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func audit(ctx context.Context, tx *sql.Tx, actor, action, target, summary string) error {
	_, e := tx.ExecContext(ctx, "INSERT INTO audit_log(user_id,action,created_at,target_id,summary) VALUES(?,?,?,?,?)", optional(actor), action, stamp(), target, summary)
	return e
}
func (s *Service) ValidateSource(ctx context.Context, v Source) error {
	n := 0
	for _, on := range []bool{v.Static != nil, v.Show != nil, v.Collection != nil, v.Rule != nil} {
		if on {
			n++
		}
	}
	if n != 1 {
		return invalid("source", "Choose exactly one source configuration")
	}
	c := RuleCompiler{DB: s.db}
	switch v.Type {
	case "STATIC":
		if v.Static == nil {
			return invalid("source.static", "Static configuration required")
		}
		if len(v.Static.MediaIDs) > 1000 {
			return invalid("source.static.mediaIds", "Static channels support at most 1000 selected items")
		}
		seen := map[string]bool{}
		args := make([]any, 0, len(v.Static.MediaIDs))
		for _, id := range v.Static.MediaIDs {
			if seen[id] {
				return invalid("source.static.mediaIds", "Duplicate media items are not allowed")
			}
			seen[id] = true
			args = append(args, id)
		}
		if len(args) > 0 {
			var count int
			if e := s.db.QueryRowContext(ctx, "SELECT count(*) FROM plex_media_items m WHERE "+playableSQL+" AND m.id IN ("+placeholders(len(args))+")", args...).Scan(&count); e != nil {
				return e
			}
			if count != len(args) {
				return invalid("source.static.mediaIds", "Select existing movies or episodes; shows and seasons are containers")
			}
		}
	case "SHOW":
		if v.Show == nil {
			return invalid("source.show", "Show configuration required")
		}
		if e := c.reference(ctx, "show", v.Show.ShowID); e != nil {
			return e
		}
		if v.Show.EpisodeScope != "ALL" && v.Show.EpisodeScope != "SELECTED_SEASONS" {
			return invalid("source.show.episodeScope", "Choose all or selected seasons")
		}
		if len(v.Show.SeasonIDs) > 200 {
			return invalid("source.show.seasonIds", "Too many seasons")
		}
		if v.Show.EpisodeScope == "SELECTED_SEASONS" && len(v.Show.SeasonIDs) == 0 {
			return invalid("source.show.seasonIds", "Select at least one season")
		}
		seen := map[string]bool{}
		for _, id := range v.Show.SeasonIDs {
			var found bool
			e := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM plex_media_items season JOIN plex_media_items show ON show.plex_server_id=season.plex_server_id AND show.plex_rating_key=season.parent_rating_key WHERE season.id=? AND season.media_type='season' AND show.id=?)`, id, v.Show.ShowID).Scan(&found)
			if e != nil {
				return e
			}
			if !found || seen[id] {
				return invalid("source.show.seasonIds", "Select distinct seasons belonging to this show")
			}
			seen[id] = true
		}
	case "COLLECTION":
		if v.Collection == nil {
			return invalid("source.collection", "Collection required")
		}
		return c.reference(ctx, "collection", v.Collection.CollectionID)
	case "RULE":
		if v.Rule == nil {
			return invalid("source.rule", "Rule required")
		}
		_, e := c.Compile(ctx, *v.Rule)
		return e
	default:
		return invalid("source.type", "Unknown channel source type")
	}
	return nil
}

var numberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,23}$`)

func (s *Service) validate(ctx context.Context, d *Definition) error {
	d.Number = strings.TrimSpace(d.Number)
	d.Name = strings.TrimSpace(d.Name)
	d.ShortName = strings.TrimSpace(d.ShortName)
	if !numberPattern.MatchString(d.Number) {
		return invalid("number", "Use 1–24 letters, numbers, dots, hyphens or underscores")
	}
	if d.Name == "" || len(d.Name) > 100 {
		return invalid("name", "Enter a channel name of at most 100 characters")
	}
	if len(d.ShortName) > 24 || len(d.Description) > 2000 {
		return invalid("description", "Short name is limited to 24 and description to 2000 characters")
	}
	if d.Visibility != "VISIBLE" && d.Visibility != "HIDDEN" {
		return invalid("visibility", "Choose VISIBLE or HIDDEN")
	}
	refs := []struct {
		value       *string
		query, path string
	}{{d.GroupID, "SELECT EXISTS(SELECT 1 FROM channel_groups WHERE id=?)", "groupId"}, {d.ArtworkMediaID, "SELECT EXISTS(SELECT 1 FROM plex_media_items WHERE id=?)", "artworkMediaId"}}
	for _, r := range refs {
		if r.value != nil {
			var found bool
			if e := s.db.QueryRowContext(ctx, r.query, *r.value).Scan(&found); e != nil {
				return e
			}
			if !found {
				return invalid(r.path, "Selected reference does not exist")
			}
		}
	}
	for _, r := range []struct {
		a    *Appearance
		path string
	}{{&d.Logo, "logo"}, {&d.Background, "background"}} {
		if r.a.Source != "AUTO" && r.a.Source != "PLEX" && r.a.Source != "UPLOAD" {
			return invalid(r.path, "Choose automatic, Plex or uploaded artwork")
		}
		if r.a.Source == "PLEX" && d.ArtworkMediaID == nil {
			return invalid(r.path, "Choose media for Plex artwork")
		}
		if r.a.Source == "UPLOAD" {
			if r.a.UploadID == nil {
				return invalid(r.path, "Upload an image first")
			}
			var found bool
			if e := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM channel_uploads WHERE id=?)", *r.a.UploadID).Scan(&found); e != nil {
				return e
			}
			if !found {
				return invalid(r.path, "Uploaded image does not exist")
			}
		} else {
			r.a.UploadID = nil
		}
	}
	return s.ValidateSource(ctx, d.Source)
}

const columns = `c.id,c.channel_number,c.name,c.short_name,c.description,c.group_id,c.enabled,c.visibility,c.sort_order,c.logo_source,c.logo_upload_id,c.background_source,c.background_upload_id,c.default_artwork_media_id,c.created_at,c.updated_at,src.config_json`

func scanChannel(row interface{ Scan(...any) error }) (Channel, error) {
	var c Channel
	var config string
	e := row.Scan(&c.ID, &c.Number, &c.Name, &c.ShortName, &c.Description, &c.GroupID, &c.Enabled, &c.Visibility, &c.SortOrder, &c.Logo.Source, &c.Logo.UploadID, &c.Background.Source, &c.Background.UploadID, &c.ArtworkMediaID, &c.CreatedAt, &c.UpdatedAt, &config)
	if e == nil {
		e = json.Unmarshal([]byte(config), &c.Source)
	}
	c.Warnings = []string{}
	return c, e
}
func (s *Service) raw(ctx context.Context, id string) (Channel, error) {
	c, e := scanChannel(s.db.QueryRowContext(ctx, "SELECT "+columns+" FROM channels c JOIN channel_sources src ON src.channel_id=c.id WHERE c.id=?", id))
	if errors.Is(e, sql.ErrNoRows) {
		return c, fail("NOT_FOUND", "Channel not found")
	}
	if e != nil {
		return c, e
	}
	if c.Source.Type == "STATIC" {
		c.Source.Static = &StaticConfig{MediaIDs: []string{}}
		rows, e := s.db.QueryContext(ctx, "SELECT media_item_id FROM channel_static_items WHERE channel_id=? ORDER BY position", id)
		if e != nil {
			return c, e
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				return c, e
			}
			c.Source.Static.MediaIDs = append(c.Source.Static.MediaIDs, id)
		}
		e = rows.Err()
	}
	return c, e
}
func (s *Service) Get(ctx context.Context, id string) (Channel, error) {
	c, e := s.raw(ctx, id)
	if e != nil {
		return c, e
	}
	c.Count, e = s.CountSource(ctx, c.Source, false)
	if e != nil {
		if _, ok := e.(*Error); !ok {
			return c, e
		}
		c.Warnings = append(c.Warnings, "Source needs attention: "+e.Error())
		return c, nil
	}
	if c.Count == 0 {
		c.Warnings = append(c.Warnings, "No eligible media currently matches")
	}
	if c.Source.Static != nil && c.Count < len(c.Source.Static.MediaIDs) {
		c.Warnings = append(c.Warnings, "Unavailable or inactive-library items remain configured")
	}
	return c, nil
}
func (s *Service) List(ctx context.Context, offset, limit int) ([]Channel, error) {
	return s.ListGroup(ctx, offset, limit, "")
}
func (s *Service) ListGroup(ctx context.Context, offset, limit int, group string) ([]Channel, error) {
	if limit < 1 || limit > 101 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where := ""
	args := []any{}
	if group == "ungrouped" {
		where = " WHERE group_id IS NULL"
	} else if group != "" {
		where = " WHERE group_id=?"
		args = append(args, group)
	}
	args = append(args, limit, offset)
	rows, e := s.db.QueryContext(ctx, "SELECT id FROM channels"+where+" ORDER BY sort_order,channel_number COLLATE NOCASE,id LIMIT ? OFFSET ?", args...)
	if e != nil {
		return nil, e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return nil, e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	out := []Channel{}
	for _, id := range ids {
		c, e := s.Get(ctx, id)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, nil
}
func (s *Service) Save(ctx context.Context, id string, d Definition, actor string) (Channel, error) {
	return s.save(ctx, id, d, actor, "", "")
}
func (s *Service) save(ctx context.Context, id string, d Definition, actor, actionOverride, duplicateFrom string) (Channel, error) {
	if e := s.validate(ctx, &d); e != nil {
		return Channel{}, e
	}
	created := id == ""
	if created {
		id = newID()
	} else {
		if _, e := s.raw(ctx, id); e != nil {
			return Channel{}, e
		}
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return Channel{}, e
	}
	defer tx.Rollback()
	var conflict bool
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM channels WHERE channel_number=? COLLATE NOCASE AND id<>?)", d.Number, id).Scan(&conflict); e != nil {
		return Channel{}, e
	}
	if conflict {
		return Channel{}, &Error{"CONFLICT", "Channel number is already in use", []Issue{{"number", "Choose another channel number"}}}
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO channels(id,channel_number,name,short_name,description,channel_type,group_id,enabled,visibility,sort_order,logo_source,logo_upload_id,background_source,background_upload_id,default_artwork_media_id,created_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET channel_number=excluded.channel_number,name=excluded.name,short_name=excluded.short_name,description=excluded.description,channel_type=excluded.channel_type,group_id=excluded.group_id,enabled=excluded.enabled,visibility=excluded.visibility,sort_order=excluded.sort_order,logo_source=excluded.logo_source,logo_upload_id=excluded.logo_upload_id,background_source=excluded.background_source,background_upload_id=excluded.background_upload_id,default_artwork_media_id=excluded.default_artwork_media_id,updated_at=excluded.updated_at`, id, d.Number, d.Name, d.ShortName, d.Description, d.Source.Type, d.GroupID, d.Enabled, d.Visibility, d.SortOrder, d.Logo.Source, d.Logo.UploadID, d.Background.Source, d.Background.UploadID, d.ArtworkMediaID, optional(actor), stamp(), stamp())
	if e != nil {
		return Channel{}, e
	}
	config := d.Source
	if config.Static != nil {
		config.Static = &StaticConfig{MediaIDs: []string{}}
	}
	encoded, e := json.Marshal(config)
	if e != nil {
		return Channel{}, e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO channel_sources VALUES(?,?,?,?,?) ON CONFLICT(channel_id) DO UPDATE SET source_type=excluded.source_type,config_json=excluded.config_json,updated_at=excluded.updated_at", id, d.Source.Type, string(encoded), stamp(), stamp()); e != nil {
		return Channel{}, e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM channel_static_items WHERE channel_id=?", id); e != nil {
		return Channel{}, e
	}
	if d.Source.Static != nil {
		for position, item := range d.Source.Static.MediaIDs {
			if _, e = tx.ExecContext(ctx, "INSERT INTO channel_static_items VALUES(?,?,?,?)", id, item, position, stamp()); e != nil {
				return Channel{}, e
			}
		}
	}
	action := "CHANNEL_UPDATED"
	if created {
		action = "CHANNEL_CREATED"
	}
	if actionOverride != "" {
		action = actionOverride
	}
	if created {
		if duplicateFrom != "" {
			_, e = tx.ExecContext(ctx, `INSERT INTO channel_schedule_settings SELECT ?,mode,timezone,anchor_ms,lower(hex(randomblob(16))),movie_repeat_days,episode_repeat_days,past_days,future_days,include_unavailable,enabled,unixepoch(),unixepoch() FROM channel_schedule_settings WHERE channel_id=?`, id, duplicateFrom)
		} else {
			_, e = tx.ExecContext(ctx, `INSERT INTO channel_schedule_settings(channel_id,mode,timezone,anchor_ms,seed,created_at,updated_at) SELECT ?,CASE WHEN ? IN('STATIC','SHOW') THEN 'SEQUENTIAL' ELSE 'SMART_RANDOM' END,timezone,?,lower(hex(randomblob(16))),unixepoch(),unixepoch() FROM app_settings WHERE id=1`, id, d.Source.Type, (stamp()-604800)*1000)
		}
		if e != nil {
			return Channel{}, e
		}
	}
	if e = audit(ctx, tx, actor, action, id, "Channel configuration saved"); e != nil {
		return Channel{}, e
	}
	if e = tx.Commit(); e != nil {
		return Channel{}, e
	}
	return s.Get(ctx, id)
}
func (s *Service) Delete(ctx context.Context, id, actor string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, "DELETE FROM channels WHERE id=?", id)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return fail("NOT_FOUND", "Channel not found")
	}
	if e = audit(ctx, tx, actor, "CHANNEL_DELETED", id, "Channel deleted; Plex inventory retained"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) Enable(ctx context.Context, id string, enabled bool, actor string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	r, e := tx.ExecContext(ctx, "UPDATE channels SET enabled=?,updated_at=? WHERE id=?", enabled, stamp(), id)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return fail("NOT_FOUND", "Channel not found")
	}
	action := "CHANNEL_DISABLED"
	if enabled {
		action = "CHANNEL_ENABLED"
	}
	if e = audit(ctx, tx, actor, action, id, "Channel enabled state changed"); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Service) NextNumber(ctx context.Context) (string, error) {
	var next int
	e := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(CAST(channel_number AS INTEGER)),100)+1 FROM channels WHERE channel_number NOT GLOB '*[^0-9]*' AND length(channel_number)<=8`).Scan(&next)
	if next < 101 {
		next = 101
	}
	return strconv.Itoa(next), e
}
func (s *Service) Duplicate(ctx context.Context, id, actor string) (Channel, error) {
	c, e := s.raw(ctx, id)
	if e != nil {
		return c, e
	}
	c.Number, e = s.NextNumber(ctx)
	if e != nil {
		return c, e
	}
	c.Name = strings.TrimSpace(c.Name)
	if len(c.Name) > 90 {
		c.Name = string([]rune(c.Name)[:min(45, len([]rune(c.Name)))])
	}
	c.Name += " Copy"
	return s.save(ctx, "", c.Definition, actor, "CHANNEL_DUPLICATED", id)
}
