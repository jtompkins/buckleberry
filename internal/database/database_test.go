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

func TestMigration2AddsInternalEpubBuilderColumnAndDropsSyncColumns(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.Exec(
		"INSERT INTO settings (username, use_internal_epub_builder) VALUES (?, ?)",
		"reader", true,
	); err != nil {
		t.Fatalf("insert using use_internal_epub_builder column: %v", err)
	}

	var got bool
	if err := db.Get(&got, "SELECT use_internal_epub_builder FROM settings WHERE username = ?", "reader"); err != nil {
		t.Fatalf("read use_internal_epub_builder: %v", err)
	}
	if !got {
		t.Errorf("use_internal_epub_builder = %v, want true", got)
	}

	if _, err := db.Exec("SELECT sync_interval FROM settings"); err == nil {
		t.Error("sync_interval column still exists, want it dropped by migration 2")
	}
	if _, err := db.Exec("SELECT last_sync FROM settings"); err == nil {
		t.Error("last_sync column still exists, want it dropped by migration 2")
	}
}

func TestMigration3AddsLinkdingAndSourceToggleColumns(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.Exec(
		"INSERT INTO settings (username, use_wallabag, use_linkding, linkding_instance_url, linkding_api_key) VALUES (?, ?, ?, ?, ?)",
		"reader", false, true, "https://linkding.example.com", "linkding-key",
	); err != nil {
		t.Fatalf("insert using migration 3 columns: %v", err)
	}

	var got struct {
		UseWallabag bool   `db:"use_wallabag"`
		UseLinkding bool   `db:"use_linkding"`
		URL         string `db:"linkding_instance_url"`
		APIKey      string `db:"linkding_api_key"`
	}
	if err := db.Get(&got,
		"SELECT use_wallabag, use_linkding, linkding_instance_url, linkding_api_key FROM settings WHERE username = ?",
		"reader",
	); err != nil {
		t.Fatalf("read migration 3 columns: %v", err)
	}

	if got.UseWallabag || !got.UseLinkding {
		t.Errorf("use_wallabag/use_linkding = %v/%v, want false/true", got.UseWallabag, got.UseLinkding)
	}
	if got.URL != "https://linkding.example.com" || got.APIKey != "linkding-key" {
		t.Errorf("linkding settings = %q/%q, want the inserted values", got.URL, got.APIKey)
	}
}

// Rows that predate migration 3 keep working with Wallabag on, which is the
// only source those installs could have been using.
func TestMigration3DefaultsPreserveExistingWallabagUsers(t *testing.T) {
	db := newTestDB(t)

	if _, err := db.Exec("INSERT INTO settings (username) VALUES (?)", "existing"); err != nil {
		t.Fatalf("insert row without the new columns: %v", err)
	}

	var useWallabag, useLinkding bool
	if err := db.Get(&useWallabag, "SELECT use_wallabag FROM settings WHERE username = ?", "existing"); err != nil {
		t.Fatalf("read use_wallabag: %v", err)
	}
	if err := db.Get(&useLinkding, "SELECT use_linkding FROM settings WHERE username = ?", "existing"); err != nil {
		t.Fatalf("read use_linkding: %v", err)
	}

	if !useWallabag {
		t.Error("use_wallabag defaulted to false, want true so existing installs keep their feed")
	}
	if useLinkding {
		t.Error("use_linkding defaulted to true, want false")
	}
}

func TestNewFailsOnBadPath(t *testing.T) {
	// A path whose parent directory doesn't exist can't be opened/pinged.
	_, err := New(filepath.Join(t.TempDir(), "no-such-dir", "test.db"))
	if err == nil {
		t.Fatal("New() with unwritable path = nil error, want failure")
	}
}
