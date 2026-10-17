package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

func TestPhaseZeroDatabaseUpgrade(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "afterglow.db"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := migrations.ReadFile("migrations/001_foundation.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(source)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE schema_migrations(version TEXT PRIMARY KEY,checksum TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO schema_migrations VALUES(?,?)", "001_foundation.sql", fmt.Sprintf("%x", sha256.Sum256(source))); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users VALUES('owner','owner','preserved-hash','OWNER',1); INSERT INTO sessions VALUES('hash','owner','csrf',9999999999); UPDATE app_settings SET display_name='Preserved',timezone='America/New_York'`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	db, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var name, hash, session string
	if err = db.QueryRow("SELECT display_name FROM app_settings").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT password_hash FROM users").Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT csrf_token FROM sessions").Scan(&session); err != nil {
		t.Fatal(err)
	}
	if name != "Preserved" || hash != "preserved-hash" || session != "csrf" {
		t.Fatal("Phase 0 data changed")
	}
	for _, table := range []string{"plex_servers", "plex_libraries", "plex_media_items", "jobs", "artwork_cache"} {
		if _, err = db.Exec("SELECT 1 FROM " + table); err != nil {
			t.Fatal(err)
		}
	}
}
