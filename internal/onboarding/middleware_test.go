package onboarding

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type stubSettingsReader struct {
	onboarded bool
	err       error
}

func (s *stubSettingsReader) IsOnboarded() (bool, error) {
	if s.err != nil {
		return false, s.err
	}

	return s.onboarded, nil
}

func TestMiddleware(t *testing.T) {
	repo := &stubSettingsReader{false, nil}

	middleware := NewMiddleware(repo)

	app := fiber.New()

	app.Get("/test", middleware.RequireOnboarded, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	res, err := app.Test(httptest.NewRequest("GET", "/test", nil))

	if err != nil {
		t.Fatal(err)
	}

	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Errorf("status == %d, want %d", res.StatusCode, fiber.StatusFound)
	}

	if location := res.Header.Get("Location"); location != "/onboarding" {
		t.Errorf("Location = %q, want /onboarding", location)
	}
}
