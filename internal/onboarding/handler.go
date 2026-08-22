package onboarding

import (
	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
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

func (h *Handler) Onboarding(c fiber.Ctx) error {
	c.Set("Content-Type", "text/html")
	component := OnboardingView(nil, []string{})
	return component.Render(c.Context(), c.Response().BodyWriter())
}

func (h *Handler) HandleOnboarding(c fiber.Ctx) error {
	var formSettings settings.Settings

	if err := c.Bind().Form(&formSettings); err != nil {
		log.Error("bind onboarding form: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Invalid form submission")
	}

	repeatedPassword := c.FormValue("password-again")

	var errorMsgs []string

	if formSettings.Password != repeatedPassword {
		errorMsgs = append(errorMsgs, "Passwords don't match")
	}

	if !formSettings.UseWallabag && !formSettings.UseLinkding {
		errorMsgs = append(errorMsgs, "Connect at least one of Wallabag or Linkding")
	}

	if len(errorMsgs) > 0 {
		c.Set("Content-Type", "text/html")

		onboardingView := OnboardingView(&formSettings, errorMsgs)

		return onboardingView.Render(c.Context(), c.Response().BodyWriter())
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(formSettings.Password), bcrypt.DefaultCost)

	if err != nil {
		log.Errorf("hash password for user %s: %v", formSettings.Username, err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't complete onboarding")
	}

	formSettings.Password = string(hashedPassword)

	_, err = h.settingsRepo.Create(&formSettings)

	if err != nil {
		log.Error("create settings: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Couldn't complete onboarding")
	}

	return c.Redirect().To("/settings")
}
