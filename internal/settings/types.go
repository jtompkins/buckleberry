package settings

import (
	"time"
)

type WallabagSettings struct {
	UseWallabag            bool    `form:"use-wallabag" db:"use_wallabag"`
	UseInternalEpubBuilder bool    `form:"use-internal-epub-builder" db:"use_internal_epub_builder"`
	WallabagInstanceURL    *string `form:"wallabag-url" db:"wallabag_instance_url"`
	WallabagUsername       *string `form:"wallabag-username" db:"wallabag_username"`
	WallabagPassword       *string `form:"wallabag-password" db:"wallabag_password"`
	WallabagClientID       *string `form:"wallabag-client-id" db:"wallabag_client_id"`
	WallabagClientSecret   *string `form:"wallabag-client-secret" db:"wallabag_client_secret"`
}

type LinkdingSettings struct {
	UseLinkding         bool    `form:"use-linkding" db:"use_linkding"`
	LinkdingInstanceURL *string `form:"linkding-instance-url" db:"linkding_instance_url"`
	LinkdingAPIKey      *string `form:"linkding-api-key" db:"linkding_api_key"`
}

type Settings struct {
	ID       int    `db:"id"`
	Username string `form:"username" db:"username"`
	Password string `form:"password" db:"password"`
	WallabagSettings
	LinkdingSettings
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
