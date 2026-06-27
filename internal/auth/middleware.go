package auth

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
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
		return map[string]string{}
	}

	return map[string]string{settings.Username: settings.Password}
}

func (m *Middleware) Authorizer(user, pass string, c fiber.Ctx) bool {
	settings, err := m.repo.Get()
	if err != nil {
		return false
	}

	return settings.Username == user && settings.Password == pass
}
