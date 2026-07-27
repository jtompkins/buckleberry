package settings

import (
	"buckleberry/internal/linkding"
	"buckleberry/internal/wallabag"
	"errors"
	"io"
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

func (s *stubRepo) Update(in *Settings) (*Settings, error) {
	s.updated = in
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return in, nil
}

type stubWallabag struct {
	pingErr    error
	configured *wallabag.WallabagSettings
}

type stubLinkding struct {
	pingErr    error
	configured *linkding.LinkdingSettings
}

func (s *stubLinkding) Ping() error {
	return s.pingErr
}

func (s *stubLinkding) Configure(in *linkding.LinkdingSettings) {
	s.configured = in
}

func (s *stubWallabag) Ping() error {
	return s.pingErr
}

func (s *stubWallabag) Configure(in *wallabag.WallabagSettings) {
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
	repo := &stubRepo{settings: &Settings{Username: "reader", WallabagSettings: wallabag.WallabagSettings{UseWallabag: true, WallabagInstanceURL: strptr("https://wallabag.example.com")}}}
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

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
	handler := NewHandler(&stubRepo{getErr: errors.New("db down")}, &stubWallabag{}, &stubLinkding{})

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
	handler := NewHandler(repo, &stubWallabag{pingErr: errors.New("unreachable")}, &stubLinkding{pingErr: errors.New("unreachable")})

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
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

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
		t.Fatal("Update() was not called")
	}
	if repo.updated.WallabagInstanceURL == nil || *repo.updated.WallabagInstanceURL != "https://new.example.com" {
		t.Errorf("updated URL = %v, want the submitted URL", repo.updated.WallabagInstanceURL)
	}
}

func TestUpdateSettingsError(t *testing.T) {
	repo := &stubRepo{updateErr: errors.New("write failed")}
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

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

// The rendered page reports each integration's reachability independently.
func TestSettingsRendersConnectionStatus(t *testing.T) {
	tests := []struct {
		name              string
		wallabagPingErr   error
		linkdingPingErr   error
		wantWallabagState string
		wantLinkdingState string
	}{
		{name: "both reachable", wantWallabagState: "✅", wantLinkdingState: "✅"},
		{name: "wallabag down", wallabagPingErr: errors.New("unreachable"), wantWallabagState: "❌", wantLinkdingState: "✅"},
		{name: "linkding down", linkdingPingErr: errors.New("unreachable"), wantWallabagState: "✅", wantLinkdingState: "❌"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubRepo{settings: &Settings{
				Username:         "reader",
				WallabagSettings: wallabag.WallabagSettings{UseWallabag: true},
				LinkdingSettings: linkding.LinkdingSettings{UseLinkding: true},
			}}
			handler := NewHandler(repo, &stubWallabag{pingErr: tc.wallabagPingErr}, &stubLinkding{pingErr: tc.linkdingPingErr})

			app := fiber.New()
			app.Get("/settings", handler.Settings)

			res, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
			if err != nil {
				t.Fatalf("GET /settings: %v", err)
			}
			defer res.Body.Close()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("read body: %v", err)
			}
			html := string(body)

			wallabagStatus := statusAfter(t, html, "Wallabag connection:")
			if wallabagStatus != tc.wantWallabagState {
				t.Errorf("Wallabag status = %q, want %q", wallabagStatus, tc.wantWallabagState)
			}
			linkdingStatus := statusAfter(t, html, "Linkding connection:")
			if linkdingStatus != tc.wantLinkdingState {
				t.Errorf("Linkding status = %q, want %q", linkdingStatus, tc.wantLinkdingState)
			}
		})
	}
}

// statusAfter returns the first connection indicator rendered after label.
func statusAfter(t *testing.T, html, label string) string {
	t.Helper()

	idx := strings.Index(html, label)
	if idx < 0 {
		t.Fatalf("rendered page has no %q label", label)
	}

	rest := html[idx:]
	ok, bad := strings.Index(rest, "✅"), strings.Index(rest, "❌")

	if ok < 0 && bad < 0 {
		t.Fatalf("no connection indicator after %q", label)
	}
	if bad < 0 || (ok >= 0 && ok < bad) {
		return "✅"
	}
	return "❌"
}

func TestSettingsRendersExistingLinkdingSettings(t *testing.T) {
	repo := &stubRepo{settings: &Settings{
		LinkdingSettings: linkding.LinkdingSettings{
			UseLinkding:         true,
			LinkdingInstanceURL: strptr("https://linkding.example.com"),
			LinkdingAPIKey:      strptr("linkding-key"),
		},
	}}
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

	app := fiber.New()
	app.Get("/settings", handler.Settings)

	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	if err != nil {
		t.Fatalf("GET /settings: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	html := string(body)

	for _, want := range []string{"https://linkding.example.com", "linkding-key", "linkding-instance-url", "linkding-api-key"} {
		if !strings.Contains(html, want) {
			t.Errorf("rendered page missing %q", want)
		}
	}
}

// A nil *string field must render as an empty input rather than panicking --
// this is the state of a fresh install that has never configured Linkding.
func TestSettingsRendersWithUnsetLinkdingSettings(t *testing.T) {
	repo := &stubRepo{settings: &Settings{}}
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

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

func TestUpdateSettingsPersistsLinkdingSettings(t *testing.T) {
	repo := &stubRepo{}
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

	app := fiber.New()
	app.Post("/settings", handler.UpdateSettings)

	res := postForm(t, app, "/settings", map[string]string{
		"use-linkding":          "on",
		"linkding-instance-url": "https://linkding.example.com",
		"linkding-api-key":      "linkding-key",
	})
	defer res.Body.Close()

	if res.StatusCode != fiber.StatusSeeOther {
		t.Fatalf("status = %d, want %d", res.StatusCode, fiber.StatusSeeOther)
	}
	if repo.updated == nil {
		t.Fatal("Update() was not called")
	}
	if !repo.updated.UseLinkding {
		t.Error("updated UseLinkding = false, want the submitted checkbox to enable Linkding")
	}
	if repo.updated.LinkdingInstanceURL == nil || *repo.updated.LinkdingInstanceURL != "https://linkding.example.com" {
		t.Errorf("updated Linkding URL = %v, want the submitted URL", repo.updated.LinkdingInstanceURL)
	}
	if repo.updated.LinkdingAPIKey == nil || *repo.updated.LinkdingAPIKey != "linkding-key" {
		t.Errorf("updated Linkding API key = %v, want the submitted key", repo.updated.LinkdingAPIKey)
	}
}

// An unchecked checkbox isn't submitted at all, which must read as "off".
func TestUpdateSettingsUncheckedBoxesDisableSources(t *testing.T) {
	repo := &stubRepo{}
	handler := NewHandler(repo, &stubWallabag{}, &stubLinkding{})

	app := fiber.New()
	app.Post("/settings", handler.UpdateSettings)

	res := postForm(t, app, "/settings", map[string]string{"wallabag-url": "https://new.example.com"})
	defer res.Body.Close()

	if repo.updated == nil {
		t.Fatal("Update() was not called")
	}
	if repo.updated.UseWallabag {
		t.Error("updated UseWallabag = true, want false when the box is unchecked")
	}
	if repo.updated.UseLinkding {
		t.Error("updated UseLinkding = true, want false when the box is unchecked")
	}
}

// Saving settings must push the new credentials into the live clients,
// otherwise the change doesn't take effect until the process restarts.
func TestUpdateSettingsReconfiguresEnabledClients(t *testing.T) {
	repo := &stubRepo{}
	wallabagClient := &stubWallabag{}
	linkdingClient := &stubLinkding{}
	handler := NewHandler(repo, wallabagClient, linkdingClient)

	app := fiber.New()
	app.Post("/settings", handler.UpdateSettings)

	res := postForm(t, app, "/settings", map[string]string{
		"use-wallabag":           "on",
		"wallabag-url":           "https://new.example.com",
		"wallabag-username":      "new-user",
		"wallabag-password":      "new-pass",
		"wallabag-client-id":     "new-id",
		"wallabag-client-secret": "new-secret",
		"use-linkding":           "on",
		"linkding-instance-url":  "https://linkding.example.com",
		"linkding-api-key":       "linkding-key",
	})
	defer res.Body.Close()

	if wallabagClient.configured == nil {
		t.Fatal("Configure() was not called on the Wallabag client")
	}
	if wallabagClient.configured.WallabagInstanceURL == nil ||
		*wallabagClient.configured.WallabagInstanceURL != "https://new.example.com" {
		t.Errorf("Wallabag configured with %v, want the submitted URL", wallabagClient.configured.WallabagInstanceURL)
	}

	if linkdingClient.configured == nil {
		t.Fatal("Configure() was not called on the Linkding client")
	}
	if linkdingClient.configured.LinkdingInstanceURL == nil ||
		*linkdingClient.configured.LinkdingInstanceURL != "https://linkding.example.com" {
		t.Errorf("Linkding configured with %v, want the submitted URL", linkdingClient.configured.LinkdingInstanceURL)
	}
}

// A disabled source must not have credentials pushed into its client.
func TestUpdateSettingsSkipsConfigureForDisabledSources(t *testing.T) {
	repo := &stubRepo{}
	wallabagClient := &stubWallabag{}
	linkdingClient := &stubLinkding{}
	handler := NewHandler(repo, wallabagClient, linkdingClient)

	app := fiber.New()
	app.Post("/settings", handler.UpdateSettings)

	res := postForm(t, app, "/settings", map[string]string{
		"use-linkding":          "on",
		"linkding-instance-url": "https://linkding.example.com",
		"linkding-api-key":      "linkding-key",
	})
	defer res.Body.Close()

	if wallabagClient.configured != nil {
		t.Error("Configure() was called on the Wallabag client, which is disabled")
	}
	if linkdingClient.configured == nil {
		t.Error("Configure() was not called on the enabled Linkding client")
	}
}
