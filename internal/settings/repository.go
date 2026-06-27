package settings

import (
	"database/sql"
	"errors"

	"buckleberry/internal/database"
)

type Repository struct {
	db *database.DB
}

func NewRepository(db *database.DB) *Repository {
	return &Repository{
		db,
	}
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
