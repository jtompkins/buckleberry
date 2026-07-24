package settings

import (
	"path/filepath"
	"testing"

	"buckleberry/internal/database"
)

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
	updatedValues.WallabagInstanceURL = "https://new.example.com"
	updatedValues.WallabagUsername = "new-wallabag-user"
	updatedValues.WallabagPassword = "new-wallabag-password"
	updatedValues.WallabagClientID = "new-client-id"
	updatedValues.WallabagClientSecret = "new-client-secret"

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
		Username:             "reader",
		Password:             "password",
		WallabagInstanceURL:  "https://wallabag.example.com",
		WallabagUsername:     "wallabag-user",
		WallabagPassword:     "wallabag-password",
		WallabagClientID:     "client-id",
		WallabagClientSecret: "client-secret",
	}
}

func assertSettingsFields(t *testing.T, got, want *Settings) {
	t.Helper()

	if got.Username != want.Username ||
		got.Password != want.Password ||
		got.WallabagInstanceURL != want.WallabagInstanceURL ||
		got.WallabagUsername != want.WallabagUsername ||
		got.WallabagPassword != want.WallabagPassword ||
		got.WallabagClientID != want.WallabagClientID ||
		got.WallabagClientSecret != want.WallabagClientSecret {
		t.Errorf("settings fields = %#v, want %#v", got, want)
	}
}
