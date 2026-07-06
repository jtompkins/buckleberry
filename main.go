package main

import (
	"log"

	"buckleberry/internal/auth"
	"buckleberry/internal/database"
	"buckleberry/internal/opds"
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	fiberlog "github.com/gofiber/fiber/v3/log"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile(".env")
	_ = viper.ReadInConfig()
	viper.AutomaticEnv()

	viper.SetDefault("DB_PATH", "./buckleberry.db")
	viper.SetDefault("PORT", "8080")

	dbPath := viper.GetString("DB_PATH")
	db, err := database.New(dbPath)

	if err != nil {
		log.Fatal("Failed to initialize database: ", err) // do something here?
	}

	settingsRepo := settings.NewRepository(db)
	middleware := auth.NewMiddleware(settingsRepo)

	opdsHandler := opds.NewHandler(settingsRepo)

	app := fiber.New()

	fiberlog.SetLevel(fiberlog.LevelDebug)

	authMiddleware := basicauth.New(basicauth.Config{
		Users:      middleware.GetUsers(),
		Authorizer: middleware.Authorizer,
	})

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, world!")
	})

	app.Get("/opds", authMiddleware, opdsHandler.GetNavigationFeeds)
	app.Get("/opds/unread", authMiddleware, opdsHandler.GetUnreadFeed)
	app.Get("/opds/download/:id", authMiddleware, opdsHandler.GetDownload)

	port := viper.GetString("PORT")
	log.Fatal(app.Listen(":" + port))
}
