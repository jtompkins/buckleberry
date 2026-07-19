package settings

import (
	"fmt"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
)

type settingsRepository interface {
	Get() (*Settings, error)
	UpdateWallabagSettings(*Settings) (*Settings, error)
}

type wallabagPinger interface {
	Ping() error
}

func NewHandler(settingsRepo settingsRepository, wallabag wallabagPinger) *Handler {
	return &Handler{
		settingsRepo: settingsRepo,
		wallabag:     wallabag,
	}
}

type Handler struct {
	settingsRepo settingsRepository
	wallabag     wallabagPinger
}

func (h *Handler) Settings(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("couldn't fetch settings")
	}

	connected := h.wallabag.Ping() == nil

	c.Set("Content-Type", "text/html")
	settingsView := SettingsView(settings, c.Redirect().Messages(), connected)
	return settingsView.Render(c.Context(), c.Response().BodyWriter())
}

func (h *Handler) UpdateSettings(c fiber.Ctx) error {
	var formSettings Settings

	if err := c.Bind().Form(&formSettings); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form body: " + err.Error())
	}

	_, err := h.settingsRepo.UpdateWallabagSettings(&formSettings)

	if err != nil {
		return c.Redirect().With("error", fmt.Sprintf("couldn't save settings: %s", err.Error())).To("/settings")
	}

	wallabago.SetConfig(wallabago.NewWallabagConfig(
		formSettings.WallabagInstanceURL,
		formSettings.WallabagClientID,
		formSettings.WallabagClientSecret,
		formSettings.WallabagUsername,
		formSettings.WallabagPassword),
	)

	return c.Redirect().With("success", "Settings updated!").To("/settings")
}
