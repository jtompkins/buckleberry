package auth

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

func NewMiddleware(settingsRepo settingsReader) *Middleware {
	return &Middleware{
		settingsRepo: settingsRepo,
	}
}

type Middleware struct {
	settingsRepo settingsReader
}

func (m *Middleware) RequireAuth(c fiber.Ctx) error {
	sess := session.FromContext(c)
	if sess == nil || sess.Get("authenticated") != true {
		return c.Redirect().To("/")
	}

	return c.Next()
}
