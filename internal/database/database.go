package database

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sqlx.DB
}

func New(dbPath string) (*DB, error) {
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	database := &DB{db}
	if err := database.createTables(); err != nil {
		return nil, err
	}

	if err := database.updateTables(); err != nil {
		return nil, err
	}

	return database, nil
}

func (db *DB) createTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT,
		password TEXT,
		wallabag_instance_url TEXT,
		wallabag_username TEXT,
		wallabag_password TEXT,
		wallabag_client_id TEXT,
		wallabag_client_secret TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS articles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`

	db.MustExec(query)

	return nil
}

func (db *DB) updateTables() error {
	migrations := []string{}

	for _, migration := range migrations {
		_, err := db.Exec(migration)
		if err != nil {
			// SQLite will return an error if column already exists
			// This is expected behavior for migrations
			continue
		}
	}

	return nil
}
