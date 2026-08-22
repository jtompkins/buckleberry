package adapter

import (
	"buckleberry/internal/settings"
	"buckleberry/pkg/epub"
	"buckleberry/pkg/linkding"
	"buckleberry/pkg/wallabag"

	"github.com/gofiber/fiber/v3"
)

type settingsFetcher interface {
	Get() (*settings.Settings, error)
}

type Middleware struct {
	settingsRepository settingsFetcher
}

func (m *Middleware) InjectAdapters(c fiber.Ctx) error {
	settings, err := m.settingsRepository.Get()

	if err != nil {
		return err
	}

	adapters := map[string]Adapter{}

	if settings.UseWallabag {
		wallabagClient := wallabag.NewClient()
		wallabagClient.Configure(wallabag.WallabagConfig{
			WallabagInstanceURL:  *settings.WallabagInstanceURL,
			WallabagClientID:     *settings.WallabagClientID,
			WallabagClientSecret: *settings.WallabagClientSecret,
			WallabagUsername:     *settings.WallabagUsername,
			WallabagPassword:     *settings.WallabagPassword,
		})

		wallabagAdapter := NewWallabagAdapter(wallabagClient)

		adapters[wallabagAdapter.Name()] = wallabagAdapter
	}

	if settings.UseLinkding {
		linkdingClient := linkding.NewClient()
		linkdingClient.Configure(*settings.LinkdingInstanceURL, *settings.LinkdingAPIKey)

		articleFetcher := epub.ArticleFetcher{}
		epubBuilder := epub.EPUBBuilder{}

		linkdingAdapter := NewLinkdingAdapter(linkdingClient, articleFetcher, epubBuilder)

		adapters[linkdingAdapter.Name()] = linkdingAdapter
	}

	c.Locals("adapters", adapters)

	return c.Next()
}

func NewMiddleware(settingsRepo settingsFetcher) *Middleware {
	return &Middleware{
		settingsRepository: settingsRepo,
	}
}
