package main

import (
	"fmt"
	"log"
	"strings"

	"buckleberry/internal/adapter"
	"buckleberry/internal/auth"
	"buckleberry/internal/home"
	"buckleberry/internal/onboarding"
	"buckleberry/internal/opds"
	"buckleberry/internal/settings"
	"buckleberry/pkg/database"

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

	authHandler := auth.NewHandler(settingsRepo)
	homeHandler := home.NewHandler(settingsRepo)
	opdsHandler := opds.NewHandler(settingsRepo, baseURL)
	onboardingHandler := onboarding.NewHandler(settingsRepo)

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
	adapterInjectionMiddleware := adapter.NewMiddleware(settingsRepo)

	app.Get("/login", onboardingMiddleware.RequireOnboarded, authHandler.Login)
	app.Post("/login", onboardingMiddleware.RequireOnboarded, authHandler.HandleLogin)

	app.Get("/", onboardingMiddleware.RequireOnboarded, sessionAuthMiddleware.RequireAuth, adapterInjectionMiddleware.InjectAdapters, homeHandler.Home)
	app.Post("/settings", onboardingMiddleware.RequireOnboarded, sessionAuthMiddleware.RequireAuth, homeHandler.UpdateSettings)

	app.Get("/onboarding", onboardingMiddleware.RedirectIfOnboarded, onboardingHandler.Onboarding)
	app.Post("/onboarding", onboardingMiddleware.RedirectIfOnboarded, onboardingHandler.HandleOnboarding)

	opds := app.Group("/opds", onboardingMiddleware.RequireOnboarded, basicAuthMiddleware)

	opds.Get("/", adapterInjectionMiddleware.InjectAdapters, opdsHandler.GetTopLevelNavigationFeeds)
	opds.Get("/:adapter/", adapterInjectionMiddleware.InjectAdapters, opdsHandler.GetNavigationFeedsForAdapter)
	opds.Get("/:adapter/:feed", adapterInjectionMiddleware.InjectAdapters, opdsHandler.GetAcquisitionFeedForAdapter)
	opds.Get("/:adapter/:feed/:id", adapterInjectionMiddleware.InjectAdapters, opdsHandler.DownloadArticle)

	port := viper.GetString("PORT")
	log.Fatal(app.Listen(":" + port))
}
