package database

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()

	db, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	})

	return db
}

func TestNewAppliesMigrations(t *testing.T) {
	db := newTestDB(t)

	// The settings table from migration 1 must exist and be usable.
	if _, err := db.Exec("INSERT INTO settings (username) VALUES ('reader')"); err != nil {
		t.Fatalf("settings table not usable after migrate: %v", err)
	}

	var count int
	if err := db.Get(&count, "SELECT COUNT(*) FROM schema_migrations"); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if count != len(migrations) {
		t.Errorf("recorded migrations = %d, want %d", count, len(migrations))
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	first, err := New(path)
	if err != nil {
		t.Fatalf("first New() error = %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close first: %v", err)
	}

	// Re-opening the same database must not re-run or duplicate migrations.
	second, err := New(path)
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}
	defer second.Close()

	var count int
	if err := second.Get(&count, "SELECT COUNT(*) FROM schema_migrations"); err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	if count != len(migrations) {
		t.Errorf("recorded migrations after reopen = %d, want %d", count, len(migrations))
	}
}

func TestNewFailsOnBadPath(t *testing.T) {
	// A path whose parent directory doesn't exist can't be opened/pinged.
	_, err := New(filepath.Join(t.TempDir(), "no-such-dir", "test.db"))
	if err == nil {
		t.Fatal("New() with unwritable path = nil error, want failure")
	}
}
