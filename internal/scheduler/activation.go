package scheduler

import (
	"context"
	"database/sql"
)

// Explicit local-date windows are converted to UTC at configuration time.
// An entry that has begun may finish beyond the end; it is never truncated.
func (s *Service) activation(ctx context.Context, id string) (int64, int64, error) {
	var start, end int64
	e := s.db.QueryRowContext(ctx, "SELECT start_ms,end_ms FROM channel_activation_windows WHERE channel_id=?", id).Scan(&start, &end)
	if e == sql.ErrNoRows {
		return 0, 0, nil
	}
	return start, end, e
}
