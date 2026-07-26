package wallabag

type WallabagSettings struct {
	WallabagInstanceURL  *string `form:"wallabag-url" db:"wallabag_instance_url"`
	WallabagUsername     *string `form:"wallabag-username" db:"wallabag_username"`
	WallabagPassword     *string `form:"wallabag-password" db:"wallabag_password"`
	WallabagClientID     *string `form:"wallabag-client-id" db:"wallabag_client_id"`
	WallabagClientSecret *string `form:"wallabag-client-secret" db:"wallabag_client_secret"`
}
