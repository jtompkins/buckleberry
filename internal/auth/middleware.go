package auth

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"golang.org/x/crypto/bcrypt"
)

type Middleware struct {
	repo *settings.Repository
}

func NewMiddleware(repo *settings.Repository) *Middleware {
	return &Middleware{repo}
}

func (m *Middleware) GetUsers() map[string]string {
	settings, err := m.repo.Get()

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

func (m *Middleware) Authorizer(user, pass string, c fiber.Ctx) bool {
	settings, err := m.repo.Get()

	if err != nil {
		log.Error("Couldn't fetch settings from database", err)
		return false
	}

	result := user == settings.Username && pass == settings.Password

	return result
}
