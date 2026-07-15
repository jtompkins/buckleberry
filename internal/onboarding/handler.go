package onboarding

import (
	"buckleberry/internal/settings"
	"fmt"

	"github.com/Strubbl/wallabago/v9"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
)

type settingsRepo interface {
	Get() (*settings.Settings, error)
	Create(settings *settings.Settings) (*settings.Settings, error)
}

type Handler struct {
	settingsRepo settingsRepo
}

func NewHandler(repo settingsRepo) *Handler {
	return &Handler{settingsRepo: repo}
}

func (h *Handler) HandleOnboarding(c fiber.Ctx) error {
	c.Set("Content-Type", "text/html")
	component := OnboardingView()
	return component.Render(c.Context(), c.Response().BodyWriter())
}

func (h *Handler) FinishOnboarding(c fiber.Ctx) error {
	var formSettings settings.Settings

	if err := c.Bind().Form(&formSettings); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form body: " + err.Error())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(formSettings.Password), bcrypt.DefaultCost)

	if err != nil {
		return fmt.Errorf("unable to hash password for user %s: %w", formSettings.Username, err)
	}

	formSettings.Password = string(hashedPassword)

	_, err = h.settingsRepo.Create(&formSettings)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
	}

	wallabago.SetConfig(wallabago.NewWallabagConfig(
		formSettings.WallabagInstanceURL,
		formSettings.WallabagClientID,
		formSettings.WallabagClientSecret,
		formSettings.WallabagUsername,
		formSettings.WallabagPassword),
	)

	return c.Redirect().To("/")

}
