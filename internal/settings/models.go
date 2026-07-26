package settings

import (
	"buckleberry/internal/linkding"
	"buckleberry/internal/wallabag"
	"time"
)

type Settings struct {
	ID          int    `db:"id"`
	Username    string `form:"username" db:"username"`
	Password    string `form:"password" db:"password"`
	UseWallabag bool   `form:"use-wallabag" db:"use_wallabag"`
	wallabag.WallabagSettings
	UseLinkding bool `form:"use-linkding" db:"use_linkding"`
	linkding.LinkdingSettings
	UseInternalEpubBuilder bool      `form:"use-internal-epub-builder" db:"use_internal_epub_builder"`
	CreatedAt              time.Time `db:"created_at"`
	UpdatedAt              time.Time `db:"updated_at"`
}
