package auth

import (
	"buckleberry/internal/settings"
	"fmt"
	"testing"
)

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
	stub := newStubSettingsRepo(&settings.Settings{Username: "user", Password: "pass"}, nil)

	authorizer := NewAuthorizer(stub)

	res := authorizer.Authorize("user", "pass", nil)

	if !res {
		t.Fatal("Authorize() == false, want true")
	}
}

func TestAuthorizerFail(t *testing.T) {
	stub := newStubSettingsRepo(&settings.Settings{Username: "user", Password: "pass"}, nil)

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
