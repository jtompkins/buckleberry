package onboarding

import (
	"github.com/gofiber/fiber/v3"
)

type settingsReader interface {
	IsOnboarded() (bool, error)
}

type Middleware struct {
	settingsRepo settingsReader
}

func NewMiddleware(reader settingsReader) *Middleware {
	return &Middleware{settingsRepo: reader}
}

func (m *Middleware) RequireOnboarded(c fiber.Ctx) error {
	isOnboarded, err := m.settingsRepo.IsOnboarded()

	if err != nil {
		return err
	}

	if !isOnboarded {
		return c.Redirect().To("/onboarding")
	}

	return c.Next()
}

// RedirectIfOnboarded guards the onboarding routes so that a server which has
// already been set up can't be onboarded a second time (which would insert a
// duplicate settings row).
func (m *Middleware) RedirectIfOnboarded(c fiber.Ctx) error {
	isOnboarded, err := m.settingsRepo.IsOnboarded()

	if err != nil {
		return err
	}

	if isOnboarded {
		return c.Redirect().To("/")
	}

	return c.Next()
}
