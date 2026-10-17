package channels

import (
	"context"
	"database/sql"
	"regexp"
	"strconv"
	"strings"
)

func itoa(i int) string { return strconv.Itoa(i) }
func (s *Service) Groups(ctx context.Context) ([]Group, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT g.id,g.name,g.slug,g.description,g.sort_order,g.system,(SELECT count(*) FROM channels c WHERE c.group_id=g.id) FROM channel_groups g ORDER BY g.sort_order,g.name,g.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if e = rows.Scan(&g.ID, &g.Name, &g.Slug, &g.Description, &g.SortOrder, &g.System, &g.Count); e != nil {
			return nil, e
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func (s *Service) SaveGroup(ctx context.Context, id string, g Group, actor string) (Group, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" || len(g.Name) > 80 || len(g.Description) > 1000 {
		return g, invalid("name", "Enter a group name up to 80 characters and description up to 1000")
	}
	if g.Slug == "" {
		g.Slug = strings.ToLower(strings.ReplaceAll(g.Name, " ", "-"))
	}
	if len(g.Slug) > 80 || !slugPattern.MatchString(g.Slug) {
		return g, invalid("slug", "Use lowercase letters, numbers and hyphens")
	}
	create := id == ""
	if create {
		id = newID()
	}
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return g, e
	}
	defer tx.Rollback()
	if !create {
		var system bool
		if e = tx.QueryRowContext(ctx, "SELECT system FROM channel_groups WHERE id=?", id).Scan(&system); e == sql.ErrNoRows {
			return g, fail("NOT_FOUND", "Group not found")
		}
		if e != nil {
			return g, e
		}
		g.System = system
	} else {
		g.System = false
	}
	var exists bool
	if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM channel_groups WHERE slug=? AND id<>?)", g.Slug, id).Scan(&exists); e != nil {
		return g, e
	}
	if exists {
		return g, fail("CONFLICT", "Group slug is already in use")
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO channel_groups VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,slug=excluded.slug,description=excluded.description,sort_order=excluded.sort_order,updated_at=excluded.updated_at`, id, g.Name, g.Slug, g.Description, g.SortOrder, g.System, stamp(), stamp())
	if e != nil {
		return g, e
	}
	action := "CHANNEL_GROUP_UPDATED"
	if create {
		action = "CHANNEL_GROUP_CREATED"
	}
	if e = audit(ctx, tx, actor, action, id, "Channel group saved"); e != nil {
		return g, e
	}
	g.ID = id
	return g, tx.Commit()
}
func (s *Service) DeleteGroup(ctx context.Context, id string, moveTo *string, ungroup bool, actor string) error {
	tx, e := s.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var count int
	if e = tx.QueryRowContext(ctx, "SELECT count(*) FROM channels WHERE group_id=?", id).Scan(&count); e != nil {
		return e
	}
	if count > 0 && moveTo == nil && !ungroup {
		return fail("CONFLICT", "Choose a destination group or explicitly ungroup the channels")
	}
	if moveTo != nil {
		if *moveTo == id {
			return invalid("moveTo", "Choose a different group")
		}
		var exists bool
		if e = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM channel_groups WHERE id=?)", *moveTo).Scan(&exists); e != nil {
			return e
		}
		if !exists {
			return invalid("moveTo", "Destination group not found")
		}
	}
	if count > 0 {
		if _, e = tx.ExecContext(ctx, "UPDATE channels SET group_id=?,updated_at=? WHERE group_id=?", moveTo, stamp(), id); e != nil {
			return e
		}
	}
	r, e := tx.ExecContext(ctx, "DELETE FROM channel_groups WHERE id=?", id)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n == 0 {
		return fail("NOT_FOUND", "Group not found")
	}
	if e = audit(ctx, tx, actor, "CHANNEL_GROUP_DELETED", id, "Group removed; channels retained"); e != nil {
		return e
	}
	return tx.Commit()
}
