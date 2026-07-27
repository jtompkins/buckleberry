package settings

import (
	"buckleberry/internal/linkding"
	"buckleberry/internal/wallabag"
	"time"
)

type Settings struct {
	ID       int    `db:"id"`
	Username string `form:"username" db:"username"`
	Password string `form:"password" db:"password"`
	wallabag.WallabagSettings
	linkding.LinkdingSettings
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
