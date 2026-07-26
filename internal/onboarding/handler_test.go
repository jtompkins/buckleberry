package onboarding

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buckleberry/internal/settings"
	"buckleberry/internal/wallabag"

	"github.com/gofiber/fiber/v3"
)

var assertErr = errors.New("boom")

type stubOnboardingRepo struct {
	existing *settings.Settings
	getErr   error

	created   *settings.Settings
	createErr error
}

func (s *stubOnboardingRepo) Get() (*settings.Settings, error) {
	return s.existing, s.getErr
}

func (s *stubOnboardingRepo) Create(in *settings.Settings) (*settings.Settings, error) {
	s.created = in
	if s.createErr != nil {
		return nil, s.createErr
	}
	return in, nil
}

type stubConfigurer struct {
	configured *wallabag.WallabagSettings
}

func (s *stubConfigurer) Configure(in *wallabag.WallabagSettings) {
	s.configured = in
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

func validOnboardingForm() map[string]string {
	return map[string]string{
		"username":               "reader",
		"password":               "secret",
		"password-again":         "secret",
		"wallabag-url":           "https://wallabag.example.com",
		"wallabag-username":      "wal-user",
		"wallabag-password":      "wal-pass",
		"wallabag-client-id":     "client-id",
		"wallabag-client-secret": "client-secret",
	}
}

func TestHandleOnboardingSuccess(t *testing.T) {
	repo := &stubOnboardingRepo{}
	configurer := &stubConfigurer{}
	handler := NewHandler(repo, configurer)

	app := fiber.New()
	app.Post("/onboarding", handler.HandleOnboarding)

	res := postForm(t, app, "/onboarding", validOnboardingForm())
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if loc := res.Header.Get("Location"); loc != "/settings" {
		t.Errorf("redirect = %q, want /settings", loc)
	}
	if repo.created == nil {
		t.Fatal("Create() was not called")
	}
	if repo.created.Username != "reader" {
		t.Errorf("created username = %q, want reader", repo.created.Username)
	}
	if repo.created.WallabagInstanceURL != "https://wallabag.example.com" {
		t.Errorf("created wallabag URL = %q, want the submitted URL", repo.created.WallabagInstanceURL)
	}
	// The password must be stored hashed, never as the plaintext the user typed.
	if repo.created.Password == "secret" || repo.created.Password == "" {
		t.Errorf("created password = %q, want a bcrypt hash", repo.created.Password)
	}
	// Successful onboarding must push the new credentials into the Wallabag client.
	if configurer.configured == nil {
		t.Error("Configure() was not called after onboarding")
	}
}

func TestHandleOnboardingPasswordMismatch(t *testing.T) {
	repo := &stubOnboardingRepo{}
	configurer := &stubConfigurer{}
	handler := NewHandler(repo, configurer)

	app := fiber.New()
	app.Post("/onboarding", handler.HandleOnboarding)

	form := validOnboardingForm()
	form["password-again"] = "different"

	res := postForm(t, app, "/onboarding", form)
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d (re-rendered form)", res.StatusCode, fiber.StatusOK)
	}
	if repo.created != nil {
		t.Error("Create() should not be called when passwords don't match")
	}
	if configurer.configured != nil {
		t.Error("Configure() should not be called when passwords don't match")
	}
}

func TestRedirectIfOnboarded(t *testing.T) {
	tests := []struct {
		name         string
		onboarded    bool
		wantStatus   int
		wantLocation string
	}{
		{name: "already onboarded is bounced away", onboarded: true, wantStatus: fiber.StatusSeeOther, wantLocation: "/"},
		{name: "fresh install may onboard", onboarded: false, wantStatus: fiber.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			middleware := NewMiddleware(&stubSettingsReader{onboarded: tc.onboarded})

			app := fiber.New()
			app.Get("/onboarding", middleware.RedirectIfOnboarded, func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			res, err := app.Test(httptest.NewRequest(http.MethodGet, "/onboarding", nil))
			if err != nil {
				t.Fatalf("GET /onboarding: %v", err)
			}
			defer res.Body.Close()

			if res.StatusCode != tc.wantStatus {
				t.Fatalf("status = %d, want %d", res.StatusCode, tc.wantStatus)
			}
			if tc.wantLocation != "" {
				if loc := res.Header.Get("Location"); loc != tc.wantLocation {
					t.Errorf("Location = %q, want %q", loc, tc.wantLocation)
				}
			}
		})
	}
}

func TestRequireOnboardedError(t *testing.T) {
	middleware := NewMiddleware(&stubSettingsReader{err: assertErr})

	app := fiber.New()
	app.Get("/test", middleware.RequireOnboarded, func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil))
	if err != nil {
		t.Fatalf("GET /test: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusInternalServerError {
		t.Errorf("status = %d, want %d", res.StatusCode, fiber.StatusInternalServerError)
	}
}
