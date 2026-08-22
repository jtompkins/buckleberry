package home

import (
	"buckleberry/internal/adapter"
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

type settingsRepository interface {
	Get() (*settings.Settings, error)
	Update(*settings.Settings) (*settings.Settings, error)
}

func NewHandler(settingsRepo settingsRepository) *Handler {
	return &Handler{
		settingsRepo: settingsRepo,
	}
}

type Handler struct {
	settingsRepo settingsRepository
}

func (h *Handler) Home(c fiber.Ctx) error {
	adapters := c.Locals("adapters").(map[string]adapter.Adapter)
	settings, err := h.settingsRepo.Get()

	if err != nil {
		log.Error("fetch settings: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't load settings")
	}

	connectedAdapters := map[string]bool{}

	for name, adapter := range adapters {
		err = adapter.Ping()
		connectedAdapters[name] = err == nil
	}

	c.Set("Content-Type", "text/html")
	homeView := HomeView(settings, c.Redirect().Messages(), connectedAdapters)
	return homeView.Render(c.Context(), c.Response().BodyWriter())
}

func (h *Handler) UpdateSettings(c fiber.Ctx) error {
	var formSettings settings.Settings

	if err := c.Bind().Form(&formSettings); err != nil {
		log.Error("bind settings form: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form submission")
	}

	_, err := h.settingsRepo.Update(&formSettings)

	if err != nil {
		log.Error("update settings: ", err)
		return c.Redirect().With("error", "Couldn't save settings").To("/")
	}

	return c.Redirect().With("success", "Settings updated!").To("/")
}
