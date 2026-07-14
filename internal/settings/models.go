package settings

import (
	"time"
)

type Settings struct {
	ID                   int       `db:"id"`
	Username             string    `form:"username" db:"username"`
	Password             string    `form:"password" db:"password"`
	WallabagInstanceURL  string    `form:"wallabag-url" db:"wallabag_instance_url"`
	WallabagUsername     string    `form:"wallabag-username" db:"wallabag_username"`
	WallabagPassword     string    `form:"wallabag-password" db:"wallabag_password"`
	WallabagClientID     string    `form:"wallabag-client-id" db:"wallabag_client_id"`
	WallabagClientSecret string    `form:"wallabag-client-secret" db:"wallabag_client_secret"`
	SyncInterval         int       `db:"sync_interval"`
	LastSync             time.Time `db:"last_sync"`
	CreatedAt            time.Time `db:"created_at"`
	UpdatedAt            time.Time `db:"updated_at"`
}
