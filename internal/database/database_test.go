package database

import (
	"path/filepath"
	"testing"
)

func TestMigrateIsIdempotent(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if ok, err := IsSchemaUpToDate(db); err != nil || ok {
		t.Fatalf("fresh database: got up to date=%v err=%v", ok, err)
	}
	for range 2 {
		if err := Migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	if ok, err := IsSchemaUpToDate(db); err != nil || !ok {
		t.Fatalf("migrated database: got up to date=%v err=%v", ok, err)
	}

	var journalMode string
	var foreignKeys int
	db.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode)
	db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys)
	if journalMode != "wal" || foreignKeys != 1 {
		t.Fatalf("pragmas: journal_mode=%q foreign_keys=%d", journalMode, foreignKeys)
	}

	var lowered string
	if err := db.QueryRow(`SELECT lower('ÉCOLE')`).Scan(&lowered); err != nil || lowered != "école" {
		t.Fatalf("lower: got %q err=%v", lowered, err)
	}
}
