package database

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
)

func TestPhaseSixUpgradePreservesViewerPreferences(t *testing.T) {
	dir := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(dir, "afterglow.db"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE schema_migrations(version TEXT PRIMARY KEY,checksum TEXT NOT NULL)"); e != nil {
		t.Fatal(e)
	}
	files, e := migrations.ReadDir("migrations")
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range files {
		if f.Name() >= "009" {
			break
		}
		b, e := migrations.ReadFile("migrations/" + f.Name())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec(string(b)); e != nil {
			t.Fatal(e)
		}
		if _, e = db.Exec("INSERT INTO schema_migrations VALUES(?,?)", f.Name(), fmt.Sprintf("%x", sha256.Sum256(b))); e != nil {
			t.Fatal(e)
		}
	}
	_, e = db.Exec(`INSERT INTO users VALUES('u','fixture','unchanged','OWNER',1);INSERT INTO sessions VALUES('fixture-session','u','fixture-csrf',9999999999);INSERT INTO channels(id,channel_number,name,channel_type,created_at,updated_at) VALUES('c','101','Fixture','STATIC',1,1);INSERT INTO user_channel_favorites VALUES('u','c',1);INSERT INTO viewer_preferences VALUES('u','c',1)`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var mode, last string
	var favorites int
	if e = db.QueryRow("SELECT order_mode,last_channel_id FROM viewer_preferences WHERE user_id='u'").Scan(&mode, &last); e != nil {
		t.Fatal(e)
	}
	if e = db.QueryRow("SELECT count(*) FROM user_channel_favorites WHERE user_id='u'").Scan(&favorites); e != nil {
		t.Fatal(e)
	}
	if mode != "NUMBER" || last != "c" || favorites != 1 {
		t.Fatal(mode, last, favorites)
	}
}

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
	if count != 9 {
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

func TestPhaseTwoUpgradePreservesChannelsAndDefaults(t *testing.T) {
	dir := t.TempDir()
	db, e := sql.Open("sqlite", filepath.Join(dir, "afterglow.db"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE TABLE schema_migrations(version TEXT PRIMARY KEY, checksum TEXT NOT NULL)"); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"001_foundation.sql", "002_plex_inventory.sql", "003_virtual_channels.sql"} {
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
	_, e = db.Exec(`INSERT INTO users VALUES('owner','owner','preserved-hash','OWNER',1);INSERT INTO sessions VALUES('session','owner','csrf',9999999999);UPDATE app_settings SET timezone='America/New_York';INSERT INTO channels(id,channel_number,name,channel_type,created_at,updated_at) VALUES('channel','201','Preserved show','SHOW',1792238400,1792238400);INSERT INTO channel_sources VALUES('channel','SHOW','{"type":"SHOW"}',1,1);`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	db, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	var mode, zone, seed string
	var anchor int64
	if e = db.QueryRow("SELECT mode,timezone,seed,anchor_ms FROM channel_schedule_settings WHERE channel_id='channel'").Scan(&mode, &zone, &seed, &anchor); e != nil {
		t.Fatal(e)
	}
	if mode != "SEQUENTIAL" || zone != "America/New_York" || len(seed) != 32 || anchor != (1792238400-604800)*1000 {
		t.Fatal("defaults", mode, zone, anchor)
	}
	var source, password, csrf string
	db.QueryRow("SELECT config_json FROM channel_sources").Scan(&source)
	db.QueryRow("SELECT password_hash FROM users").Scan(&password)
	db.QueryRow("SELECT csrf_token FROM sessions").Scan(&csrf)
	if source != `{"type":"SHOW"}` || password != "preserved-hash" || csrf != "csrf" {
		t.Fatal("upgrade changed baseline")
	}
	if e = Migrate(db); e != nil {
		t.Fatal(e)
	}
	var seedAgain string
	db.QueryRow("SELECT seed FROM channel_schedule_settings").Scan(&seedAgain)
	if seedAgain != seed {
		t.Fatal("migration changed seed")
	}
}
