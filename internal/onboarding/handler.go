package onboarding

import (
	"buckleberry/internal/linkding"
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

type linkdingConfigurer interface {
	Configure(*linkding.LinkdingSettings)
}

type Handler struct {
	settingsRepo settingsRepo
	wallabag     wallabagConfigurer
	linkding     linkdingConfigurer
}

func NewHandler(repo settingsRepo, wallabag wallabagConfigurer, linkding linkdingConfigurer) *Handler {
	return &Handler{settingsRepo: repo, wallabag: wallabag, linkding: linkding}
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

	// An install with no source has nothing to serve, so require at least one.
	// This is enforced here rather than in the form because the integration
	// partials each own their own Alpine scope.
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

	if formSettings.UseWallabag {
		h.wallabag.Configure(&formSettings.WallabagSettings)
	}

	if formSettings.UseLinkding {
		h.linkding.Configure(&formSettings.LinkdingSettings)
	}

	return c.Redirect().To("/settings")
}
