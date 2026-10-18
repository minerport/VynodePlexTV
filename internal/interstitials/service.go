package interstitials

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	DB  *sql.DB
	Dir string
}

func New(db *sql.DB, dir string) *Service { return &Service{db, filepath.Join(dir, "presentation")} }
func ID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}

type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }
func bad(s string) error       { return &Error{s} }
func validKind(s string) bool {
	switch s {
	case "BUMPER", "IDENT", "TRAILER", "COMING_UP_NEXT", "CUSTOM_CLIP", "FILLER":
		return true
	}
	return false
}
func (s *Service) Assets(ctx context.Context) ([]Asset, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,name,kind,source,COALESCE(media_id,''),duration_ms,enabled,available,active_from,active_until,container,video_codec,audio_codec,width,height,size_bytes FROM interstitial_assets ORDER BY id`)
	if e != nil {
		return nil, e
	}
	out := []Asset{}
	for rows.Next() {
		var a Asset
		if e = rows.Scan(&a.ID, &a.Name, &a.Kind, &a.Source, &a.MediaID, &a.Duration, &a.Enabled, &a.Available, &a.ActiveFrom, &a.ActiveUntil, &a.Container, &a.VideoCodec, &a.AudioCodec, &a.Width, &a.Height, &a.Size); e != nil {
			rows.Close()
			return nil, e
		}
		a.Tags = []string{}
		out = append(out, a)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return nil, e
	}
	rows, e = s.DB.QueryContext(ctx, "SELECT asset_id,tag FROM interstitial_tags ORDER BY tag")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	index := map[string]int{}
	for i, a := range out {
		index[a.ID] = i
	}
	for rows.Next() {
		var id, tag string
		if e = rows.Scan(&id, &tag); e != nil {
			return nil, e
		}
		if i, ok := index[id]; ok {
			out[i].Tags = append(out[i].Tags, tag)
		}
	}
	return out, rows.Err()
}
func (s *Service) Asset(ctx context.Context, id string) (Asset, error) {
	all, e := s.Assets(ctx)
	if e != nil {
		return Asset{}, e
	}
	for _, a := range all {
		if a.ID == id {
			return a, nil
		}
	}
	return Asset{}, bad("Presentation asset not found")
}
func (s *Service) SaveAsset(ctx context.Context, a Asset, actor string) (Asset, error) {
	if strings.TrimSpace(a.Name) == "" || len(a.Name) > 160 || !validKind(a.Kind) {
		return a, bad("Choose a name and presentation type")
	}
	for _, d := range []string{a.ActiveFrom, a.ActiveUntil} {
		if d != "" {
			if _, e := time.Parse("2006-01-02", d); e != nil {
				return a, bad("Use YYYY-MM-DD seasonal dates")
			}
		}
	}
	if a.ActiveFrom != "" && a.ActiveUntil != "" && a.ActiveUntil < a.ActiveFrom {
		return a, bad("Season ends before it starts")
	}
	if len(a.Tags) > 20 {
		return a, bad("Use at most 20 tags")
	}
	for i, t := range a.Tags {
		a.Tags[i] = strings.ToLower(strings.TrimSpace(t))
		if a.Tags[i] == "" || len(t) > 40 {
			return a, bad("Tags must contain 1–40 characters")
		}
	}
	fresh := a.ID == ""
	if !fresh {
		old, e := s.Asset(ctx, a.ID)
		if e != nil {
			return a, e
		}
		a.Source = old.Source
		a.MediaID = old.MediaID
		a.Duration = old.Duration
		a.Container = old.Container
		a.VideoCodec = old.VideoCodec
		a.AudioCodec = old.AudioCodec
		a.Width = old.Width
		a.Height = old.Height
		a.Size = old.Size
		a.Available = old.Available
	} else {
		a.ID = ID()
		a.Available = true
		if a.Source == "PLEX_MEDIA" {
			var d sql.NullInt64
			var available bool
			e := s.DB.QueryRowContext(ctx, "SELECT duration_ms,available FROM plex_media_items WHERE id=? AND media_type IN('movie','episode','clip')", a.MediaID).Scan(&d, &available)
			if e != nil || !available || !d.Valid {
				return a, bad("Choose available playable local Plex metadata")
			}
			a.Duration = d.Int64
		} else if a.Source != "GENERATED_METADATA" {
			return a, bad("Use the upload endpoint for local clips")
		}
	}
	if a.Duration < 1 || a.Duration > 600000 {
		return a, bad("Presentation duration must be between 1 ms and 10 minutes")
	}
	if a.Source == "GENERATED_METADATA" && a.Kind != "COMING_UP_NEXT" {
		return a, bad("Metadata segments must be Coming Up Next")
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return a, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `INSERT INTO interstitial_assets(id,name,kind,source,media_id,duration_ms,enabled,available,active_from,active_until,created_by,created_at,updated_at) VALUES(?,?,?,?,NULLIF(?,''),?,?,?,?,?,?,unixepoch(),unixepoch()) ON CONFLICT(id) DO UPDATE SET name=excluded.name,kind=excluded.kind,enabled=excluded.enabled,active_from=excluded.active_from,active_until=excluded.active_until,updated_at=excluded.updated_at`, a.ID, a.Name, a.Kind, a.Source, a.MediaID, a.Duration, a.Enabled, a.Available, a.ActiveFrom, a.ActiveUntil, actor)
	if e != nil {
		return a, e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM interstitial_tags WHERE asset_id=?", a.ID); e != nil {
		return a, e
	}
	for _, tag := range a.Tags {
		if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO interstitial_tags VALUES(?,?)", a.ID, tag); e != nil {
			return a, e
		}
	}
	return a, tx.Commit()
}
func (s *Service) DeleteAsset(ctx context.Context, id string) error {
	p, e := s.Path(id)
	if e != nil {
		return e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM presentation_entries WHERE asset_id=?", id).Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return bad("Asset is referenced by retained broadcasts. Disable it and repair future schedules; delete after history expires")
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM presentation_history WHERE asset_id=?", id); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM interstitial_assets WHERE id=?", id); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	if e = os.Remove(p); e != nil && !errors.Is(e, os.ErrNotExist) {
		return bad("Asset record removed; file cleanup failed")
	}
	return nil
}

func (s *Service) Path(id string) (string, error) {
	b, e := hex.DecodeString(id)
	if e != nil || len(b) != 16 {
		return "", bad("Invalid asset identity")
	}
	return filepath.Join(s.Dir, id+".mp4"), nil
}
func (s *Service) Rules(ctx context.Context) ([]Rule, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT id,name,COALESCE(channel_id,''),enabled,kind,tag,placement,media_type,every_n,repeat_minutes,max_duration_ms,priority FROM interstitial_rules ORDER BY priority,id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		if e = rows.Scan(&r.ID, &r.Name, &r.ChannelID, &r.Enabled, &r.Kind, &r.Tag, &r.Placement, &r.MediaType, &r.Every, &r.RepeatMinutes, &r.MaxDuration, &r.Priority); e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Service) SaveRule(ctx context.Context, r Rule) (Rule, error) {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 160 || !validKind(r.Kind) || r.Every < 1 || r.Every > 1000 || r.RepeatMinutes < 0 || r.RepeatMinutes > 10080 || r.MaxDuration < 1 || r.MaxDuration > 600000 {
		return r, bad("Check rule name, type, frequency, duration and repeat window")
	}
	if r.Placement != "BEFORE" && r.Placement != "BETWEEN" && r.Placement != "HOURLY" {
		return r, bad("Choose before, between or hourly placement")
	}
	if r.MediaType != "" && r.MediaType != "movie" && r.MediaType != "episode" {
		return r, bad("Choose movie, episode or any program")
	}
	if r.ID == "" {
		r.ID = ID()
	}
	_, e := s.DB.ExecContext(ctx, `INSERT INTO interstitial_rules VALUES(?,?,NULLIF(?,''),?,?,?,?,?,?,?,?,?,unixepoch(),unixepoch()) ON CONFLICT(id) DO UPDATE SET name=excluded.name,channel_id=excluded.channel_id,enabled=excluded.enabled,kind=excluded.kind,tag=excluded.tag,placement=excluded.placement,media_type=excluded.media_type,every_n=excluded.every_n,repeat_minutes=excluded.repeat_minutes,max_duration_ms=excluded.max_duration_ms,priority=excluded.priority,updated_at=excluded.updated_at`, r.ID, r.Name, r.ChannelID, r.Enabled, r.Kind, strings.ToLower(strings.TrimSpace(r.Tag)), r.Placement, r.MediaType, r.Every, r.RepeatMinutes, r.MaxDuration, r.Priority)
	return r, e
}
func (s *Service) Branding(ctx context.Context, id string) (Branding, error) {
	v := DefaultBranding(id)
	e := s.DB.QueryRowContext(ctx, `SELECT enabled,inherit_defaults,bug_enabled,bug_corner,bug_opacity,bug_size,COALESCE(compact_logo_id,''),next_up_seconds FROM channel_presentation WHERE channel_id=?`, id).Scan(&v.Enabled, &v.Inherit, &v.Bug, &v.Corner, &v.Opacity, &v.Size, &v.CompactLogoID, &v.NextUp)
	if e == sql.ErrNoRows {
		e = nil
	}
	return v, e
}
func (s *Service) SaveBranding(ctx context.Context, v Branding) error {
	if v.Opacity < .1 || v.Opacity > .7 || v.Size < 32 || v.Size > 128 || v.NextUp < 0 || v.NextUp > 120 {
		return bad("Check logo opacity/size and next-up timing")
	}
	switch v.Corner {
	case "top-left", "top-right", "bottom-left", "bottom-right":
	default:
		return bad("Choose a logo corner")
	}
	_, e := s.DB.ExecContext(ctx, `INSERT INTO channel_presentation VALUES(?,?,?,?,?,?,?,NULLIF(?,''),?) ON CONFLICT(channel_id) DO UPDATE SET enabled=excluded.enabled,inherit_defaults=excluded.inherit_defaults,bug_enabled=excluded.bug_enabled,bug_corner=excluded.bug_corner,bug_opacity=excluded.bug_opacity,bug_size=excluded.bug_size,compact_logo_id=excluded.compact_logo_id,next_up_seconds=excluded.next_up_seconds`, v.ChannelID, v.Enabled, v.Inherit, v.Bug, v.Corner, v.Opacity, v.Size, v.CompactLogoID, v.NextUp)
	return e
}
func (s *Service) Snapshot(ctx context.Context, id string) (Snapshot, error) {
	var out Snapshot
	var e error
	out.Branding, e = s.Branding(ctx, id)
	if e != nil || !out.Branding.Enabled {
		return out, e
	}
	all, e := s.Rules(ctx)
	if e != nil {
		return out, e
	}
	overrides := map[string]bool{}
	rows, e := s.DB.QueryContext(ctx, "SELECT rule_id,enabled FROM channel_interstitial_rules WHERE channel_id=?", id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var rule string
		var on bool
		if e = rows.Scan(&rule, &on); e != nil {
			rows.Close()
			return out, e
		}
		overrides[rule] = on
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, r := range all {
		if r.ChannelID == id || r.ChannelID == "" && out.Branding.Inherit {
			if on, ok := overrides[r.ID]; ok {
				r.Enabled = on
			}
			if r.Enabled {
				out.Rules = append(out.Rules, r)
			}
		}
	}
	out.Assets, e = s.Assets(ctx)
	if e != nil {
		return out, e
	}
	for i, a := range out.Assets {
		if a.Source == "UPLOAD" {
			p, _ := s.Path(a.ID)
			_, err := os.Stat(p)
			out.Assets[i].Available = err == nil
			if a.Available != out.Assets[i].Available {
				if _, e = s.DB.ExecContext(ctx, "UPDATE interstitial_assets SET available=? WHERE id=?", out.Assets[i].Available, a.ID); e != nil {
					return out, e
				}
			}
		} else if a.MediaID != "" {
			var available bool
			e = s.DB.QueryRowContext(ctx, "SELECT available FROM plex_media_items WHERE id=?", a.MediaID).Scan(&available)
			if e != nil {
				return out, e
			}
			out.Assets[i].Available = available && (a.Source != "PLEX_EXTRA" || a.Available)
		}
	}
	return out, nil
}
