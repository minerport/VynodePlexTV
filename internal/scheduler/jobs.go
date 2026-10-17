package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Job struct {
	ID        string `json:"id"`
	ChannelID string `json:"channelId"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Processed int    `json:"processed"`
	Repaired  int    `json:"repaired"`
	Error     string `json:"error"`
	Created   int64  `json:"createdAt"`
	Started   *int64 `json:"startedAt"`
	Completed *int64 `json:"completedAt"`
	Start     *int64 `json:"affectedStart"`
	End       *int64 `json:"affectedEnd"`
}

func (s *Service) Jobs(ctx context.Context, id string) ([]Job, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT id,channel_id,type,status,processed,repaired,error,created_at,started_at,completed_at,affected_start,affected_end FROM jobs WHERE type LIKE 'SCHEDULE_%' AND (?='' OR channel_id=?) ORDER BY created_at DESC,rowid DESC LIMIT 30`, id, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		var j Job
		if e = rows.Scan(&j.ID, &j.ChannelID, &j.Type, &j.Status, &j.Processed, &j.Repaired, &j.Error, &j.Created, &j.Started, &j.Completed, &j.Start, &j.End); e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Service) Enqueue(ctx context.Context, id, kind, actor string) (string, error) {
	if kind != "SCHEDULE_GENERATE" && kind != "SCHEDULE_EXTEND" && kind != "SCHEDULE_REPAIR" && kind != "SCHEDULE_REGENERATE" {
		return "", invalid("Unsupported scheduling job")
	}
	if _, e := s.Settings(ctx, id); e != nil {
		return "", e
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	var active, activeType string
	e = tx.QueryRowContext(ctx, `SELECT id,type FROM jobs WHERE channel_id=? AND type LIKE 'SCHEDULE_%' AND status IN('QUEUED','RUNNING')`, id).Scan(&active, &activeType)
	if e == nil {
		if kind == "SCHEDULE_REGENERATE" && activeType != kind {
			return "", &Error{"CONFLICT", "Another scheduling job is active. Wait for it to finish before regenerating."}
		}
		return active, nil
	}
	if e != sql.ErrNoRows {
		return "", e
	}
	job := uid()
	var user any
	if actor != "" {
		user = actor
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO jobs(id,type,channel_id,actor_id,mode,status,created_at) VALUES(?,?,?,?,'','QUEUED',?)`, job, kind, id, user, s.Now().Unix())
	if e != nil {
		return "", e
	}
	return job, tx.Commit()
}

// One worker bounds both CPU and SQLite writes. Interrupted transactions roll back.
func (s *Service) Run(ctx context.Context) error {
	if _, e := s.db.ExecContext(ctx, `UPDATE jobs SET status='CANCELLED',completed_at=?,error='Application restarted; prior timeline preserved' WHERE status='RUNNING' AND type LIKE 'SCHEDULE_%'`, s.Now().Unix()); e != nil {
		return e
	}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	last := time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(last) > 5*time.Minute {
			if e := s.Maintain(ctx); e != nil && ctx.Err() == nil {
				s.log.Error("Scheduler maintenance failed", "message", SafeError(e))
			}
			last = time.Now()
		}
		worked, e := s.RunNext(ctx)
		if e != nil && ctx.Err() == nil {
			s.log.Error("Scheduler worker failed", "message", SafeError(e))
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
func (s *Service) Maintain(ctx context.Context) error {
	for offset := 0; ; offset += 100 {
		ids, e := s.ChannelIDs(ctx, offset, 100)
		if e != nil {
			return e
		}
		for _, id := range ids {
			var enabled bool
			if e = s.db.QueryRowContext(ctx, `SELECT c.enabled AND ss.enabled FROM channels c JOIN channel_schedule_settings ss ON ss.channel_id=c.id WHERE c.id=?`, id).Scan(&enabled); e != nil {
				return e
			}
			if enabled {
				if _, e = s.Enqueue(ctx, id, "SCHEDULE_EXTEND", ""); e != nil {
					return e
				}
			}
		}
		if len(ids) < 100 {
			return nil
		}
	}
}
func (s *Service) RunNext(ctx context.Context) (bool, error) {
	var id, channel, kind string
	var actor *string
	e := s.db.QueryRowContext(ctx, `SELECT id,channel_id,type,actor_id FROM jobs WHERE type LIKE 'SCHEDULE_%' AND status='QUEUED' ORDER BY created_at,rowid LIMIT 1`).Scan(&id, &channel, &kind, &actor)
	if e == sql.ErrNoRows {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	result, e := s.db.ExecContext(ctx, `UPDATE jobs SET status='RUNNING',started_at=? WHERE id=? AND status='QUEUED' AND cancel_requested=0`, s.Now().Unix(), id)
	if e != nil {
		return false, e
	}
	n, e := result.RowsAffected()
	if e != nil || n == 0 {
		return true, e
	}
	user := ""
	if actor != nil {
		user = *actor
	}
	out, e := s.ensure(ctx, channel, 0, kind == "SCHEDULE_REGENERATE", user, id)
	finish, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if e == nil {
		_, e = s.db.ExecContext(finish, `UPDATE jobs SET status='SUCCEEDED',completed_at=?,current_item=? WHERE id=? AND status='RUNNING'`, s.Now().Unix(), out.Status, id)
		return true, e
	}
	status := "FAILED"
	message := SafeError(e)
	if errors.Is(e, context.Canceled) || ctx.Err() != nil {
		status = "CANCELLED"
		message = "Job interrupted; prior timeline preserved"
	}
	if _, err := s.db.ExecContext(finish, `UPDATE jobs SET status=?,completed_at=?,error=? WHERE id=? AND status='RUNNING'`, status, s.Now().Unix(), message, id); err != nil {
		return true, err
	}
	if status == "FAILED" {
		_, err := s.db.ExecContext(finish, `INSERT INTO scheduler_state(channel_id,status,last_error) VALUES(?,'ERROR',?) ON CONFLICT(channel_id) DO UPDATE SET status='ERROR',last_error=excluded.last_error`, channel, message)
		if err != nil {
			return true, err
		}
	}
	s.log.Info("Schedule job finished", "job", id, "channel", channel, "status", status)
	return true, nil
}
