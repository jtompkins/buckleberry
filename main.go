package main

import (
	"fmt"
	"log"
	"strings"

	"buckleberry/internal/auth"
	"buckleberry/internal/database"
	"buckleberry/internal/onboarding"
	"buckleberry/internal/opds"
	"buckleberry/internal/settings"
	"buckleberry/internal/wallabag"

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

	baseURL := viper.GetString("BASE_URL")
	// Send Secure cookies whenever we're served over HTTPS. Override with
	// COOKIE_SECURE if TLS is terminated by an upstream proxy.
	viper.SetDefault("COOKIE_SECURE", strings.HasPrefix(baseURL, "https://"))

	dbPath := viper.GetString("DB_PATH")
	db, err := database.New(dbPath)

	if err != nil {
		log.Fatal("Failed to initialize database: ", err) // do something here?
	}

	settingsRepo := settings.NewRepository(db)
	wallabagClient := wallabag.NewClient()

	isOnboarded, err := settingsRepo.IsOnboarded()

	if err != nil {
		log.Fatal("Failed to find is onboarded: ", err)
	} else if isOnboarded {
		currentSettings, err := settingsRepo.Get()

		if err != nil {
			log.Fatal("getting settings: ", err.Error())
		}

		wallabagClient.Configure(currentSettings)
	}

	authHandler := auth.NewHandler(settingsRepo)
	settingsHandler := settings.NewHandler(settingsRepo, wallabagClient)
	opdsHandler := opds.NewHandler(settingsRepo, wallabagClient, baseURL)
	onboardingHandler := onboarding.NewHandler(settingsRepo, wallabagClient)

	app := fiber.New()

	app.Use(session.New(session.Config{
		CookieHTTPOnly: true,
		CookieSameSite: "Lax",
		CookieSecure:   viper.GetBool("COOKIE_SECURE"),
	}))

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

	app.Get("/onboarding", onboardingMiddleware.RedirectIfOnboarded, onboardingHandler.Onboarding)
	app.Post("/onboarding", onboardingMiddleware.RedirectIfOnboarded, onboardingHandler.HandleOnboarding)

	opds := app.Group("/opds", onboardingMiddleware.RequireOnboarded, basicAuthMiddleware)

	opds.Get("/", opdsHandler.GetNavigationFeeds)
	opds.Get("/unread", opdsHandler.GetUnreadFeed)
	opds.Get("/download/:id", opdsHandler.GetDownload)

	port := viper.GetString("PORT")
	log.Fatal(app.Listen(":" + port))
}
