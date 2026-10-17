package playback

import (
	"afterglow.local/tv/internal/plex"
	"afterglow.local/tv/internal/scheduler"
	"context"
	"database/sql"
	"log/slog"
	"math"
	"net/http"
	"sync"
	"time"
)

type liveSession struct {
	mu                         sync.Mutex
	user, auth, client, server string
	caps                       plex.Capabilities
	source                     plex.PlaybackSource
	tune                       Tune
	last                       int64
	closed                     bool
	cancel                     context.CancelFunc
	ctx                        context.Context
}
type Service struct {
	db        *sql.DB
	schedule  *scheduler.Service
	plex      Adapter
	log       *slog.Logger
	mu        sync.Mutex
	sessions  map[string]*liveSession
	Now       func() time.Time
	tuneSlots chan struct{}
}

func New(db *sql.DB, schedule *scheduler.Service, adapter Adapter, log *slog.Logger) *Service {
	return &Service{db: db, schedule: schedule, plex: adapter, log: log, sessions: map[string]*liveSession{}, Now: time.Now, tuneSlots: make(chan struct{}, 8)}
}
func (s *Service) channel(ctx context.Context, id string) (Channel, error) {
	var c Channel
	e := s.db.QueryRowContext(ctx, `SELECT c.id,c.channel_number,c.name,COALESCE(json_extract(src.config_json,'$.recent.kind'),''),COALESCE((SELECT timezone FROM channel_schedule_settings WHERE channel_id=c.id),'UTC') FROM channels c JOIN channel_sources src ON src.channel_id=c.id WHERE c.id=? AND c.enabled=1 AND c.visibility='VISIBLE'`, id).Scan(&c.ID, &c.Number, &c.Name, &c.RecentKind, &c.Timezone)
	if e == sql.ErrNoRows {
		return c, failure("NOT_FOUND", "Channel is unavailable or disabled")
	}
	c.Logo = "/api/v1/watch/channels/" + c.ID + "/artwork/logo"
	return c, e
}
func (s *Service) Channel(ctx context.Context, id string) (Channel, error) { return s.channel(ctx, id) }
func safeProgram(p *scheduler.Entry) {
	if p != nil {
		p.Artwork = "/api/v1/watch/channels/" + p.ChannelID + "/programs/" + p.ID + "/artwork"
	}
}
func (s *Service) OnNow(ctx context.Context) ([]OnNow, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id FROM channels WHERE enabled=1 AND visibility='VISIBLE' ORDER BY CAST(channel_number AS REAL),channel_number,id LIMIT 1000")
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
	out := []OnNow{}
	at := s.Now()
	for _, id := range ids {
		c, e := s.channel(ctx, id)
		if e != nil {
			return nil, e
		}
		p, e := s.schedule.GetProgramAt(ctx, id, at)
		if e != nil {
			return nil, e
		}
		next, e := s.schedule.Next(ctx, id, at)
		if e != nil {
			return nil, e
		}
		safeProgram(p)
		safeProgram(next)
		progress := float64(0)
		if p != nil && p.Duration > 0 {
			progress = math.Min(100, math.Max(0, float64(at.UnixMilli()-p.Start)/float64(p.Duration)*100))
		}
		out = append(out, OnNow{c, p, next, progress})
	}
	return out, nil
}
func (s *Service) Tune(ctx context.Context, user, auth string, r TuneRequest) (Tune, error) {
	var out Tune
	select {
	case s.tuneSlots <- struct{}{}:
		defer func() { <-s.tuneSlots }()
	default:
		return out, failure("BUSY", "Several channels are tuning. Try again in a moment")
	}
	if r.Mode != "LIVE" && r.Mode != "FROM_BEGINNING" {
		return out, failure("INVALID", "Choose Watch Live or Watch From Beginning")
	}
	if len(r.ClientID) < 1 || len(r.ClientID) > 100 || r.Capabilities.ClientType != "browser" {
		return out, failure("INVALID", "Invalid playback client")
	}
	c, e := s.channel(ctx, r.ChannelID)
	if e != nil {
		return out, e
	}
	p, e := s.schedule.GetProgramAt(ctx, c.ID, s.Now())
	if e != nil {
		return out, e
	}
	if p == nil {
		return out, failure("EMPTY", "No program is scheduled now. Ask an administrator to generate this channel's schedule")
	}
	if !p.Available {
		return out, failure("NOT_FOUND", "This program is currently unavailable")
	}
	if r.ReplaceSession != "" {
		if e = s.Stop(ctx, user, r.ReplaceSession); e != nil {
			return out, e
		}
	}
	s.mu.Lock()
	active := 0
	for _, v := range s.sessions {
		if v.user == user {
			active++
		}
	}
	s.mu.Unlock()
	if active >= 8 {
		return out, failure("BUSY", "Too many active viewers for this account. Close another player or wait for expired sessions to clear")
	}
	id := uid()
	resolved, e := s.plex.ResolvePlayback(ctx, p.MediaID, r.Capabilities, id)
	if e != nil {
		return out, e
	}
	now := s.Now()
	if now.UnixMilli() >= p.End {
		resolved.Source.Close(context.Background())
		return out, failure("BOUNDARY", "The program just changed. Retry to join the next program")
	}
	next, e := s.schedule.Next(ctx, c.ID, now)
	if e != nil {
		resolved.Source.Close(context.Background())
		return out, e
	}
	safeProgram(p)
	safeProgram(next)
	offset := Offset(now.UnixMilli(), p.Start, resolved.Duration)
	position := offset
	if r.Mode == "FROM_BEGINNING" {
		position = 0
	}
	out = Tune{ID: id, Channel: c, Program: *p, Next: next, Mode: r.Mode, State: "STARTING", ServerTime: now.UnixMilli(), LiveOffset: offset, Position: position, Duration: resolved.Duration, StreamURL: "/api/v1/playback/" + id + "/stream/main", Mime: resolved.Mime, Delivery: resolved.Mode, AddedAt: resolved.AddedAt}
	_, e = s.db.ExecContext(ctx, `INSERT INTO viewing_sessions(id,user_id,channel_id,program_id,media_item_id,plex_server_id,auth_session_hash,started_at,last_seen_at,playback_mode,scheduled_live_offset_ms,current_position_ms,state,client_identifier,client_type,delivery_mode) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, id, user, c.ID, p.ID, p.MediaID, resolved.ServerID, auth, now.UnixMilli(), now.UnixMilli(), r.Mode, offset, position, "STARTING", r.ClientID, r.Capabilities.ClientType, resolved.Mode)
	if e != nil {
		resolved.Source.Close(context.Background())
		return Tune{}, e
	}
	streamCtx, cancel := context.WithCancel(context.Background())
	v := &liveSession{user: user, auth: auth, client: r.ClientID, server: resolved.ServerID, caps: r.Capabilities, source: resolved.Source, tune: out, last: now.UnixMilli(), ctx: streamCtx, cancel: cancel}
	s.mu.Lock()
	s.sessions[id] = v
	s.mu.Unlock()
	return out, nil
}
func (s *Service) owned(user, id string) (*liveSession, error) {
	s.mu.Lock()
	v := s.sessions[id]
	s.mu.Unlock()
	if v == nil || v.user != user {
		return nil, failure("NOT_FOUND", "Playback session has ended. Tune again")
	}
	return v, nil
}
func (s *Service) Get(user, id string) (Tune, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return Tune{}, e
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return Tune{}, failure("NOT_FOUND", "Playback session has ended")
	}
	return v.tune, nil
}
func (s *Service) Heartbeat(ctx context.Context, user, id string, h Heartbeat) (Update, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return Update{}, e
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.closed {
		return Update{}, failure("NOT_FOUND", "Playback session has ended")
	}
	if h.State != "PLAYING" && h.State != "PAUSED" && h.State != "ERROR" {
		return Update{}, failure("INVALID", "Invalid playback state")
	}
	now := s.Now().UnixMilli()
	if h.Position < 0 || h.Position > v.tune.Duration+5000 {
		return Update{}, failure("INVALID", "Invalid playback position")
	}
	if _, e = s.channel(ctx, v.tune.Channel.ID); e != nil {
		return Update{}, e
	}
	position := min(h.Position, v.tune.Duration)
	live := Offset(now, v.tune.Program.Start, v.tune.Duration)
	transition := v.tune.Mode == "LIVE" && h.State == "PLAYING" && now >= v.tune.Program.End
	mode := v.tune.Mode
	if !transition {
		if h.State == "PAUSED" || math.Abs(float64(position-live)) > 10000 || now >= v.tune.Program.End {
			if mode != "FROM_BEGINNING" {
				mode = "BEHIND_LIVE"
			}
		} else if h.State == "PLAYING" {
			mode = "LIVE"
		}
	}
	message := ""
	if h.Ended && now < v.tune.Program.End-10000 {
		message = "Media ended before its scheduled slot. Go Live or wait for the next program"
		s.log.Warn("Media ended before schedule boundary", "channel", v.tune.Channel.ID, "program", v.tune.Program.ID)
	}
	_, e = s.db.ExecContext(ctx, `UPDATE viewing_sessions SET last_seen_at=?,playback_mode=?,scheduled_live_offset_ms=?,current_position_ms=?,state=? WHERE id=?`, now, mode, live, position, h.State, id)
	if e != nil {
		return Update{}, e
	}
	v.last = now
	v.tune.Mode = mode
	v.tune.Position = position
	v.tune.State = h.State
	v.tune.ServerTime = now
	v.tune.LiveOffset = live
	return Update{now, mode, live, transition, h.Ended, message}, nil
}
func (s *Service) GoLive(ctx context.Context, user, auth, id string) (Tune, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return Tune{}, e
	}
	v.mu.Lock()
	r := TuneRequest{ChannelID: v.tune.Channel.ID, Mode: "LIVE", ClientID: v.client, Capabilities: v.caps, ReplaceSession: id}
	v.mu.Unlock()
	return s.Tune(ctx, user, auth, r)
}
func (s *Service) Stop(ctx context.Context, user, id string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	v, e := s.owned(user, id)
	if e != nil {
		var owner string
		err := s.db.QueryRowContext(ctx, "SELECT user_id FROM viewing_sessions WHERE id=?", id).Scan(&owner)
		if err == nil && owner == user {
			return nil
		}
		return e
	}
	v.mu.Lock()
	if v.closed {
		v.mu.Unlock()
		return nil
	}
	v.closed = true
	v.cancel()
	v.mu.Unlock()
	_, e = s.db.ExecContext(ctx, "UPDATE viewing_sessions SET state='STOPPED',last_seen_at=? WHERE id=?", s.Now().UnixMilli(), id)
	if err := v.source.Close(context.Background()); err != nil {
		s.log.Warn("Plex session cleanup deferred to server timeout", "session", id)
	}
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
	return e
}
func (s *Service) Fetch(ctx context.Context, user, id, resource, rangeHeader string) (*http.Response, error) {
	v, e := s.owned(user, id)
	if e != nil {
		return nil, e
	}
	v.mu.Lock()
	closed := v.closed
	expired := s.Now().UnixMilli()-v.last > 90000
	channel := v.tune.Channel.ID
	v.mu.Unlock()
	if closed || expired {
		return nil, failure("NOT_FOUND", "Playback session expired. Tune again")
	}
	if _, e = s.channel(ctx, channel); e != nil {
		return nil, e
	}
	requestCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(v.ctx, cancel)
	r, e := v.source.Fetch(requestCtx, resource, rangeHeader)
	if e != nil {
		stop()
		cancel()
		return nil, e
	}
	r.Body = &cancelBody{ReadCloser: r.Body, cancel: func() { stop(); cancel() }}
	return r, nil
}
