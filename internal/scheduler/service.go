package scheduler

import (
	"afterglow.local/tv/internal/channels"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"
)

type Membership interface {
	ScheduleSnapshot(context.Context, string, bool) (channels.Channel, []channels.ScheduleItem, error)
}
type Service struct {
	db      *sql.DB
	members Membership
	log     *slog.Logger
	Now     func() time.Time
	mu      sync.Mutex
}

func New(db *sql.DB, m Membership, log *slog.Logger) *Service {
	return &Service{db: db, members: m, log: log, Now: time.Now}
}

const settingColumns = "channel_id,mode,timezone,anchor_ms,seed,movie_repeat_days,episode_repeat_days,past_days,future_days,include_unavailable,enabled"

func (s *Service) Settings(ctx context.Context, id string) (Settings, error) {
	var v Settings
	e := s.db.QueryRowContext(ctx, "SELECT "+settingColumns+" FROM channel_schedule_settings WHERE channel_id=?", id).Scan(&v.ChannelID, &v.Mode, &v.Timezone, &v.Anchor, &v.Seed, &v.MovieDays, &v.EpisodeDays, &v.PastDays, &v.FutureDays, &v.IncludeUnavailable, &v.Enabled)
	if errors.Is(e, sql.ErrNoRows) {
		return v, &Error{"NOT_FOUND", "Channel schedule settings not found"}
	}
	return v, e
}
func valid(v Settings) error {
	if v.Mode != "SEQUENTIAL" && v.Mode != "RANDOM" && v.Mode != "SMART_RANDOM" {
		return invalid("Choose sequential, random or smart random")
	}
	if _, e := time.LoadLocation(v.Timezone); e != nil {
		return invalid("Choose an IANA timezone")
	}
	if v.Anchor < 946684800000 || v.Anchor > 4102444800000 {
		return invalid("Anchor must be between 2000 and 2100")
	}
	if len(v.Seed) < 1 || len(v.Seed) > 128 {
		return invalid("Seed must contain 1–128 characters")
	}
	if v.MovieDays < 0 || v.MovieDays > 365 || v.EpisodeDays < 0 || v.EpisodeDays > 365 || v.PastDays < 0 || v.PastDays > 30 || v.FutureDays < 1 || v.FutureDays > 30 {
		return invalid("Horizon must be 0–30 past / 1–30 future days; repeat windows 0–365 days")
	}
	return nil
}
func (s *Service) SaveSettings(ctx context.Context, id string, v Settings, actor string) (Settings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := valid(v); e != nil {
		return v, e
	}
	old, e := s.Settings(ctx, id)
	if e != nil {
		return v, e
	}
	v.ChannelID = id
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return v, e
	}
	defer tx.Rollback()
	_, e = tx.ExecContext(ctx, `UPDATE channel_schedule_settings SET mode=?,timezone=?,anchor_ms=?,seed=?,movie_repeat_days=?,episode_repeat_days=?,past_days=?,future_days=?,include_unavailable=?,enabled=?,updated_at=? WHERE channel_id=?`, v.Mode, v.Timezone, v.Anchor, v.Seed, v.MovieDays, v.EpisodeDays, v.PastDays, v.FutureDays, v.IncludeUnavailable, v.Enabled, s.Now().Unix(), id)
	if e != nil {
		return v, e
	}
	if e = s.audit(ctx, tx, actor, "SCHEDULE_SETTINGS_UPDATED", id, "Settings saved; existing current/past programs protected"); e != nil {
		return v, e
	}
	if v.Seed != old.Seed {
		if e = s.audit(ctx, tx, actor, "SCHEDULE_SEED_CHANGED", id, "Seed changed; future timeline requires repair"); e != nil {
			return v, e
		}
	}
	return v, tx.Commit()
}
func (s *Service) audit(ctx context.Context, tx *sql.Tx, actor, action, id, summary string) error {
	var user any
	if actor != "" {
		user = actor
	}
	_, e := tx.ExecContext(ctx, "INSERT INTO audit_log(user_id,action,created_at,target_id,summary) VALUES(?,?,?,?,?)", user, action, s.Now().Unix(), id, summary)
	return e
}
func (s *Service) state(ctx context.Context, id string) (State, error) {
	v := State{Status: "NEEDS_GENERATION", Membership: "[]"}
	e := s.db.QueryRowContext(ctx, `SELECT fingerprint,config_fingerprint,membership_json,revision,next_sequence,covered_until,status,last_generated,last_error FROM scheduler_state WHERE channel_id=?`, id).Scan(&v.Fingerprint, &v.Config, &v.Membership, &v.Revision, &v.Next, &v.Until, &v.Status, &v.Last, &v.Error)
	if e == sql.ErrNoRows {
		e = nil
	}
	return v, e
}
func (s *Service) input(ctx context.Context, v Settings) (channels.Channel, []Item, int, string, string, error) {
	c, media, e := s.members.ScheduleSnapshot(ctx, v.ChannelID, v.IncludeUnavailable)
	if e != nil {
		return c, nil, 0, "", "", e
	}
	items := []Item{}
	for _, m := range media {
		if m.DurationMS == nil || *m.DurationMS <= 0 || *m.DurationMS > 7*dayMS {
			continue
		}
		season := -1
		if m.Season != nil {
			season = *m.Season
		}
		items = append(items, Item{m.ID, *m.DurationMS, m.Type, m.ShowID, season})
	}
	if v.Mode != "SEQUENTIAL" {
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	}
	// Horizon/timezone changes extend/prune without changing the selection sequence.
	config := hash([]any{v.ChannelID, v.Mode, v.Seed, v.Anchor, v.MovieDays, v.EpisodeDays, v.IncludeUnavailable, c.Source})
	return c, items, len(media), hash([]any{config, items}), config, nil
}
func (s *Service) horizon(v Settings) (int64, int64) {
	loc, _ := time.LoadLocation(v.Timezone)
	now := s.Now().In(loc)
	return now.AddDate(0, 0, -v.PastDays).UnixMilli(), now.AddDate(0, 0, v.FutureDays).UnixMilli()
}
func (s *Service) EnsureScheduleCoverage(ctx context.Context, id string, through time.Time) (Result, error) {
	return s.ensure(ctx, id, through.UnixMilli(), false, "", "")
}
func (s *Service) RegenerateFuture(ctx context.Context, id, actor string) (Result, error) {
	return s.ensure(ctx, id, 0, true, actor, "")
}
func (s *Service) Repair(ctx context.Context, id, actor string) (Result, error) {
	return s.ensure(ctx, id, 0, false, actor, "")
}
func (s *Service) ensure(ctx context.Context, id string, through int64, force bool, actor, job string) (Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Result{}
	v, e := s.Settings(ctx, id)
	if e != nil {
		return out, e
	}
	if e = valid(v); e != nil {
		return out, e
	}
	c, items, _, fp, conf, e := s.input(ctx, v)
	if e != nil {
		return out, e
	}
	if !c.Enabled || !v.Enabled {
		out.Status = "DISABLED"
		return out, nil
	}
	st, e := s.state(ctx, id)
	if e != nil {
		return out, e
	}
	past, end := s.horizon(v)
	if through == 0 {
		through = end
	}
	if through > end+dayMS {
		return out, invalid("Requested coverage exceeds configured future horizon")
	}
	now := s.Now().UnixMilli()
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return out, e
	}
	defer tx.Rollback()
	rows, e := tx.QueryContext(ctx, `SELECT media_item_id,start_time_utc,end_time_utc,duration_ms,sequence_number FROM channel_programs WHERE channel_id=? ORDER BY start_time_utc,sequence_number`, id)
	if e != nil {
		return out, e
	}
	timeline := []Entry{}
	for rows.Next() {
		var p Entry
		if e = rows.Scan(&p.MediaID, &p.Start, &p.End, &p.Duration, &p.Sequence); e != nil {
			rows.Close()
			return out, e
		}
		timeline = append(timeline, p)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	byID := map[string]Item{}
	for _, m := range items {
		byID[m.ID] = m
	}
	old := []Item{}
	if e = json.Unmarshal([]byte(st.Membership), &old); e != nil {
		return out, e
	}
	oldIDs := map[string]bool{}
	for _, m := range old {
		oldIDs[m.ID] = true
	}
	added := false
	for _, m := range items {
		if !oldIDs[m.ID] {
			added = true
		}
	}
	orderChanged := false
	if v.Mode == "SEQUENTIAL" && len(old) == len(items) {
		for i := range items {
			if old[i].ID != items[i].ID {
				orderChanged = true
				break
			}
		}
	}
	cut := len(timeline)
	for i, p := range timeline {
		if p.Start <= now {
			continue
		}
		bad := false
		if i > 0 && p.Start != timeline[i-1].End {
			bad = true
		}
		m, ok := byID[p.MediaID]
		if !ok || m.Duration != p.Duration {
			bad = true
		}
		if force || conf != st.Config || added || orderChanged {
			bad = true
		}
		if bad {
			cut = i
			break
		}
	}
	// Preserve every entry that has started, even if an old corrupted interval overlaps now.

	from := v.Anchor
	seq := int64(0)
	if len(timeline) > 0 {
		if cut == 0 {
			from = timeline[0].Start
			seq = timeline[0].Sequence
		} else {
			p := timeline[cut-1]
			from = p.End
			seq = p.Sequence + 1
		}
	} else if st.Until != nil {
		from = *st.Until
		seq = st.Next
	}
	if cut < len(timeline) {
		out.Repaired = len(timeline) - cut
		if _, e = tx.ExecContext(ctx, "DELETE FROM channel_programs WHERE channel_id=? AND sequence_number>=?", id, timeline[cut].Sequence); e != nil {
			return out, e
		}
	}
	history := map[string]int64{}
	rows, e = tx.QueryContext(ctx, "SELECT media_item_id,last_start_ms FROM channel_schedule_history WHERE channel_id=?", id)
	if e != nil {
		return out, e
	}
	for rows.Next() {
		var key string
		var at int64
		if e = rows.Scan(&key, &at); e != nil {
			rows.Close()
			return out, e
		}
		history[key] = at
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return out, e
	}
	for _, p := range timeline[:cut] {
		history[p.MediaID] = p.Start
	}
	var previous *Item
	if cut > 0 {
		p := timeline[cut-1]
		m, ok := byID[p.MediaID]
		if !ok {
			m = Item{ID: p.MediaID}
		}
		previous = &m
	}
	revision := st.Revision
	if fp != st.Fingerprint || force {
		revision++
	}
	if revision == 0 {
		revision = 1
	}
	generated, e := Generate(ctx, Request{v, items, from, through, seq, history, previous})
	if e != nil {
		return out, e
	}
	out.Start = from
	out.End = from
	out.Status = "READY"
	if len(items) == 0 {
		out.Status = "EMPTY"
	}
	stmt, e := tx.PrepareContext(ctx, `INSERT INTO channel_programs(id,channel_id,media_item_id,source_membership_identity,start_time_utc,end_time_utc,duration_ms,sequence_number,generation_id,schedule_revision,policy_reason,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)
	if e != nil {
		return out, e
	}
	defer stmt.Close()
	generation := uid()
	for _, p := range generated {
		if _, e = stmt.ExecContext(ctx, hash([]any{id, p.Sequence, p.Start, p.MediaID, p.Duration, revision}), id, p.MediaID, hash(byID[p.MediaID]), p.Start, p.End, p.Duration, p.Sequence, generation, revision, p.Reason, s.Now().Unix()); e != nil {
			return out, e
		}
		out.Generated++
		out.End = p.End
		seq = p.Sequence + 1
	}
	// Keep the last air-start of pruned programs, so long repeat windows survive retention and restart.
	_, e = tx.ExecContext(ctx, `INSERT INTO channel_schedule_history(channel_id,media_item_id,last_start_ms) SELECT channel_id,media_item_id,MAX(start_time_utc) FROM channel_programs WHERE channel_id=? AND end_time_utc<? GROUP BY channel_id,media_item_id ON CONFLICT(channel_id,media_item_id) DO UPDATE SET last_start_ms=MAX(last_start_ms,excluded.last_start_ms)`, id, past)
	if e != nil {
		return out, e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM channel_programs WHERE channel_id=? AND end_time_utc<?", id, past); e != nil {
		return out, e
	}
	encoded, _ := json.Marshal(items)
	last := st.Last
	if out.Generated > 0 || out.Repaired > 0 {
		n := s.Now().UnixMilli()
		last = &n
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO scheduler_state VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(channel_id) DO UPDATE SET fingerprint=excluded.fingerprint,config_fingerprint=excluded.config_fingerprint,membership_json=excluded.membership_json,revision=excluded.revision,next_sequence=excluded.next_sequence,covered_until=excluded.covered_until,status=excluded.status,last_generated=excluded.last_generated,last_error=''`, id, fp, conf, string(encoded), revision, seq, out.End, out.Status, last, "")
	if e != nil {
		return out, e
	}
	if actor != "" {
		action := "SCHEDULE_GENERATED"
		if force {
			action = "SCHEDULE_REGENERATED"
		} else if out.Repaired > 0 {
			action = "SCHEDULE_REPAIRED"
		}
		if e = s.audit(ctx, tx, actor, action, id, fmt.Sprintf("Future range [%d,%d); generated %d; replaced %d", out.Start, out.End, out.Generated, out.Repaired)); e != nil {
			return out, e
		}
	}
	if job != "" {
		var cancelled bool
		if e = tx.QueryRowContext(ctx, "SELECT cancel_requested FROM jobs WHERE id=?", job).Scan(&cancelled); e != nil {
			return out, e
		}
		if cancelled {
			return out, context.Canceled
		}
		_, e = tx.ExecContext(ctx, "UPDATE jobs SET status='SUCCEEDED',completed_at=?,processed=?,total=?,repaired=?,affected_start=?,affected_end=? WHERE id=?", s.Now().Unix(), out.Generated, out.Generated, out.Repaired, out.Start, out.End, job)
		if e != nil {
			return out, e
		}
	}
	if e = ctx.Err(); e != nil {
		return out, e
	}
	return out, tx.Commit()
}
