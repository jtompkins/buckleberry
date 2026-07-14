package auth

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
)

type settingsReader interface {
	Get() (*settings.Settings, error)
}

type BasicAuthorizer struct {
	settingsRepo settingsReader
}

func NewAuthorizer(repo settingsReader) *BasicAuthorizer {
	return &BasicAuthorizer{repo}
}

func (m *BasicAuthorizer) Authorize(user, pass string, c fiber.Ctx) bool {
	settings, err := m.settingsRepo.Get()

	if err != nil {
		log.Error("Couldn't fetch settings from database", err)
		return false
	}

	result := user == settings.Username && pass == settings.Password

	return result
}
