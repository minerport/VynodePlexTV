package plex

import (
	"context"
	"database/sql"
	"errors"
)

func findLibrary(list []Library, key string) (Library, error) {
	for _, l := range list {
		if l.Key == key {
			return l, nil
		}
	}
	return Library{}, failure("NOT_FOUND", "Plex library is no longer visible; existing inventory was preserved")
}
func (s *Service) syncLibrary(ctx context.Context, j Job) error {
	l, err := s.Library(ctx, *j.LibraryID)
	if err != nil {
		return err
	}
	if !l.Selected || !l.Present {
		return failure("INVALID", "Library is not selected or no longer present")
	}
	client, server, err := s.provider(ctx, l.ServerID)
	if err != nil {
		return err
	}
	identity, err := client.Identity(ctx)
	if err != nil {
		return err
	}
	if identity.MachineIdentifier != server.MachineIdentifier {
		return failure("IDENTITY", "Plex identity changed; existing inventory was preserved")
	}
	libraries, err := client.Libraries(ctx)
	if err != nil {
		return err
	}
	before, err := findLibrary(libraries, l.Key)
	if err != nil {
		return err
	}
	if before.Type != l.Type || before.UUID != l.UUID {
		return failure("IDENTITY", "Plex library identity changed; refresh discovery before syncing")
	}
	kinds := []int{1}
	names := []string{"movie"}
	if l.Type == "show" {
		kinds = []int{2, 3, 4}
		names = []string{"show", "season", "episode"}
	}
	first := make([]Page, len(kinds))
	total := 0
	for i, kind := range kinds {
		first[i], err = client.Page(ctx, l.Key, kind, 0)
		if err != nil {
			return err
		}
		if first[i].Total == nil || *first[i].Total < 0 {
			return failure("RESPONSE", "Plex did not provide a reliable total; reconciliation was skipped")
		}
		total += *first[i].Total
	}
	if _, err = s.db.ExecContext(ctx, "UPDATE jobs SET total=? WHERE id=?", total, j.ID); err != nil {
		return err
	}
	processed := 0
	for ki, kind := range kinds {
		expected := *first[ki].Total
		offset := 0
		p := first[ki]
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if p.Total == nil || *p.Total != expected || p.Offset != offset || len(p.Items) > expected-offset || (len(p.Items) == 0 && offset < expected) {
				return failure("RESPONSE", "Plex inventory changed or pagination ended early; reconciliation was skipped")
			}
			changed := make([]MediaItem, 0, len(p.Items))
			unchanged := []string{}
			for _, item := range p.Items {
				if item.Type != names[ki] || item.RatingKey == "" {
					return failure("RESPONSE", "Plex returned an unexpected media type or missing identity")
				}
				var localID, localLibrary string
				var updated sql.NullInt64
				err = s.db.QueryRowContext(ctx, "SELECT id,plex_library_id,updated_at_source FROM plex_media_items WHERE plex_server_id=? AND plex_rating_key=?", l.ServerID, item.RatingKey).Scan(&localID, &localLibrary, &updated)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if localID != "" && localLibrary != l.ID {
					return failure("RESPONSE", "Plex returned an item from a different library")
				}
				if j.Mode == "incremental" && localID != "" && updated.Valid && item.SourceUpdatedAt != nil && updated.Int64 == *item.SourceUpdatedAt {
					unchanged = append(unchanged, localID)
					continue
				}
				if _, err = s.db.ExecContext(ctx, "UPDATE jobs SET current_item=? WHERE id=?", item.Title, j.ID); err != nil {
					return err
				}
				detail, e := client.Metadata(ctx, item.RatingKey)
				if e != nil {
					return e
				}
				if detail.Type != item.Type {
					return failure("RESPONSE", "Plex item changed type during sync")
				}
				changed = append(changed, detail)
			}
			if err = s.upsertBatch(ctx, j.ID, l, changed); err != nil {
				return err
			}
			tx, e := s.db.BeginTx(ctx, nil)
			if e != nil {
				return e
			}
			for _, id := range unchanged {
				if _, e = tx.ExecContext(ctx, "INSERT INTO sync_seen VALUES(?,?)", j.ID, id); e != nil {
					tx.Rollback()
					return failure("RESPONSE", "Plex pagination repeated an item; reconciliation was skipped")
				}
				if _, e = tx.ExecContext(ctx, "UPDATE plex_media_items SET available=1 WHERE id=?", id); e != nil {
					tx.Rollback()
					return e
				}
			}
			processed += len(p.Items)
			if _, e = tx.ExecContext(ctx, "UPDATE jobs SET processed=? WHERE id=?", processed, j.ID); e != nil {
				tx.Rollback()
				return e
			}
			if e = tx.Commit(); e != nil {
				return e
			}
			offset += len(p.Items)
			if offset == expected {
				break
			}
			p, err = client.Page(ctx, l.Key, kind, offset)
			if err != nil {
				return err
			}
		}
		// Recheck the authoritative count at the end of each traversal.
		end, e := client.Page(ctx, l.Key, kind, 0)
		if e != nil {
			return e
		}
		if end.Total == nil || *end.Total != expected {
			return failure("RESPONSE", "Plex inventory changed during sync; reconciliation was skipped")
		}
	}
	libraries, err = client.Libraries(ctx)
	if err != nil {
		return err
	}
	after, err := findLibrary(libraries, l.Key)
	if err != nil {
		return err
	}
	if after.Revision != before.Revision || after.UUID != before.UUID || after.Type != before.Type {
		return failure("RESPONSE", "Plex library changed during sync; reconciliation was skipped")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	// Reconciliation and SUCCEEDED are one transaction: cancellation cannot slip between them.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var cancelled bool
	var status string
	if err = tx.QueryRowContext(ctx, "SELECT cancel_requested,status FROM jobs WHERE id=?", j.ID).Scan(&cancelled, &status); err != nil {
		return err
	}
	if cancelled || status != "RUNNING" {
		return contextCanceled
	}
	var seen int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sync_seen WHERE job_id=?", j.ID).Scan(&seen); err != nil {
		return err
	}
	if seen != total {
		return failure("RESPONSE", "Plex inventory count did not match; reconciliation was skipped")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE plex_media_items SET available=0,updated_at=? WHERE plex_library_id=? AND available=1 AND id NOT IN (SELECT media_item_id FROM sync_seen WHERE job_id=?)`, now(), l.ID, j.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE plex_libraries SET last_synced_at=?,last_full_sync_at=CASE WHEN ?='full' THEN ? ELSE last_full_sync_at END,source_revision=?,updated_at=? WHERE id=?`, now(), j.Mode, now(), after.Revision, now(), l.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE plex_servers SET status='CONNECTED',last_success_at=?,last_error='',updated_at=? WHERE id=?", now(), now(), l.ServerID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE jobs SET status='SUCCEEDED',completed_at=?,current_item='' WHERE id=?", now(), j.ID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM sync_seen WHERE job_id=?", j.ID); err != nil {
		return err
	}
	// Commit a durable, deduplicated schedule check with the authoritative scan.
	// The scheduler resolves local membership and leaves unaffected timelines intact.
	// Broad rules may span libraries, so check every enabled channel rather than
	// guessing dependencies from rule JSON. Existing active jobs/periodic maintenance
	// also re-resolve membership, protecting changes arriving during a running job.
	if _, err = tx.ExecContext(ctx, `INSERT INTO jobs(id,type,channel_id,mode,status,created_at)
 SELECT lower(hex(randomblob(16))),'SCHEDULE_REPAIR',c.id,'','QUEUED',?
 FROM channels c JOIN channel_schedule_settings ss ON ss.channel_id=c.id
 WHERE c.enabled=1 AND ss.enabled=1 AND NOT EXISTS (
 SELECT 1 FROM jobs j WHERE j.channel_id=c.id AND j.type LIKE 'SCHEDULE_%' AND j.status IN('QUEUED','RUNNING'))`, now()); err != nil {
		return err
	}
	return tx.Commit()
}
