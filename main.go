package main

import (
	"fmt"
	"log"
	"strings"

	"buckleberry/internal/auth"
	"buckleberry/internal/database"
	"buckleberry/internal/epub"
	"buckleberry/internal/linkding"
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
	linkdingClient := linkding.NewClient()
	articleFetcher := epub.ArticleFetcher{}
	epubBuilder := epub.EPUBBuilder{}

	isOnboarded, err := settingsRepo.IsOnboarded()

	if err != nil {
		log.Fatal("Failed to find is onboarded: ", err)
	} else if isOnboarded {
		currentSettings, err := settingsRepo.Get()

		if err != nil {
			log.Fatal("getting settings: ", err.Error())
		}

		if currentSettings.UseWallabag {
			wallabagClient.Configure(&currentSettings.WallabagSettings)
		}

		if currentSettings.UseLinkding {
			linkdingClient.Configure(&currentSettings.LinkdingSettings)
		}
	}

	authHandler := auth.NewHandler(settingsRepo)
	settingsHandler := settings.NewHandler(settingsRepo, wallabagClient, linkdingClient)
	opdsHandler := opds.NewHandler(settingsRepo, wallabagClient, linkdingClient, articleFetcher, epubBuilder, baseURL)
	onboardingHandler := onboarding.NewHandler(settingsRepo, wallabagClient, linkdingClient)

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
	opds.Get("/wallabag/", opdsHandler.RequireWallabag, opdsHandler.GetUnreadWallabagFeed)
	opds.Get("/wallabag/:id", opdsHandler.RequireWallabag, opdsHandler.GetWallabagDownload)
	opds.Get("/linkding/", opdsHandler.RequireLinkding, opdsHandler.GetUnreadLinkdingFeed)
	opds.Get("/linkding/:id", opdsHandler.RequireLinkding, opdsHandler.GetLinkdingDownload)

	app.Get("/healthcheck", onboardingMiddleware.RequireOnboarded, func(c fiber.Ctx) error {
		settings, err := settingsRepo.Get()

		if err != nil {
			fiberlog.Error("Health check: unable to fetch settings: ", err)
			return c.SendStatus(fiber.StatusInternalServerError)
		}

		if settings.UseWallabag {
			if err = wallabagClient.Ping(); err != nil {
				return c.SendStatus(fiber.StatusFailedDependency)
			}
		}

		if settings.UseLinkding {
			if err = linkdingClient.Ping(); err != nil {
				return c.SendStatus(fiber.StatusFailedDependency)
			}
		}

		return c.SendStatus(fiber.StatusOK)
	})

	port := viper.GetString("PORT")
	log.Fatal(app.Listen(":" + port))
}
