package onboarding

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
)

type settingsRepo interface {
	Get() (*settings.Settings, error)
	Create(settings *settings.Settings) (bool, error)
}

type Handler struct {
	settingsRepo settingsRepo
}

func NewHandler(repo settingsRepo) *Handler {
	return &Handler{settingsRepo: repo}
}

func (h *Handler) HandleOnboarding(c fiber.Ctx) {

}

func (h *Handler) FinishOnboarding() {

}
