package database

import "testing"

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
	if count != 2 {
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
