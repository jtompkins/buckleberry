package settings

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

type stubRepo struct {
	settings  *Settings
	getErr    error
	updated   *Settings
	updateErr error
}

func (s *stubRepo) Get() (*Settings, error) {
	return s.settings, s.getErr
}

func (s *stubRepo) UpdateWallabagSettings(in *Settings) (*Settings, error) {
	s.updated = in
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return in, nil
}

type stubWallabag struct {
	pingErr    error
	configured *Settings
}

func (s *stubWallabag) Ping() error {
	return s.pingErr
}

func (s *stubWallabag) Configure(in *Settings) {
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

func TestSettingsRenders(t *testing.T) {
	repo := &stubRepo{settings: &Settings{Username: "reader", WallabagInstanceURL: "https://wallabag.example.com"}}
	handler := NewHandler(repo, &stubWallabag{})

	app := fiber.New()
	app.Get("/settings", handler.Settings)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestSettingsRepoError(t *testing.T) {
	handler := NewHandler(&stubRepo{getErr: errors.New("db down")}, &stubWallabag{})

	app := fiber.New()
	app.Get("/settings", handler.Settings)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusInternalServerError)
	}
}

// A failing Ping must not fail the page; it just renders as "not connected".
func TestSettingsRendersWhenWallabagUnreachable(t *testing.T) {
	repo := &stubRepo{settings: &Settings{}}
	handler := NewHandler(repo, &stubWallabag{pingErr: errors.New("unreachable")})

	app := fiber.New()
	app.Get("/settings", handler.Settings)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusOK)
	}
}

func TestUpdateSettingsSuccess(t *testing.T) {
	repo := &stubRepo{}
	handler := NewHandler(repo, &stubWallabag{})

	app := fiber.New()
	app.Post("/settings", handler.UpdateSettings)

	res := postForm(t, app, "/settings", map[string]string{
		"wallabag-url":           "https://new.example.com",
		"wallabag-username":      "new-user",
		"wallabag-password":      "new-pass",
		"wallabag-client-id":     "new-id",
		"wallabag-client-secret": "new-secret",
	})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if loc := res.Header.Get("Location"); loc != "/settings" {
		t.Errorf("redirect = %q, want /settings", loc)
	}
	if repo.updated == nil {
		t.Fatal("UpdateWallabagSettings() was not called")
	}
	if repo.updated.WallabagInstanceURL != "https://new.example.com" {
		t.Errorf("updated URL = %q, want the submitted URL", repo.updated.WallabagInstanceURL)
	}
}

func TestUpdateSettingsError(t *testing.T) {
	repo := &stubRepo{updateErr: errors.New("write failed")}
	handler := NewHandler(repo, &stubWallabag{})

	app := fiber.New()
	app.Post("/settings", handler.UpdateSettings)

	res := postForm(t, app, "/settings", map[string]string{"wallabag-url": "https://new.example.com"})
	defer res.Body.Close()

	// On failure the handler redirects back to /settings with a flash error.
	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want redirect %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if loc := res.Header.Get("Location"); loc != "/settings" {
		t.Errorf("redirect = %q, want /settings", loc)
	}
}
