package onboarding

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func setupFiberApp(mw *Middleware) *fiber.App {
	app := fiber.New()

	app.Get("/notonboarded", mw.RequireOnboarded, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	app.Get("/onboarded", mw.RedirectIfOnboarded, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	return app
}

type mockSettingsReader struct {
	mock.Mock
}

func (m *mockSettingsReader) IsOnboarded() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func TestReturnsErrorIfSettingsFetchFails(t *testing.T) {
	mockRepo := new(mockSettingsReader)
	mockRepo.On("IsOnboarded").Return(false, fmt.Errorf("failed"))

	mw := NewMiddleware(mockRepo)

	app := setupFiberApp(mw)

	res, _ := app.Test(httptest.NewRequest("GET", "/onboarded", nil))

	require.Equal(t, 500, res.StatusCode)
}

func TestRedirectsIfNotOnboarded(t *testing.T) {
	mockRepo := new(mockSettingsReader)
	mockRepo.On("IsOnboarded").Return(false, nil)

	mw := NewMiddleware(mockRepo)
	app := setupFiberApp(mw)

	res, err := app.Test(httptest.NewRequest("GET", "/notonboarded", nil))

	require.Nil(t, err)

	url, _ := res.Location()

	require.Equal(t, "/onboarding", url.Path)
}

func TestDoesntAllowOnboardingIfAlreadyOnboarded(t *testing.T) {
	mockRepo := new(mockSettingsReader)
	mockRepo.On("IsOnboarded").Return(true, nil)

	mw := NewMiddleware(mockRepo)
	app := setupFiberApp(mw)

	res, err := app.Test(httptest.NewRequest("GET", "/onboarded", nil))

	require.Nil(t, err)

	url, _ := res.Location()

	require.Equal(t, "/", url.Path)
}
