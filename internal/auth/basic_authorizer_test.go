package auth

import (
	"buckleberry/internal/settings"
	"fmt"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func hashPassword(t *testing.T, password string) string {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	return string(hash)
}

type stubSettingsRepo struct {
	settings *settings.Settings
	err      error
}

func (s *stubSettingsRepo) Get() (*settings.Settings, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.settings, nil
}

func newStubSettingsRepo(s *settings.Settings, err error) *stubSettingsRepo {
	return &stubSettingsRepo{s, err}
}

func TestSettingsFetchFailure(t *testing.T) {
	stub := newStubSettingsRepo(nil, fmt.Errorf("failed"))

	authorizer := NewAuthorizer(stub)

	res := authorizer.Authorize("user", "pass", nil)

	if res {
		t.Fatal("Authorize() == true, want false", false, res)
	}
}

func TestAuthorizerPass(t *testing.T) {
	stub := newStubSettingsRepo(&settings.Settings{Username: "user", Password: hashPassword(t, "pass")}, nil)

	authorizer := NewAuthorizer(stub)

	res := authorizer.Authorize("user", "pass", nil)

	if !res {
		t.Fatal("Authorize() == false, want true")
	}
}

func TestAuthorizerFail(t *testing.T) {
	stub := newStubSettingsRepo(&settings.Settings{Username: "user", Password: hashPassword(t, "pass")}, nil)

	authorizer := NewAuthorizer(stub)

	res := authorizer.Authorize("notuser", "pass", nil)

	if res {
		t.Fatal("Authorize(invalid username) == true, want false")
	}

	res = authorizer.Authorize("user", "notpass", nil)

	if res {
		t.Fatal("Authorize(invalid password) == true, want false")
	}
}

// A settings row with an empty password hash must never authorize, otherwise a
// freshly created (but not yet configured) instance would be wide open.
func TestAuthorizerRejectsEmptyStoredPassword(t *testing.T) {
	stub := newStubSettingsRepo(&settings.Settings{Username: "user", Password: ""}, nil)

	authorizer := NewAuthorizer(stub)

	if authorizer.Authorize("user", "", nil) {
		t.Fatal("Authorize() with empty stored hash == true, want false")
	}
}
