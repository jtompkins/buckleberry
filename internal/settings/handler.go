package settings

import (
	"fmt"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
)

func NewHandler(settingsRepo *Repository) *Handler {
	return &Handler{
		settingsRepo: settingsRepo,
	}
}

type Handler struct {
	settingsRepo *Repository
}

func (h *Handler) Settings(c fiber.Ctx) error {
	settings, err := h.settingsRepo.Get()

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("couldn't fetch settings")
	}

	_, err = wallabago.GetTags(wallabago.APICall)

	c.Set("Content-Type", "text/html")
	settingsView := SettingsView(settings, c.Redirect().Messages(), err == nil)
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
