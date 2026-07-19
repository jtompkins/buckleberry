package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buckleberry/internal/settings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/session"
)

// newAuthApp wires the login handler plus a RequireAuth-protected route on a
// single app that shares one session store, so the login -> protected flow can
// be exercised end to end.
func newAuthApp(repo settingsReader) *fiber.App {
	app := fiber.New()
	app.Use(session.New())

	handler := NewHandler(repo)
	middleware := NewMiddleware(repo)

	app.Get("/", handler.Login)
	app.Post("/login", handler.HandleLogin)
	app.Get("/protected", middleware.RequireAuth, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	return app
}

func postForm(t *testing.T, app *fiber.App, path string, values map[string]string) *http.Response {
	t.Helper()

	form := make([]string, 0, len(values))
	for k, v := range values {
		form = append(form, k+"="+v)
	}

	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(strings.Join(form, "&")))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}

	return res
}

// sessionCookie extracts just the name=value pair from a Set-Cookie header so it
// can be replayed on a follow-up request.
func sessionCookie(t *testing.T, res *http.Response) string {
	t.Helper()

	raw := res.Header.Get("Set-Cookie")
	if raw == "" {
		t.Fatal("expected a session cookie to be set, got none")
	}

	return strings.Split(raw, ";")[0]
}

func TestLoginRendersForm(t *testing.T) {
	app := newAuthApp(newStubSettingsRepo(&settings.Settings{}, nil))

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestHandleLoginSuccessGrantsAccess(t *testing.T) {
	repo := newStubSettingsRepo(&settings.Settings{Username: "reader", Password: hashPassword(t, "secret")}, nil)
	app := newAuthApp(repo)

	res := postForm(t, app, "/login", map[string]string{"username": "reader", "password": "secret"})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("login status = %d, want %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if loc := res.Header.Get("Location"); loc != "/settings" {
		t.Errorf("login redirect = %q, want /settings", loc)
	}

	// The session established by logging in must satisfy RequireAuth.
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Cookie", sessionCookie(t, res))

	protected, err := app.Test(req)
	if err != nil {
		t.Fatalf("GET /protected: %v", err)
	}
	defer protected.Body.Close()

	if protected.StatusCode != fiber.StatusOK {
		t.Fatalf("protected status = %d, want %d", protected.StatusCode, fiber.StatusOK)
	}
}

func TestHandleLoginRejectsBadPassword(t *testing.T) {
	repo := newStubSettingsRepo(&settings.Settings{Username: "reader", Password: hashPassword(t, "secret")}, nil)
	app := newAuthApp(repo)

	res := postForm(t, app, "/login", map[string]string{"username": "reader", "password": "wrong"})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want redirect %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if loc := res.Header.Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want / (back to login)", loc)
	}
}

func TestHandleLoginRejectsUnknownUser(t *testing.T) {
	repo := newStubSettingsRepo(&settings.Settings{Username: "reader", Password: hashPassword(t, "secret")}, nil)
	app := newAuthApp(repo)

	res := postForm(t, app, "/login", map[string]string{"username": "intruder", "password": "secret"})
	defer res.Body.Close()

	if loc := res.Header.Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want / (back to login)", loc)
	}
}

// RequireAuth must reject a request that carries no authenticated session. This
// is the regression guard for the auth-bypass bug where any request with a
// session cookie (i.e. every request) was allowed through.
func TestRequireAuthBlocksAnonymous(t *testing.T) {
	app := newAuthApp(newStubSettingsRepo(&settings.Settings{}, nil))

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/protected", nil))
	if err != nil {
		t.Fatalf("GET /protected: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want redirect %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if loc := res.Header.Get("Location"); loc != "/" {
		t.Errorf("redirect = %q, want /", loc)
	}
}
