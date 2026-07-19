package auth

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

func NewMiddleware(settingsRepo *settings.Repository) *Middleware {
	return &Middleware{
		settingsRepo: settingsRepo,
	}
}

type Middleware struct {
	settingsRepo *settings.Repository
}

func (m *Middleware) RequireAuth(c fiber.Ctx) error {
	sess := session.FromContext(c)
	if sess == nil {
		return c.Redirect().To("/")
	}

	return c.Next()
}
