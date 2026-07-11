package main

import (
	"fmt"
	"log"

	"buckleberry/internal/auth"
	"buckleberry/internal/database"
	"buckleberry/internal/onboarding"
	"buckleberry/internal/opds"
	"buckleberry/internal/settings"
	"buckleberry/internal/wallabag"

	"github.com/Strubbl/wallabago/v9"
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
	viper.SetDefault("BASE_URL", fmt.Sprintf("http://localhost:%s", viper.GetString("PORT")))

	dbPath := viper.GetString("DB_PATH")
	db, err := database.New(dbPath)

	if err != nil {
		log.Fatal("Failed to initialize database: ", err) // do something here?
	}

	settingsRepo := settings.NewRepository(db)

	settings, err := settingsRepo.Get()

	if err != nil {
		log.Fatal("getting settings: ", err.Error())
	}

	wallabago.SetConfig(wallabago.NewWallabagConfig(
		settings.WallabagInstanceURL,
		settings.WallabagClientID,
		settings.WallabagClientSecret,
		settings.WallabagUsername,
		settings.WallabagPassword),
	)

	authorizer := auth.NewAuthorizer(settingsRepo)
	opdsHandler := opds.NewHandler(settingsRepo, wallabag.NewClient(), viper.GetString("BASE_URL"))
	onboardingHandler := onboarding.NewHandler(settingsRepo)

	app := fiber.New()

	fiberlog.SetLevel(fiberlog.LevelDebug)

	authMiddleware := basicauth.New(basicauth.Config{
		Users:      map[string]string{},
		Authorizer: authorizer.Authorize,
	})

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Hello, world!")
	})

	app.Get("/onboarding", onboardingHandler.HandleOnboarding)
	app.Get("/opds", authMiddleware, opdsHandler.GetNavigationFeeds)
	app.Get("/opds/unread", authMiddleware, opdsHandler.GetUnreadFeed)
	app.Get("/opds/download/:id", authMiddleware, opdsHandler.GetDownload)

	port := viper.GetString("PORT")
	log.Fatal(app.Listen(":" + port))
}
