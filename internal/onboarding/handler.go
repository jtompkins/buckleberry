package onboarding

import (
	"buckleberry/internal/settings"
	"buckleberry/internal/wallabag"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/log"
	"golang.org/x/crypto/bcrypt"
)

type settingsRepo interface {
	Get() (*settings.Settings, error)
	Create(settings *settings.Settings) (*settings.Settings, error)
}

type wallabagConfigurer interface {
	Configure(*wallabag.WallabagSettings)
}

type Handler struct {
	settingsRepo settingsRepo
	wallabag     wallabagConfigurer
}

func NewHandler(repo settingsRepo, wallabag wallabagConfigurer) *Handler {
	return &Handler{settingsRepo: repo, wallabag: wallabag}
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

	if formSettings.Password != repeatedPassword {
		c.Set("Content-Type", "text/html")

		onboardingView := OnboardingView(&formSettings, []string{"Passwords don't match"})

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

	if formSettings.UseWallabag {
		h.wallabag.Configure(&formSettings.WallabagSettings)
	}

	return c.Redirect().To("/settings")
}
