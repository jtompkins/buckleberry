package settings

import (
	"buckleberry/internal/wallabag"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

type settingsRepository interface {
	Get() (*Settings, error)
	UpdateWallabagSettings(*Settings) (*Settings, error)
}

type wallabagClient interface {
	Ping() error
	Configure(*wallabag.WallabagSettings)
}

type linkdingPinger interface {
	Ping() error
}

func NewHandler(settingsRepo settingsRepository, wallabag wallabagClient, linkding linkdingPinger) *Handler {
	return &Handler{
		settingsRepo:   settingsRepo,
		wallabagClient: wallabag,
		linkdingClient: linkding,
	}
}

type Handler struct {
	settingsRepo   settingsRepository
	wallabagClient wallabagClient
	linkdingClient linkdingPinger
}

func (h *Handler) Settings(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		log.Error("fetch settings: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't load settings")
	}

	wallabagConnected := h.wallabagClient.Ping() == nil
	linkdingConnected := h.linkdingClient.Ping() == nil

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

	_, err := h.settingsRepo.UpdateWallabagSettings(&formSettings)

	if err != nil {
		log.Error("update settings: ", err)
		return c.Redirect().With("error", "Couldn't save settings").To("/settings")
	}

	h.wallabagClient.Configure(&formSettings.WallabagSettings)

	return c.Redirect().With("success", "Settings updated!").To("/settings")
}
