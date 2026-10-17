package playback

import (
	"context"
	"io"
	"time"
)

type cancelBody struct {
	io.ReadCloser
	cancel func()
}

func (b *cancelBody) Close() error { e := b.ReadCloser.Close(); b.cancel(); return e }
func (s *Service) Active(ctx context.Context) ([]Active, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT v.id,u.username,c.channel_number||' · '||c.name,m.title,v.client_type,v.playback_mode,v.state,v.delivery_mode,v.started_at,v.last_seen_at FROM viewing_sessions v JOIN users u ON u.id=v.user_id JOIN channels c ON c.id=v.channel_id JOIN plex_media_items m ON m.id=v.media_item_id WHERE v.state NOT IN ('STOPPED','ERROR') AND v.last_seen_at>? ORDER BY v.started_at DESC LIMIT 200`, s.Now().Add(-90*time.Second).UnixMilli())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Active{}
	for rows.Next() {
		var v Active
		if e = rows.Scan(&v.ID, &v.User, &v.Channel, &v.Program, &v.Client, &v.Mode, &v.State, &v.Delivery, &v.Started, &v.LastSeen); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Service) StopAuth(ctx context.Context, auth string) error {
	s.mu.Lock()
	ids := map[string]string{}
	for id, v := range s.sessions {
		if v.auth == auth {
			ids[id] = v.user
		}
	}
	s.mu.Unlock()
	for id, user := range ids {
		if e := s.Stop(ctx, user, id); e != nil {
			return e
		}
	}
	return nil
}
func (s *Service) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	sessions := map[string]*liveSession{}
	for id, v := range s.sessions {
		sessions[id] = v
	}
	s.mu.Unlock()
	for id, v := range sessions {
		v.mu.Lock()
		expired := s.Now().UnixMilli()-v.last > 90000
		v.mu.Unlock()
		var authExists bool
		if e := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=? AND expires_at>?)", v.auth, s.Now().Unix()).Scan(&authExists); e != nil {
			return e
		}
		if expired || !authExists {
			if e := s.Stop(ctx, v.user, id); e != nil {
				return e
			}
		}
	}
	if _, e := s.db.ExecContext(ctx, "UPDATE viewing_sessions SET state='STOPPED' WHERE state<>'STOPPED' AND last_seen_at<?", s.Now().Add(-90*time.Second).UnixMilli()); e != nil {
		return e
	}
	_, e := s.db.ExecContext(ctx, "DELETE FROM viewing_sessions WHERE state IN ('STOPPED','ERROR') AND last_seen_at<?", s.Now().Add(-7*24*time.Hour).UnixMilli())
	return e
}
func (s *Service) Recover(ctx context.Context) error {
	// Restarted process cannot reuse stale proxy descriptors. Stop only our persisted Plex session IDs.
	rows, e := s.db.QueryContext(ctx, "SELECT id,plex_server_id,delivery_mode FROM viewing_sessions WHERE state<>'STOPPED'")
	if e != nil {
		return e
	}
	type old struct{ id, server, mode string }
	pending := []old{}
	for rows.Next() {
		var v old
		if e = rows.Scan(&v.id, &v.server, &v.mode); e != nil {
			rows.Close()
			return e
		}
		pending = append(pending, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if _, e = s.db.ExecContext(ctx, "UPDATE viewing_sessions SET state='STOPPED' WHERE state<>'STOPPED'"); e != nil {
		return e
	}
	for _, v := range pending {
		if e = s.plex.StopPlayback(ctx, v.server, v.id, v.mode); e != nil {
			s.log.Warn("Restart cleanup could not reach Plex", "session", v.id)
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			pending := map[string]string{}
			for id, v := range s.sessions {
				pending[id] = v.user
			}
			s.mu.Unlock()
			for id, user := range pending {
				s.Stop(context.Background(), user, id)
			}
			return nil
		case <-ticker.C:
			if e := s.Cleanup(ctx); e != nil {
				return e
			}
		}
	}
}
