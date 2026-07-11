package settings

import (
	"database/sql"
	"errors"
	"fmt"

	"buckleberry/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{db}
}

func (r *Repository) Get() (*Settings, error) {
	var settings Settings

	err := r.db.Get(&settings, "SELECT * FROM settings LIMIT 1")
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		return nil, err
	}

	return &settings, nil
}

func (r *Repository) Create(settings *Settings) (bool, error) {
	// Use SQLx's NamedExec to automatically map struct fields to query parameters
	_, err := r.db.NamedExec(
		"INSERT INTO settings (username, password, wallabag_instance_url, wallabag_username, wallabag_password, wallabag_client_id, wallabag_client_secret) VALUES (:username, :password, :wallabag_instance_url, :wallabag_username, :wallabag_password, :wallabag_client_id, :wallabag_client_secret)",
		settings,
	)

	if err != nil {
		return false, fmt.Errorf("saving settings: %w", err)
	}

	return true, nil
}

func (r *Repository) Update(settings *Settings) (*Settings, error) {
	// Use SQLx's NamedExec to automatically map struct fields to query parameters
	_, err := r.db.NamedExec(
		"UPDATE settings SET password = :password, wallabag_instance_url = :wallabag_instance_url, wallabag_username = :wallabag_username, wallabag_password = :wallabag_password, wallabag_client_id = :wallabag_client_id, wallabag_client_secret = :wallabag_client_secret",
		settings,
	)

	if err != nil {
		return nil, fmt.Errorf("updating settings: %w", err)
	}

	return r.Get()
}
