package plex

import (
	"context"
	"database/sql"
	"errors"
)

const libraryColumns = `l.id,l.plex_server_id,l.plex_section_key,l.title,l.type,l.agent,l.scanner,l.language,l.uuid,l.source_revision,l.selected,l.present,l.last_synced_at,l.last_full_sync_at,(SELECT count(*) FROM plex_media_items m WHERE m.plex_library_id=l.id AND m.available=1)`

func scanLibrary(row scanner) (Library, error) {
	var l Library
	err := row.Scan(&l.ID, &l.ServerID, &l.Key, &l.Title, &l.Type, &l.Agent, &l.Scanner, &l.Language, &l.UUID, &l.Revision, &l.Selected, &l.Present, &l.LastSynced, &l.LastFullSync, &l.ItemCount)
	l.Supported = l.Type == "movie" || l.Type == "show"
	return l, err
}
func (s *Service) Libraries(ctx context.Context, serverID string) ([]Library, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+libraryColumns+" FROM plex_libraries l WHERE l.plex_server_id=? ORDER BY l.title", serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Library{}
	for rows.Next() {
		l, e := scanLibrary(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
func (s *Service) Library(ctx context.Context, id string) (Library, error) {
	l, err := scanLibrary(s.db.QueryRowContext(ctx, "SELECT "+libraryColumns+" FROM plex_libraries l WHERE l.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return l, failure("NOT_FOUND", "Library not found")
	}
	return l, err
}
func (s *Service) SelectLibrary(ctx context.Context, id string, selected bool, actor string) error {
	l, err := s.Library(ctx, id)
	if err != nil {
		return err
	}
	if selected && (!l.Supported || !l.Present) {
		return failure("INVALID", "Only present movie and TV libraries can be selected in Phase 1")
	}
	server, err := s.Server(ctx, l.ServerID)
	if err != nil {
		return err
	}
	if selected && server.Disabled {
		return failure("INVALID", "Reconnect the Plex server before selecting libraries")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active int
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM jobs WHERE library_id=? AND status IN ('QUEUED','RUNNING')", id).Scan(&active); err != nil {
		return err
	}
	if active > 0 {
		return failure("CONFLICT", "Cancel or finish this library's sync before changing its selection")
	}
	if _, err = tx.ExecContext(ctx, "UPDATE plex_libraries SET selected=?,updated_at=? WHERE id=?", selected, now(), id); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "plex.library.selection"); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) discover(ctx context.Context, id string) error {
	client, server, err := s.provider(ctx, id)
	if err != nil {
		return err
	}
	identity, err := client.Identity(ctx)
	if err != nil {
		return err
	}
	if identity.MachineIdentifier != server.MachineIdentifier {
		return failure("IDENTITY", "Plex identity changed; library discovery stopped")
	}
	libraries, err := client.Libraries(ctx)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "UPDATE plex_libraries SET present=0 WHERE plex_server_id=?", id); err != nil {
		return err
	}
	for _, l := range libraries {
		_, err = tx.ExecContext(ctx, `INSERT INTO plex_libraries(id,plex_server_id,plex_section_key,title,type,agent,scanner,language,uuid,source_revision,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(plex_server_id,plex_section_key) DO UPDATE SET title=excluded.title,type=excluded.type,agent=excluded.agent,scanner=excluded.scanner,language=excluded.language,uuid=excluded.uuid,source_revision=excluded.source_revision,present=1,updated_at=excluded.updated_at`, newID(), id, l.Key, l.Title, l.Type, l.Agent, l.Scanner, l.Language, l.UUID, l.Revision, now(), now())
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
