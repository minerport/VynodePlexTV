package personalization

import (
	"context"
	"database/sql"
)

// Heartbeats are observations, not proof of playing through a seek or disconnect.
const MaxHeartbeatCreditMS int64 = 30000
const MeaningfulWatchMS int64 = 120000
const MinimumAffinitySessions = 3
const HistoryRetentionDays = 180
const RankingVersion = 1

type HistoryStart struct {
	ID, UserID, ChannelID, ProgramID, MediaID, Origin, ClientType string
	At, Position, Duration                                        int64
}

func (s *Service) StartHistory(ctx context.Context, v HistoryStart) error {
	if v.Duration <= 0 {
		return ErrInvalid
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO user_program_history(id,user_id,channel_id,program_id,media_item_id,started_at,program_duration_ms,playback_origin,client_type,last_sample_at,last_position_ms,last_state) VALUES(?,?,?,?,?,?,?,?,?,?,?,'STARTING') ON CONFLICT(id) DO NOTHING`, v.ID, v.UserID, v.ChannelID, v.ProgramID, v.MediaID, v.At, v.Duration, v.Origin, v.ClientType, v.At, v.Position)
	return err
}
func Credit(previousAt, now, previousPosition, position int64, previousState, state string) int64 {
	elapsed := now - previousAt
	advance := position - previousPosition
	if previousState != "PLAYING" || (state != "PLAYING" && state != "PAUSED") || elapsed <= 0 || advance <= 0 || elapsed > MaxHeartbeatCreditMS || advance > elapsed+2000 {
		return 0
	}
	return min(elapsed, advance)
}

// Clearing history deletes the row; late heartbeats deliberately cannot recreate it.
func (s *Service) ObserveHistory(ctx context.Context, user, id string, now, position int64, state string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var at, prior, watched, duration int64
	var oldState string
	err = tx.QueryRowContext(ctx, `SELECT last_sample_at,last_position_ms,last_state,watch_duration_ms,program_duration_ms FROM user_program_history WHERE id=? AND user_id=? AND ended_at IS NULL`, id, user).Scan(&at, &prior, &oldState, &watched, &duration)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if now <= at {
		return nil
	}
	watched += Credit(at, now, prior, position, oldState, state)
	_, err = tx.ExecContext(ctx, `UPDATE user_program_history SET last_sample_at=?,last_position_ms=?,last_state=?,watch_duration_ms=?,completion_ratio=?,ended_at=CASE WHEN ? IN ('STOPPED','ERROR') THEN ? ELSE NULL END WHERE id=? AND user_id=?`, now, position, state, watched, min(1, float64(watched)/float64(duration)), state, now, id, user)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO personalization_state(user_id,dirty) VALUES(?,1) ON CONFLICT(user_id) DO UPDATE SET dirty=1`, user)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) EndHistory(ctx context.Context, user, id string, at int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE user_program_history SET ended_at=MAX(last_sample_at,?),last_state='STOPPED' WHERE id=? AND user_id=? AND ended_at IS NULL`, at, id, user)
	return err
}
func (s *Service) ClearHistory(ctx context.Context, user string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"user_program_history", "user_content_affinities", "personalization_state"} {
		if _, err = tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE user_id=?", user); err != nil {
			return err
		}
	}
	return tx.Commit()
}

type History struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	ChannelID  *string `json:"channelId"`
	Started    int64   `json:"startedAt"`
	Ended      *int64  `json:"endedAt"`
	Watched    int64   `json:"watchDurationMs"`
	Duration   int64   `json:"programDurationMs"`
	Completion float64 `json:"completionRatio"`
	Origin     string  `json:"origin"`
}

func (s *Service) History(ctx context.Context, user string, offset int) ([]History, error) {
	if offset < 0 || offset > 100000 {
		return nil, ErrInvalid
	}
	out := []History{}
	rows, err := s.db.QueryContext(ctx, `SELECT h.id,m.title,h.channel_id,h.started_at,h.ended_at,h.watch_duration_ms,h.program_duration_ms,h.completion_ratio,h.playback_origin FROM user_program_history h JOIN plex_media_items m ON m.id=h.media_item_id WHERE h.user_id=? ORDER BY h.started_at DESC,h.id LIMIT 50 OFFSET ?`, user, offset)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var h History
		if err = rows.Scan(&h.ID, &h.Title, &h.ChannelID, &h.Started, &h.Ended, &h.Watched, &h.Duration, &h.Completion, &h.Origin); err != nil {
			return out, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
