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

func (r *Repository) IsOnboarded() (bool, error) {
	settings, err := r.Get()

	if err != nil {
		return false, fmt.Errorf("is onboarded: fetching settings: %w", err)
	}

	return settings != nil, nil
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

func (r *Repository) Create(settings *Settings) (*Settings, error) {
	_, err := r.db.NamedExec(
		"INSERT INTO settings (username, password, wallabag_instance_url, wallabag_username, wallabag_password, wallabag_client_id, wallabag_client_secret) VALUES (:username, :password, :wallabag_instance_url, :wallabag_username, :wallabag_password, :wallabag_client_id, :wallabag_client_secret)",
		settings,
	)

	if err != nil {
		return nil, fmt.Errorf("saving settings: %w", err)
	}

	return r.Get()
}

func (r *Repository) UpdateWallabagSettings(settings *Settings) (*Settings, error) {
	_, err := r.db.NamedExec(
		"UPDATE settings SET wallabag_instance_url = :wallabag_instance_url, wallabag_username = :wallabag_username, wallabag_password = :wallabag_password, wallabag_client_id = :wallabag_client_id, wallabag_client_secret = :wallabag_client_secret",
		settings,
	)

	if err != nil {
		return nil, fmt.Errorf("updating settings: %w", err)
	}

	return r.Get()
}
