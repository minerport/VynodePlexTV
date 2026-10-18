package personalization

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"time"
)

func (s *Service) Enqueue(ctx context.Context, user string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	_, err := s.db.ExecContext(ctx, `INSERT INTO jobs(id,type,status,created_at,actor_id) SELECT ?,'REBUILD_PERSONALIZATION','QUEUED',?,? WHERE NOT EXISTS(SELECT 1 FROM jobs WHERE type='REBUILD_PERSONALIZATION' AND actor_id=? AND status IN ('QUEUED','RUNNING'))`, id, time.Now().Unix(), user, user)
	if err != nil {
		return "", err
	}
	err = s.db.QueryRowContext(ctx, `SELECT id FROM jobs WHERE type='REBUILD_PERSONALIZATION' AND actor_id=? AND status IN ('QUEUED','RUNNING')`, user).Scan(&id)
	return id, err
}
func (s *Service) Maintain(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id FROM personalization_state WHERE (dirty=1 AND rebuilt_at<?) OR rebuilt_at<? ORDER BY rebuilt_at LIMIT 100`, time.Now().Add(-time.Minute).UnixMilli(), time.Now().Add(-24*time.Hour).UnixMilli())
	if err != nil {
		return err
	}
	users := []string{}
	for rows.Next() {
		var user string
		if err = rows.Scan(&user); err != nil {
			rows.Close()
			return err
		}
		users = append(users, user)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, user := range users {
		if _, err = s.Enqueue(ctx, user); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='QUEUED',started_at=NULL WHERE type='REBUILD_PERSONALIZATION' AND status='RUNNING'`); err != nil {
		return err
	}
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	for {
		if err := s.Maintain(ctx); err != nil && ctx.Err() == nil {
			return err
		}
		var id, user string
		err := s.db.QueryRowContext(ctx, `SELECT id,actor_id FROM jobs WHERE type='REBUILD_PERSONALIZATION' AND status='QUEUED' ORDER BY created_at,id LIMIT 1`).Scan(&id, &user)
		if err != nil && err != sql.ErrNoRows && ctx.Err() == nil {
			return err
		}
		if err == nil {
			if _, err = s.db.ExecContext(ctx, `UPDATE jobs SET status='RUNNING',started_at=? WHERE id=?`, time.Now().Unix(), id); err != nil {
				return err
			}
			err = s.Rebuild(ctx, user, time.Now())
			if ctx.Err() != nil {
				return nil
			}
			state, message := "SUCCEEDED", ""
			if err != nil {
				state, message = "FAILED", "Personalization rebuild failed"
			}
			if _, err = s.db.ExecContext(ctx, `UPDATE jobs SET status=?,error=?,completed_at=? WHERE id=?`, state, message, time.Now().Unix(), id); err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}
