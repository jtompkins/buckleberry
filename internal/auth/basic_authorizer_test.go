package auth

import (
	"buckleberry/internal/settings"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

type mockSettingsRepo struct {
	mock.Mock
}

func (m *mockSettingsRepo) Get() (*settings.Settings, error) {
	args := m.Called()
	return args.Get(0).(*settings.Settings), args.Error(1)
}

func TestSettingsFetchFailure(t *testing.T) {
	mockedRepo := new(mockSettingsRepo)
	mockedRepo.On("Get").Return(&settings.Settings{}, fmt.Errorf("failed"))

	authorizer := NewAuthorizer(mockedRepo)

	require.False(t, authorizer.Authorize("user", "pass", nil))
}

func TestValidCredentials(t *testing.T) {
	setting := &settings.Settings{
		Username: "user",
		Password: hashPassword(t, "pass"),
	}

	mockedRepo := new(mockSettingsRepo)
	mockedRepo.On("Get").Return(setting, nil)

	authorizer := NewAuthorizer(mockedRepo)

	require.True(t, authorizer.Authorize("user", "pass", nil))
}

func TestInvalidCredentials(t *testing.T) {
	setting := &settings.Settings{
		Username: "user",
		Password: hashPassword(t, "pass"),
	}

	mockedRepo := new(mockSettingsRepo)
	mockedRepo.On("Get").Return(setting, nil)

	authorizer := NewAuthorizer(mockedRepo)

	require.False(t, authorizer.Authorize("baduser", "pass", nil))
	require.False(t, authorizer.Authorize("user", "badpass", nil))
}
