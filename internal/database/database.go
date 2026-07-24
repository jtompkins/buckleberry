package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sqlx.DB
}

type migration struct {
	version int
	query   string
}

var migrations = []migration{
	{
		version: 1,
		query: `
			CREATE TABLE IF NOT EXISTS settings (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT,
				password TEXT,
				wallabag_instance_url TEXT,
				wallabag_username TEXT,
				wallabag_password TEXT,
				wallabag_client_id TEXT,
				wallabag_client_secret TEXT,
				sync_interval INTEGER DEFAULT 15,
				last_sync DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
			);
		`,
	},
	{
		version: 2,
		query: `
			ALTER TABLE settings ADD COLUMN use_internal_epub_builder INTEGER DEFAULT FALSE;
			ALTER TABLE settings DROP COLUMN sync_interval;
			ALTER TABLE settings DROP COLUMN last_sync;	
		`,
	},
}

func New(dbPath string) (*DB, error) {
	ctx := context.Background()

	sqlDB, err := sqlx.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	db := &DB{sqlDB}

	if err := db.migrate(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) migrate(ctx context.Context) error {
	tx, err := db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migrations: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY
		)
	`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	for _, migration := range migrations {
		var applied bool
		if err := tx.GetContext(ctx, &applied, `
			SELECT EXISTS(
				SELECT 1 FROM schema_migrations WHERE version = ?
			)
		`, migration.version); err != nil {
			return fmt.Errorf("check migration %d: %w", migration.version, err)
		}

		if applied {
			continue
		}

		if _, err := tx.ExecContext(ctx, migration.query); err != nil {
			return fmt.Errorf("apply migration %d: %w", migration.version, err)
		}

		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO schema_migrations (version) VALUES (?)",
			migration.version,
		); err != nil {
			return fmt.Errorf("record migration %d: %w", migration.version, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}

	return nil
}
