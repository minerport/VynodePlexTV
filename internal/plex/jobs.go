package plex

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type Job struct {
	ID              string  `json:"id"`
	Type            string  `json:"type"`
	ServerID        *string `json:"serverId"`
	LibraryID       *string `json:"libraryId"`
	Mode            string  `json:"mode"`
	Status          string  `json:"status"`
	CreatedAt       int64   `json:"createdAt"`
	StartedAt       *int64  `json:"startedAt"`
	CompletedAt     *int64  `json:"completedAt"`
	Processed       int     `json:"processed"`
	Total           *int    `json:"total"`
	CurrentItem     string  `json:"currentItem"`
	Error           string  `json:"error"`
	CancelRequested bool    `json:"cancelRequested"`
}

const jobColumns = `id,type,server_id,library_id,mode,status,created_at,started_at,completed_at,processed,total,current_item,error,cancel_requested`

func scanJob(row scanner) (Job, error) {
	var j Job
	err := row.Scan(&j.ID, &j.Type, &j.ServerID, &j.LibraryID, &j.Mode, &j.Status, &j.CreatedAt, &j.StartedAt, &j.CompletedAt, &j.Processed, &j.Total, &j.CurrentItem, &j.Error, &j.CancelRequested)
	return j, err
}
func (s *Service) Jobs(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+jobColumns+" FROM jobs ORDER BY created_at DESC,rowid DESC LIMIT 100")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Job{}
	for rows.Next() {
		j, e := scanJob(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, j)
	}
	return out, rows.Err()
}
func (s *Service) Job(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(s.db.QueryRowContext(ctx, "SELECT "+jobColumns+" FROM jobs WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return j, failure("NOT_FOUND", "Job not found")
	}
	return j, err
}
func (s *Service) Enqueue(ctx context.Context, kind, serverID, libraryID, mode, actor string) (Job, error) {
	if kind != "PLEX_DISCOVERY" && kind != "PLEX_HEALTH_CHECK" && kind != "PLEX_LIBRARY_SYNC" && kind != "PLEX_ARTWORK_MAINTENANCE" {
		return Job{}, failure("INVALID", "Unsupported job type")
	}
	if mode == "" {
		mode = "incremental"
	}
	if mode != "full" && mode != "incremental" {
		return Job{}, failure("INVALID", "Sync mode must be full or incremental")
	}
	if kind == "PLEX_LIBRARY_SYNC" {
		l, err := s.Library(ctx, libraryID)
		if err != nil {
			return Job{}, err
		}
		if !l.Selected || !l.Present || !l.Supported {
			return Job{}, failure("INVALID", "Select a present movie or TV library before synchronizing")
		}
		serverID = l.ServerID
	}
	if kind != "PLEX_ARTWORK_MAINTENANCE" {
		v, err := s.Server(ctx, serverID)
		if err != nil {
			return Job{}, err
		}
		if v.Disabled {
			return Job{}, failure("INVALID", "Reconnect this server before starting jobs")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Job{}, err
	}
	defer tx.Rollback()
	var active string
	err = tx.QueryRowContext(ctx, `SELECT id FROM jobs WHERE type=? AND COALESCE(server_id,'')=? AND COALESCE(library_id,'')=? AND status IN ('QUEUED','RUNNING')`, kind, serverID, libraryID).Scan(&active)
	if err == nil {
		tx.Rollback()
		return s.Job(ctx, active)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Job{}, err
	}
	var server, library any
	if serverID != "" {
		server = serverID
	}
	if libraryID != "" {
		library = libraryID
	}
	id := newID()
	if _, err = tx.ExecContext(ctx, `INSERT INTO jobs(id,type,server_id,library_id,mode,status,created_at) VALUES(?,?,?,?,?,'QUEUED',?)`, id, kind, server, library, mode, now()); err != nil {
		return Job{}, err
	}
	if actor != "" {
		if err = audit(ctx, tx, actor, "plex.job.queued"); err != nil {
			return Job{}, err
		}
	}
	if err = tx.Commit(); err != nil {
		return Job{}, err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return s.Job(ctx, id)
}
func (s *Service) Cancel(ctx context.Context, id, actor string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE jobs SET cancel_requested=1,status=CASE WHEN status='QUEUED' THEN 'CANCELLED' ELSE status END,completed_at=CASE WHEN status='QUEUED' THEN ? ELSE completed_at END WHERE id=? AND status IN ('QUEUED','RUNNING')`, now(), id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return failure("CONFLICT", "This job has already completed or does not exist")
	}
	if err = audit(ctx, tx, actor, "plex.job.cancelled"); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	if s.activeID == id && s.activeCancel != nil {
		s.activeCancel()
	}
	s.mu.Unlock()
	return nil
}

// Run is a single bounded worker. Restarted RUNNING jobs are cancelled, never resumed into reconciliation.
func (s *Service) Run(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET status='CANCELLED',cancel_requested=1,completed_at=?,error='Application restarted during job; inventory preserved' WHERE status='RUNNING' AND type LIKE 'PLEX_%'`, now()); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sync_seen WHERE job_id IN (SELECT id FROM jobs WHERE status NOT IN ('RUNNING','QUEUED'))"); err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastScheduled := time.Time{}
	for {
		if ctx.Err() != nil {
			return nil
		}
		if time.Since(lastScheduled) > 30*time.Second {
			if err := s.schedule(ctx); err != nil && ctx.Err() == nil {
				s.log.Error("Plex job scheduling failed")
			}
			lastScheduled = time.Now()
		}
		worked, err := s.runNext(ctx)
		if err != nil && ctx.Err() == nil {
			s.log.Error("Plex job worker failed", "message", SafeMessage(err))
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.wake:
		case <-ticker.C:
		}
	}
}
func (s *Service) runNext(ctx context.Context) (bool, error) {
	j, err := scanJob(s.db.QueryRowContext(ctx, "SELECT "+jobColumns+" FROM jobs WHERE status='QUEUED' AND type LIKE 'PLEX_%' ORDER BY created_at,rowid LIMIT 1"))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	jobCtx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.activeID = j.ID
	s.activeCancel = cancel
	s.mu.Unlock()
	defer func() { cancel(); s.mu.Lock(); s.activeID = ""; s.activeCancel = nil; s.mu.Unlock() }()
	res, err := s.db.ExecContext(ctx, "UPDATE jobs SET status='RUNNING',started_at=? WHERE id=? AND status='QUEUED' AND cancel_requested=0", now(), j.ID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return true, nil
	}
	switch j.Type {
	case "PLEX_DISCOVERY":
		err = s.discover(jobCtx, *j.ServerID)
	case "PLEX_HEALTH_CHECK":
		err = s.health(jobCtx, *j.ServerID)
	case "PLEX_LIBRARY_SYNC":
		err = s.syncLibrary(jobCtx, j)
	case "PLEX_ARTWORK_MAINTENANCE":
		err = s.maintainArtwork(jobCtx)
	}
	status := "SUCCEEDED"
	summary := ""
	if err != nil {
		status = "FAILED"
		summary = SafeMessage(err)
	}
	// Use a short independent context to persist cancellation during application shutdown.
	finish, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	current, e := s.Job(finish, j.ID)
	if e != nil {
		return true, e
	}
	if current.Status == "SUCCEEDED" {
		return true, nil
	}
	if jobCtx.Err() != nil || current.CancelRequested || errors.Is(err, contextCanceled) {
		status = "CANCELLED"
		summary = "Job interrupted or cancelled; existing inventory preserved"
	}
	if _, e = s.db.ExecContext(finish, "UPDATE jobs SET status=?,completed_at=?,error=? WHERE id=? AND status='RUNNING'", status, now(), summary, j.ID); e != nil {
		return true, e
	}
	if _, e = s.db.ExecContext(finish, "DELETE FROM sync_seen WHERE job_id=?", j.ID); e != nil {
		return true, e
	}
	if j.ServerID != nil && status == "FAILED" {
		state := "DEGRADED"
		switch ErrorCode(err) {
		case "UNAUTHORIZED":
			state = "UNAUTHORIZED"
		case "OFFLINE", "TIMEOUT", "DNS":
			state = "OFFLINE"
		}
		if _, e = s.db.ExecContext(finish, "UPDATE plex_servers SET status=?,last_error=?,updated_at=? WHERE id=?", state, summary, now(), *j.ServerID); e != nil {
			return true, e
		}
	}
	s.log.Info("Plex job finished", "job", j.ID, "type", j.Type, "status", status)
	return true, nil
}
func (s *Service) schedule(ctx context.Context) error {
	s.linkMu.Lock()
	s.pruneLinksLocked()
	s.linkMu.Unlock()
	options, err := s.Settings(ctx)
	if err != nil {
		return err
	}
	servers, err := s.Servers(ctx)
	if err != nil {
		return err
	}
	for _, server := range servers {
		if server.Disabled {
			continue
		}
		var last int64
		if err = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(created_at),0) FROM jobs WHERE type='PLEX_HEALTH_CHECK' AND server_id=?", server.ID).Scan(&last); err != nil {
			return err
		}
		if now()-last >= int64(options.HealthIntervalSeconds) {
			if _, err = s.Enqueue(ctx, "PLEX_HEALTH_CHECK", server.ID, "", "", ""); err != nil {
				return err
			}
		}
		libraries, err := s.Libraries(ctx, server.ID)
		if err != nil {
			return err
		}
		for _, l := range libraries {
			if !l.Selected || !l.Present || !l.Supported {
				continue
			}
			if err = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(created_at),0) FROM jobs WHERE type='PLEX_LIBRARY_SYNC' AND library_id=?", l.ID).Scan(&last); err != nil {
				return err
			}
			if now()-last < int64(options.SyncIntervalSeconds) {
				continue
			}
			mode := "incremental"
			if l.LastFullSync == nil || now()-*l.LastFullSync >= int64(options.FullSyncIntervalSeconds) {
				mode = "full"
			}
			if _, err = s.Enqueue(ctx, "PLEX_LIBRARY_SYNC", server.ID, l.ID, mode, ""); err != nil {
				return err
			}
		}
	}
	var last int64
	if err = s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(created_at),0) FROM jobs WHERE type='PLEX_ARTWORK_MAINTENANCE'").Scan(&last); err != nil {
		return err
	}
	if now()-last >= 3600 {
		_, err = s.Enqueue(ctx, "PLEX_ARTWORK_MAINTENANCE", "", "", "", "")
	}
	return err
}
