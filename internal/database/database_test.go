package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

func TestPhaseOneUpgradePreservesIdentityAndInventory(t *testing.T) {
	dir := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(dir, "afterglow.db"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, checksum TEXT NOT NULL)"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"001_foundation.sql", "002_plex_inventory.sql"} {
		b, e := migrations.ReadFile("migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(b)); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec("INSERT INTO schema_migrations VALUES(?,?)", name, fmt.Sprintf("%x", sha256.Sum256(b))); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.Exec(`INSERT INTO users VALUES('owner','owner','existing-hash','OWNER',1); INSERT INTO sessions VALUES('existing-session','owner','existing-csrf',9999999999); UPDATE app_settings SET display_name='Preserved room';`); e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for query, want := range map[string]string{"SELECT password_hash FROM users WHERE id='owner'": "existing-hash", "SELECT csrf_token FROM sessions": "existing-csrf", "SELECT display_name FROM app_settings": "Preserved room"} {
		var got string
		if e = db.QueryRow(query).Scan(&got); e != nil || got != want {
			t.Fatalf("upgrade lost foundation: %v", e)
		}
	}
	var n int
	if e = db.QueryRow("SELECT count(*) FROM channels").Scan(&n); e != nil || n != 0 {
		t.Fatal("channels migration", e)
	}
	if e = Migrate(db); e != nil {
		t.Fatal(e)
	}
}

func TestMigrationsAreIdempotentAndVerified(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = Migrate(db); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatal(count)
	}
	if _, err = db.Exec("UPDATE schema_migrations SET checksum='tampered'"); err != nil {
		t.Fatal(err)
	}
	if Migrate(db) == nil {
		t.Fatal("changed migration accepted")
	}
}
func TestNewerDatabaseRejected(t *testing.T) {
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("INSERT INTO schema_migrations VALUES('999_future.sql','future')"); err != nil {
		t.Fatal(err)
	}
	if Migrate(db) == nil {
		t.Fatal("newer database accepted")
	}
}
