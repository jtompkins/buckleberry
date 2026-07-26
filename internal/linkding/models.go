package linkding

type LinkdingSettings struct {
	LinkdingInstanceURL *string `form:"linkding-instance-url" db:"linkding_instance_url"`
	LinkdingAPIKey      *string `form:"linkding-api-key" db:"linkding_api_key"`
}
