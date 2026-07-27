package settings

import (
	"buckleberry/internal/linkding"
	"buckleberry/internal/wallabag"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

type settingsRepository interface {
	Get() (*Settings, error)
	Update(*Settings) (*Settings, error)
}

type wallabagClient interface {
	Ping() error
	Configure(*wallabag.WallabagSettings)
}

type linkdingClient interface {
	Ping() error
	Configure(*linkding.LinkdingSettings)
}

func NewHandler(settingsRepo settingsRepository, wallabag wallabagClient, linkding linkdingClient) *Handler {
	return &Handler{
		settingsRepo:   settingsRepo,
		wallabagClient: wallabag,
		linkdingClient: linkding,
	}
}

type Handler struct {
	settingsRepo   settingsRepository
	wallabagClient wallabagClient
	linkdingClient linkdingClient
}

func (h *Handler) Settings(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		log.Error("fetch settings: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't load settings")
	}

	wallabagConnected := settings.UseWallabag && h.wallabagClient.Ping() == nil
	linkdingConnected := settings.UseLinkding && h.linkdingClient.Ping() == nil

	c.Set("Content-Type", "text/html")
	settingsView := SettingsView(settings, c.Redirect().Messages(), wallabagConnected, linkdingConnected)
	return settingsView.Render(c.Context(), c.Response().BodyWriter())
}

func (h *Handler) UpdateSettings(c fiber.Ctx) error {
	var formSettings Settings

	if err := c.Bind().Form(&formSettings); err != nil {
		log.Error("bind settings form: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form submission")
	}

	_, err := h.settingsRepo.Update(&formSettings)

	if err != nil {
		log.Error("update settings: ", err)
		return c.Redirect().With("error", "Couldn't save settings").To("/settings")
	}

	if formSettings.UseWallabag {
		h.wallabagClient.Configure(&formSettings.WallabagSettings)
	}

	if formSettings.UseLinkding {
		h.linkdingClient.Configure(&formSettings.LinkdingSettings)
	}
	return c.Redirect().With("success", "Settings updated!").To("/settings")
}
