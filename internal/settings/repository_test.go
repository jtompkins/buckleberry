package settings

import (
	"path/filepath"
	"strconv"
	"testing"

	"buckleberry/internal/database"
	"buckleberry/internal/linkding"
	"buckleberry/internal/wallabag"
)

func strptr(s string) *string {
	return &s
}

func newTestRepository(t *testing.T) *Repository {
	t.Helper()

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	return NewRepository(db)
}

func TestRepositoryGetWhenEmpty(t *testing.T) {
	repo := newTestRepository(t)

	got, err := repo.Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != nil {
		t.Fatalf("Get() = %#v, want nil", got)
	}
}

func TestRepositoryOnboardingState(t *testing.T) {
	repo := newTestRepository(t)

	onboarded, err := repo.IsOnboarded()
	if err != nil {
		t.Fatalf("IsOnboarded() before Create() error = %v", err)
	}
	if onboarded {
		t.Fatal("IsOnboarded() before Create() = true, want false")
	}

	if _, err := repo.Create(testSettings()); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	onboarded, err = repo.IsOnboarded()
	if err != nil {
		t.Fatalf("IsOnboarded() after Create() error = %v", err)
	}
	if !onboarded {
		t.Fatal("IsOnboarded() after Create() = false, want true")
	}
}

func TestRepositoryCreateAndUpdate(t *testing.T) {
	repo := newTestRepository(t)
	want := testSettings()

	created, err := repo.Create(want)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	assertSettingsFields(t, created, want)
	if created.ID == 0 {
		t.Error("Create() ID = 0, want generated ID")
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Error("Create() did not populate database timestamps")
	}

	// UpdateWallabagSettings only touches the wallabag_* columns; the app
	// username/password are intentionally left untouched.
	updatedValues := *created
	updatedValues.WallabagInstanceURL = strptr("https://new.example.com")
	updatedValues.WallabagUsername = strptr("new-wallabag-user")
	updatedValues.WallabagPassword = strptr("new-wallabag-password")
	updatedValues.WallabagClientID = strptr("new-client-id")
	updatedValues.WallabagClientSecret = strptr("new-client-secret")
	updatedValues.UseWallabag = false
	updatedValues.UseLinkding = true
	updatedValues.LinkdingInstanceURL = strptr("https://new-linkding.example.com")
	updatedValues.LinkdingAPIKey = strptr("new-linkding-key")

	updated, err := repo.UpdateWallabagSettings(&updatedValues)
	if err != nil {
		t.Fatalf("UpdateWallabagSettings() error = %v", err)
	}
	assertSettingsFields(t, updated, &updatedValues)

	// The credentials the update doesn't manage must be preserved.
	if updated.Username != created.Username || updated.Password != created.Password {
		t.Errorf("UpdateWallabagSettings() changed app credentials: got user=%q pass=%q, want user=%q pass=%q",
			updated.Username, updated.Password, created.Username, created.Password)
	}
}

func testSettings() *Settings {
	return &Settings{
		Username:    "reader",
		Password:    "password",
		UseWallabag: true,
		WallabagSettings: wallabag.WallabagSettings{
			WallabagInstanceURL:  strptr("https://wallabag.example.com"),
			WallabagUsername:     strptr("wallabag-user"),
			WallabagPassword:     strptr("wallabag-password"),
			WallabagClientID:     strptr("client-id"),
			WallabagClientSecret: strptr("client-secret"),
		},
		UseLinkding: true,
		LinkdingSettings: linkding.LinkdingSettings{
			LinkdingInstanceURL: strptr("https://linkding.example.com"),
			LinkdingAPIKey:      strptr("linkding-key"),
		},
	}
}

func assertSettingsFields(t *testing.T, got, want *Settings) {
	t.Helper()

	if got.Username != want.Username || got.Password != want.Password {
		t.Errorf("app credentials = %q/%q, want %q/%q", got.Username, got.Password, want.Username, want.Password)
	}
	if got.UseWallabag != want.UseWallabag {
		t.Errorf("UseWallabag = %v, want %v", got.UseWallabag, want.UseWallabag)
	}
	if got.UseLinkding != want.UseLinkding {
		t.Errorf("UseLinkding = %v, want %v", got.UseLinkding, want.UseLinkding)
	}

	fields := []struct {
		name      string
		got, want *string
	}{
		{"WallabagInstanceURL", got.WallabagInstanceURL, want.WallabagInstanceURL},
		{"WallabagUsername", got.WallabagUsername, want.WallabagUsername},
		{"WallabagPassword", got.WallabagPassword, want.WallabagPassword},
		{"WallabagClientID", got.WallabagClientID, want.WallabagClientID},
		{"WallabagClientSecret", got.WallabagClientSecret, want.WallabagClientSecret},
		{"LinkdingInstanceURL", got.LinkdingInstanceURL, want.LinkdingInstanceURL},
		{"LinkdingAPIKey", got.LinkdingAPIKey, want.LinkdingAPIKey},
	}

	for _, field := range fields {
		if !equalStringPtr(field.got, field.want) {
			t.Errorf("%s = %s, want %s", field.name, formatStringPtr(field.got), formatStringPtr(field.want))
		}
	}
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func formatStringPtr(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return strconv.Quote(*s)
}
