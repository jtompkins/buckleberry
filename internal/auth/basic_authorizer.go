package auth

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"golang.org/x/crypto/bcrypt"
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

	return user == settings.Username && bcrypt.CompareHashAndPassword([]byte(pass), []byte(settings.Password)) == nil
}
