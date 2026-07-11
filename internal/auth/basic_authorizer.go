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

func NewAuthorizer(repo *settings.Repository) *BasicAuthorizer {
	return &BasicAuthorizer{repo}
}

func (m *BasicAuthorizer) GetUsers() map[string]string {
	settings, err := m.settingsRepo.Get()

	if err != nil {
		log.Error("Couldn't fetch settings from database", err)
		return map[string]string{}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(settings.Password), bcrypt.DefaultCost)

	if err != nil {
		log.Fatal("Could not hash settings password", err.Error())
	}

	return map[string]string{settings.Username: string(hashedPassword)}
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
