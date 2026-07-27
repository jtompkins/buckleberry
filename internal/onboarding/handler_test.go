package onboarding

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"buckleberry/internal/linkding"
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

type stubLinkdingConfigurer struct {
	configured *linkding.LinkdingSettings
}

func (s *stubLinkdingConfigurer) Configure(in *linkding.LinkdingSettings) {
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
		"use-wallabag":           "on",
	}
}

func TestHandleOnboardingSuccess(t *testing.T) {
	repo := &stubOnboardingRepo{}
	configurer := &stubConfigurer{}
	handler := NewHandler(repo, configurer, &stubLinkdingConfigurer{})

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
	if repo.created.WallabagInstanceURL == nil || *repo.created.WallabagInstanceURL != "https://wallabag.example.com" {
		t.Errorf("created wallabag URL = %v, want the submitted URL", repo.created.WallabagInstanceURL)
	}
	if !repo.created.UseWallabag {
		t.Error("created UseWallabag = false, want the submitted checkbox to enable Wallabag")
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
	handler := NewHandler(repo, configurer, &stubLinkdingConfigurer{})

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

func TestHandleOnboardingPersistsLinkdingSettings(t *testing.T) {
	repo := &stubOnboardingRepo{}
	handler := NewHandler(repo, &stubConfigurer{}, &stubLinkdingConfigurer{})

	app := fiber.New()
	app.Post("/onboarding", handler.HandleOnboarding)

	res := postForm(t, app, "/onboarding", map[string]string{
		"username":              "reader",
		"password":              "secret",
		"password-again":        "secret",
		"use-linkding":          "on",
		"linkding-instance-url": "https://linkding.example.com",
		"linkding-api-key":      "linkding-key",
	})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if repo.created == nil {
		t.Fatal("Create() was not called")
	}
	if !repo.created.UseLinkding {
		t.Error("created UseLinkding = false, want Linkding enabled")
	}
	if repo.created.LinkdingInstanceURL == nil || *repo.created.LinkdingInstanceURL != "https://linkding.example.com" {
		t.Errorf("created Linkding URL = %v, want the submitted URL", repo.created.LinkdingInstanceURL)
	}
	if repo.created.LinkdingAPIKey == nil || *repo.created.LinkdingAPIKey != "linkding-key" {
		t.Errorf("created Linkding API key = %v, want the submitted key", repo.created.LinkdingAPIKey)
	}
}

// Onboarding without the Wallabag box ticked must not push empty credentials
// into the Wallabag client.
func TestHandleOnboardingSkipsWallabagConfigureWhenDisabled(t *testing.T) {
	repo := &stubOnboardingRepo{}
	configurer := &stubConfigurer{}
	handler := NewHandler(repo, configurer, &stubLinkdingConfigurer{})

	app := fiber.New()
	app.Post("/onboarding", handler.HandleOnboarding)

	res := postForm(t, app, "/onboarding", map[string]string{
		"username":       "reader",
		"password":       "secret",
		"password-again": "secret",
		"use-linkding":   "on",
	})
	defer res.Body.Close()

	if repo.created == nil {
		t.Fatal("Create() was not called")
	}
	if repo.created.UseWallabag {
		t.Error("created UseWallabag = true, want false when the box is unchecked")
	}
	if configurer.configured != nil {
		t.Error("Configure() was called even though Wallabag is disabled")
	}
}

// The integration partials are rendered with no settings to display on a
// fresh install, which must not panic.
func TestOnboardingRendersOnFreshInstall(t *testing.T) {
	handler := NewHandler(&stubOnboardingRepo{}, &stubConfigurer{}, &stubLinkdingConfigurer{})

	app := fiber.New()
	app.Get("/onboarding", handler.Onboarding)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/onboarding", nil))
	if err != nil {
		t.Fatalf("GET /onboarding: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)

	// Each integration checkbox lives in its own partial, so exactly one of
	// each must be rendered -- a duplicate would submit the field twice.
	for _, field := range []string{`name="use-wallabag"`, `name="use-linkding"`} {
		if got := strings.Count(html, field); got != 1 {
			t.Errorf("rendered %s %d times, want exactly 1", field, got)
		}
	}
}

func TestHandleOnboardingConfiguresLinkding(t *testing.T) {
	repo := &stubOnboardingRepo{}
	linkdingConfigurer := &stubLinkdingConfigurer{}
	wallabagConfigurer := &stubConfigurer{}
	handler := NewHandler(repo, wallabagConfigurer, linkdingConfigurer)

	app := fiber.New()
	app.Post("/onboarding", handler.HandleOnboarding)

	res := postForm(t, app, "/onboarding", map[string]string{
		"username":              "reader",
		"password":              "secret",
		"password-again":        "secret",
		"use-linkding":          "on",
		"linkding-instance-url": "https://linkding.example.com",
		"linkding-api-key":      "linkding-key",
	})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusSeeOther)
	}
	// Without this the Linkding client stays unconfigured until a restart.
	if linkdingConfigurer.configured == nil {
		t.Fatal("Configure() was not called on the Linkding client after onboarding")
	}
	if linkdingConfigurer.configured.LinkdingInstanceURL == nil ||
		*linkdingConfigurer.configured.LinkdingInstanceURL != "https://linkding.example.com" {
		t.Errorf("configured URL = %v, want the submitted URL", linkdingConfigurer.configured.LinkdingInstanceURL)
	}
	if wallabagConfigurer.configured != nil {
		t.Error("Configure() was called on the Wallabag client, which is disabled")
	}
}

// An install with no source selected has nothing to serve.
func TestHandleOnboardingRequiresASource(t *testing.T) {
	repo := &stubOnboardingRepo{}
	wallabagConfigurer := &stubConfigurer{}
	linkdingConfigurer := &stubLinkdingConfigurer{}
	handler := NewHandler(repo, wallabagConfigurer, linkdingConfigurer)

	app := fiber.New()
	app.Post("/onboarding", handler.HandleOnboarding)

	res := postForm(t, app, "/onboarding", map[string]string{
		"username":       "reader",
		"password":       "secret",
		"password-again": "secret",
	})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d (re-rendered form)", res.StatusCode, fiber.StatusOK)
	}
	if repo.created != nil {
		t.Error("Create() should not be called when no source is selected")
	}
	if wallabagConfigurer.configured != nil || linkdingConfigurer.configured != nil {
		t.Error("no client should be configured when no source is selected")
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "at least one") {
		t.Error("re-rendered form does not explain that a source is required")
	}
}
