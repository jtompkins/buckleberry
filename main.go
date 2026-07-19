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
	"github.com/gofiber/fiber/v3/middleware/session"
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

	isOnboarded, err := settingsRepo.IsOnboarded()

	if err != nil {
		log.Fatal("Failed to find is onboarded: ", err)
	} else if isOnboarded {
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
	}

	authHandler := auth.NewHandler(settingsRepo)
	settingsHandler := settings.NewHandler(settingsRepo)
	opdsHandler := opds.NewHandler(settingsRepo, wallabag.NewClient(), viper.GetString("BASE_URL"))
	onboardingHandler := onboarding.NewHandler(settingsRepo)

	app := fiber.New()

	app.Use(session.New())

	fiberlog.SetLevel(fiberlog.LevelDebug)

	authorizer := auth.NewAuthorizer(settingsRepo)
	basicAuthMiddleware := basicauth.New(basicauth.Config{
		Users:      map[string]string{},
		Authorizer: authorizer.Authorize,
	})

	sessionAuthMiddleware := auth.NewMiddleware(settingsRepo)
	onboardingMiddleware := onboarding.NewMiddleware(settingsRepo)

	app.Get("/", onboardingMiddleware.RequireOnboarded, authHandler.Login)
	app.Post("/login", onboardingMiddleware.RequireOnboarded, authHandler.HandleLogin)

	app.Get("/settings", onboardingMiddleware.RequireOnboarded, sessionAuthMiddleware.RequireAuth, settingsHandler.Settings)
	app.Post("/settings", onboardingMiddleware.RequireOnboarded, sessionAuthMiddleware.RequireAuth, settingsHandler.UpdateSettings)

	app.Get("/onboarding", onboardingHandler.HandleOnboarding)
	app.Post("/onboarding", onboardingHandler.FinishOnboarding)

	opds := app.Group("/opds", onboardingMiddleware.RequireOnboarded, basicAuthMiddleware)

	opds.Get("/", opdsHandler.GetNavigationFeeds)
	opds.Get("/unread", opdsHandler.GetUnreadFeed)
	opds.Get("/download/:id", opdsHandler.GetDownload)

	port := viper.GetString("PORT")
	log.Fatal(app.Listen(":" + port))
}
