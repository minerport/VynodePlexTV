package database

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(dir string) (*sql.DB, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "afterglow.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return nil, err
	}
	if err = Migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, checksum TEXT NOT NULL)`); err != nil {
		return err
	}
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, f := range files {
		known[f.Name()] = true
	}
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return err
	}
	for rows.Next() {
		var version string
		if err = rows.Scan(&version); err != nil {
			rows.Close()
			return err
		}
		if !known[version] {
			rows.Close()
			return fmt.Errorf("database contains unknown migration %s", version)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		contents, err := migrations.ReadFile("migrations/" + f.Name())
		if err != nil {
			return err
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
		var applied string
		err = db.QueryRow("SELECT checksum FROM schema_migrations WHERE version=?", f.Name()).Scan(&applied)
		if err == nil {
			if applied != checksum {
				return fmt.Errorf("migration checksum mismatch: %s", f.Name())
			}
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		tx, err := db.Begin()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(string(contents)); err == nil {
			_, err = tx.Exec("INSERT INTO schema_migrations VALUES(?,?)", f.Name(), checksum)
		}
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}
