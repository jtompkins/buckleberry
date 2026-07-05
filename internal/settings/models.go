package settings

import (
	"time"
)

type Settings struct {
	ID                   int       `json:"id" db:"id"`
	Username             string    `json:"username" db:"username"`
	Password             string    `json:"password" db:"password"`
	WallabagInstanceURL  string    `json:"wallabag_instance_url" db:"wallabag_instance_url"`
	WallabagUsername     string    `json:"wallabag_username" db:"wallabag_username"`
	WallabagPassword     string    `json:"wallabag_password" db:"wallabag_password"`
	WallabagClientID     string    `json:"wallabag_client_id" db:"wallabag_client_id"`
	WallabagClientSecret string    `json:"wallabag_client_secret" db:"wallabag_client_secret"`
	SyncInterval         int       `json:"sync_interval" db:"sync_interval"`
	LastSync             time.Time `json:"last_sync" db:"last_sync"`
	CreatedAt            time.Time `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time `json:"updated_at" db:"updated_at"`
}
