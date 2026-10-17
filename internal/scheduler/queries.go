package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const entrySelect = `SELECT p.id,p.channel_id,p.media_item_id,p.start_time_utc,p.end_time_utc,p.duration_ms,p.sequence_number,p.schedule_revision,p.policy_reason,m.title,COALESCE(sh.title,''),m.parent_index_number,m.index_number,m.available FROM channel_programs p JOIN plex_media_items m ON m.id=p.media_item_id LEFT JOIN plex_media_items sh ON sh.plex_server_id=m.plex_server_id AND sh.plex_rating_key=m.grandparent_rating_key `

type scanner interface{ Scan(...any) error }

func scanEntry(row scanner) (Entry, error) {
	var p Entry
	e := row.Scan(&p.ID, &p.ChannelID, &p.MediaID, &p.Start, &p.End, &p.Duration, &p.Sequence, &p.Revision, &p.Reason, &p.Title, &p.ShowTitle, &p.Season, &p.Episode, &p.Available)
	p.Artwork = "/api/v1/media/" + p.MediaID + "/artwork/poster"
	return p, e
}
func (s *Service) Programs(ctx context.Context, id string, start, end int64, offset, limit int) (Page, error) {
	v, e := s.Settings(ctx, id)
	out := Page{Items: []Entry{}, Start: start, End: end, Timezone: v.Timezone}
	if e != nil {
		return out, e
	}
	if end <= start || end-start > 32*dayMS || offset < 0 {
		return out, invalid("Choose an interval of up to 32 days and a non-negative offset")
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, e := s.db.QueryContext(ctx, entrySelect+`WHERE p.channel_id=? AND p.end_time_utc>? AND p.start_time_utc<? ORDER BY p.start_time_utc LIMIT ? OFFSET ?`, id, start, end, limit+1, offset)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		p, e := scanEntry(rows)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, p)
	}
	out.More = len(out.Items) > limit
	if out.More {
		out.Items = out.Items[:limit]
	}
	return out, rows.Err()
}

// GetProgramAt uses half-open intervals [start,end).
func (s *Service) GetProgramAt(ctx context.Context, id string, at time.Time) (*Entry, error) {
	return s.one(ctx, id, `p.start_time_utc<=? AND p.end_time_utc>? ORDER BY p.start_time_utc DESC`, at.UnixMilli(), at.UnixMilli())
}
func (s *Service) Next(ctx context.Context, id string, at time.Time) (*Entry, error) {
	return s.one(ctx, id, `p.start_time_utc>? ORDER BY p.start_time_utc`, at.UnixMilli())
}
func (s *Service) Previous(ctx context.Context, id string, at time.Time) (*Entry, error) {
	return s.one(ctx, id, `p.end_time_utc<=? ORDER BY p.end_time_utc DESC`, at.UnixMilli())
}
func (s *Service) one(ctx context.Context, id, where string, args ...any) (*Entry, error) {
	args = append([]any{id}, args...)
	v, e := scanEntry(s.db.QueryRowContext(ctx, entrySelect+`WHERE p.channel_id=? AND `+where+` LIMIT 1`, args...))
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	return &v, e
}
func (s *Service) Diagnostics(ctx context.Context, id string) ([]string, error) {
	out := []string{}
	rows, e := s.db.QueryContext(ctx, `SELECT p.start_time_utc,p.end_time_utc,p.duration_ms,p.sequence_number,m.id FROM channel_programs p LEFT JOIN plex_media_items m ON m.id=p.media_item_id WHERE channel_id=? ORDER BY start_time_utc`, id)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	var end int64
	seen := map[int64]bool{}
	counts := map[string]int{}
	first := true
	for rows.Next() {
		var a, b, d, n int64
		var media *string
		if e = rows.Scan(&a, &b, &d, &n, &media); e != nil {
			return out, e
		}
		if !first && a < end {
			counts["overlaps"]++
		}
		if !first && a > end {
			counts["gaps"]++
		}
		if d <= 0 || b-a != d {
			counts["invalid durations"]++
		}
		if seen[n] {
			counts["duplicate positions"]++
		}
		if media == nil {
			counts["missing media references"]++
		}
		seen[n] = true
		end = b
		first = false
	}
	for _, key := range []string{"overlaps", "gaps", "invalid durations", "duplicate positions", "missing media references"} {
		if counts[key] > 0 {
			out = append(out, fmt.Sprintf("%d %s", counts[key], key))
		}
	}
	return out, rows.Err()
}
func (s *Service) Status(ctx context.Context, id string) (Status, error) {
	v, e := s.Settings(ctx, id)
	out := Status{ChannelID: id, Mode: v.Mode, Timezone: v.Timezone, Integrity: []string{}, Warnings: []string{}}
	if e != nil {
		return out, e
	}
	c, items, count, fp, _, e := s.input(ctx, v)
	if e != nil {
		return out, e
	}
	st, e := s.state(ctx, id)
	if e != nil {
		return out, e
	}
	out.Name = c.Name
	out.Number = c.Number
	out.Resolved = count
	out.Schedulable = len(items)
	out.Excluded = count - len(items)
	out.Revision = st.Revision
	out.Last = st.Last
	out.Error = st.Error
	out.Status = st.Status
	if e = s.db.QueryRowContext(ctx, `SELECT MIN(start_time_utc),MAX(end_time_utc),COUNT(*) FROM channel_programs WHERE channel_id=?`, id).Scan(&out.Start, &out.End, &out.Count); e != nil {
		return out, e
	}
	out.Integrity, e = s.Diagnostics(ctx, id)
	if e != nil {
		return out, e
	}
	_, through := s.horizon(v)
	if out.Count == 0 || out.End == nil || *out.End < through {
		out.Status = "NEEDS_GENERATION"
	}
	if st.Fingerprint != "" && st.Fingerprint != fp || len(out.Integrity) > 0 {
		out.Status = "NEEDS_REPAIR"
	}
	if len(items) == 0 {
		out.Status = "EMPTY"
	}
	if out.Error != "" {
		out.Status = "ERROR"
	}
	var active int
	if e = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE channel_id=? AND status IN('QUEUED','RUNNING') AND type LIKE 'SCHEDULE_%'`, id).Scan(&active); e != nil {
		return out, e
	}
	if active > 0 {
		out.Status = "GENERATING"
	}
	if !c.Enabled || !v.Enabled {
		out.Status = "DISABLED"
	}
	if out.Excluded > 0 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("%d items excluded: missing or invalid duration (maximum 7 days)", out.Excluded))
	}
	return out, nil
}
func (s *Service) Readable(ctx context.Context, id string, admin bool) error {
	var enabled bool
	if e := s.db.QueryRowContext(ctx, "SELECT enabled FROM channels WHERE id=?", id).Scan(&enabled); e != nil {
		if e == sql.ErrNoRows {
			return &Error{"NOT_FOUND", "Channel not found"}
		}
		return e
	}
	if !enabled && !admin {
		return &Error{"NOT_FOUND", "Channel not found"}
	}
	return nil
}
func (s *Service) ChannelIDs(ctx context.Context, offset, limit int) ([]string, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 1 || limit > 100 {
		limit = 100
	}
	rows, e := s.db.QueryContext(ctx, "SELECT id FROM channels ORDER BY channel_number,id LIMIT ? OFFSET ?", limit, offset)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			return nil, e
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
