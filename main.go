package main

import (
	"log"

	"buckleberry/internal/auth"
	"buckleberry/internal/database"
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	dbPath := viper.GetString("DB_PATH")
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database: ", err) // do something here?
	}

	settingsRepo := settings.NewRepository(db)

	app := fiber.New()

	middleware := auth.NewMiddleware(settingsRepo)

	app.Use([]string{"/", "/opds"}, basicauth.New(basicauth.Config{
		Users:      middleware.GetUsers(),
		Authorizer: middleware.Authorizer,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, world!")
	})

	log.Fatal(app.Listen(":3000"))
}
