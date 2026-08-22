package auth

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
	"golang.org/x/crypto/bcrypt"
)

func NewHandler(settingsRepo settingsReader) *Handler {
	return &Handler{
		settingsRepo: settingsRepo,
	}
}

type Handler struct {
	settingsRepo settingsReader
}

func (h *Handler) Login(c fiber.Ctx) error {
	c.Set("Content-Type", "text/html")

	component := LoginView(c.Redirect().Messages())
	return component.Render(c.Context(), c.Response().BodyWriter())
}

func (h *Handler) HandleLogin(c fiber.Ctx) error {

	sess := session.FromContext(c)

	username := c.FormValue("username")
	password := c.FormValue("password")

	settings, err := h.settingsRepo.Get()

	if err != nil {
		return c.Redirect().With("error", "Couldn't fetch settings").To("/login")
	}

	compareResult := bcrypt.CompareHashAndPassword([]byte(settings.Password), []byte(password))

	if username == settings.Username && compareResult == nil {
		// Important: Regenerate the session ID to prevent fixation
		// This changes the session ID while preserving existing data
		if err := sess.Regenerate(); err != nil {
			return c.Status(500).SendString("Session error")
		}

		// Add authentication data to existing session
		sess.Set("authenticated", true)

		return c.Redirect().To("/")
	}

	return c.Redirect().With("error", "Invalid credentials").To("/login")
}
