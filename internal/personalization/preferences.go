// Package personalization owns private discovery signals and browsing preferences.
// It does not alter shared broadcast timelines.
package personalization

import (
	"context"
	"database/sql"
	"errors"
)

var ErrInvalid = errors.New("Invalid personal channel preference")

type Service struct{ db *sql.DB }

func New(db *sql.DB) *Service { return &Service{db: db} }

type Channel struct {
	ID       string `json:"id"`
	Number   string `json:"number"`
	Name     string `json:"name"`
	Hidden   bool   `json:"hidden"`
	Favorite bool   `json:"favorite"`
}
type Preferences struct {
	Mode     string    `json:"mode"`
	Channels []Channel `json:"channels"`
}

func (s *Service) Channels(ctx context.Context, user string) (Preferences, error) {
	out := Preferences{Mode: "NUMBER", Channels: []Channel{}}
	err := s.db.QueryRowContext(ctx, "SELECT order_mode FROM viewer_preferences WHERE user_id=?", user).Scan(&out.Mode)
	if err != nil && err != sql.ErrNoRows {
		return out, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.id,c.channel_number,c.name,COALESCE(p.hidden,0),f.user_id IS NOT NULL FROM channels c LEFT JOIN user_channel_preferences p ON p.channel_id=c.id AND p.user_id=? LEFT JOIN user_channel_favorites f ON f.channel_id=c.id AND f.user_id=? WHERE c.enabled=1 AND c.visibility='VISIBLE' ORDER BY COALESCE(p.position,2147483647),CAST(c.channel_number AS REAL),c.channel_number,c.id`, user, user)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Channel
		if err = rows.Scan(&c.ID, &c.Number, &c.Name, &c.Hidden, &c.Favorite); err != nil {
			return out, err
		}
		out.Channels = append(out.Channels, c)
	}
	return out, rows.Err()
}

// Order is an atomic permutation of all currently visible enabled channels.
// Hidden channels participate so showing them restores their previous position.
func (s *Service) Order(ctx context.Context, user, mode string, ids []string) error {
	if mode != "NUMBER" && mode != "PERSONAL" {
		return ErrInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if ids != nil {
		rows, e := tx.QueryContext(ctx, `SELECT id FROM channels WHERE enabled=1 AND visibility='VISIBLE'`)
		if e != nil {
			return e
		}
		valid := map[string]bool{}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			valid[id] = true
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(ids) != len(valid) {
			return ErrInvalid
		}
		for pos, id := range ids {
			if !valid[id] {
				return ErrInvalid
			}
			delete(valid, id)
			if _, e = tx.ExecContext(ctx, `INSERT INTO user_channel_preferences(user_id,channel_id,position) VALUES(?,?,?) ON CONFLICT(user_id,channel_id) DO UPDATE SET position=excluded.position`, user, id, pos); e != nil {
				return e
			}
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO viewer_preferences(user_id,updated_at,order_mode) VALUES(?,unixepoch()*1000,?) ON CONFLICT(user_id) DO UPDATE SET order_mode=excluded.order_mode,updated_at=excluded.updated_at`, user, mode)
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) Hide(ctx context.Context, user, id string, hidden bool) error {
	result, err := s.db.ExecContext(ctx, `INSERT INTO user_channel_preferences(user_id,channel_id,hidden) SELECT ?,id,? FROM channels WHERE id=? AND enabled=1 AND visibility='VISIBLE' ON CONFLICT(user_id,channel_id) DO UPDATE SET hidden=excluded.hidden`, user, hidden, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n == 0 {
		return ErrInvalid
	}
	return err
}
func (s *Service) Reset(ctx context.Context, user string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_channel_preferences WHERE user_id=?", user); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE viewer_preferences SET order_mode='NUMBER' WHERE user_id=?", user); err != nil {
		return err
	}
	return tx.Commit()
}
