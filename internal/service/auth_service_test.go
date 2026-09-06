package service

import (
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"url_shortener/internal/apperrors"
	"url_shortener/internal/models"
)

type fakeUserRepository struct {
	users   map[string]*models.User
	saveErr error
	getErr  error
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: make(map[string]*models.User)}
}

func (f *fakeUserRepository) Save(u *models.User) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.users[u.Email] = u
	return nil
}

func (f *fakeUserRepository) GetByEmail(email string) (*models.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	u, ok := f.users[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func newTestAuthService(t *testing.T) (*AuthService, *fakeUserRepository) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	users := newFakeUserRepository()
	svc := NewAuthService(users, client, []byte("test-secret"), 15*time.Minute, 7*24*time.Hour)
	return svc, users
}

func TestRegister_NewUser(t *testing.T) {
	svc, users := newTestAuthService(t)

	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	stored, ok := users.users["alice@example.com"]
	if !ok {
		t.Fatal("Register() did not save the user")
	}
	if stored.PasswordHash == "hunter2" {
		t.Error("Register() stored the raw password instead of a hash")
	}
}

func TestRegister_EmailAlreadyExists(t *testing.T) {
	svc, _ := newTestAuthService(t)

	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("first Register() unexpected error: %v", err)
	}

	err := svc.Register("alice@example.com", "differentpassword")

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("second Register() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.AlreadyExists {
		t.Errorf("second Register() Kind = %v, want AlreadyExists", appErr.Kind)
	}
}

func TestLogin_Success(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	access, refresh, err := svc.Login("alice@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}
	if access == "" {
		t.Error("Login() returned empty access token")
	}
	if refresh == "" {
		t.Error("Login() returned empty refresh token")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}

	_, _, err := svc.Login("alice@example.com", "wrongpassword")

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Login() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.Unauthorized {
		t.Errorf("Login() Kind = %v, want Unauthorized", appErr.Kind)
	}
}

func TestLogin_UnknownEmail(t *testing.T) {
	svc, _ := newTestAuthService(t)

	_, _, err := svc.Login("nobody@example.com", "hunter2")

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Login() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.Unauthorized {
		t.Errorf("Login() Kind = %v, want Unauthorized", appErr.Kind)
	}
}

func TestRefresh_Success(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	_, refreshToken, err := svc.Login("alice@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	newAccess, newRefresh, err := svc.Refresh(refreshToken)
	if err != nil {
		t.Fatalf("Refresh() unexpected error: %v", err)
	}
	if newAccess == "" {
		t.Error("Refresh() returned empty access token")
	}
	if newRefresh == refreshToken {
		t.Error("Refresh() did not rotate the refresh token")
	}
}

func TestRefresh_OldTokenInvalidatedAfterRotation(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	_, refreshToken, err := svc.Login("alice@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	if _, _, err := svc.Refresh(refreshToken); err != nil {
		t.Fatalf("first Refresh() unexpected error: %v", err)
	}

	_, _, err = svc.Refresh(refreshToken)

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("second Refresh() with old token error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.Unauthorized {
		t.Errorf("second Refresh() Kind = %v, want Unauthorized", appErr.Kind)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	svc, _ := newTestAuthService(t)

	_, _, err := svc.Refresh("does-not-exist")

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Refresh() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.Unauthorized {
		t.Errorf("Refresh() Kind = %v, want Unauthorized", appErr.Kind)
	}
}

func TestLogout_RevokesRefreshToken(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if err := svc.Register("alice@example.com", "hunter2"); err != nil {
		t.Fatalf("Register() unexpected error: %v", err)
	}
	_, refreshToken, err := svc.Login("alice@example.com", "hunter2")
	if err != nil {
		t.Fatalf("Login() unexpected error: %v", err)
	}

	if err := svc.Logout(refreshToken); err != nil {
		t.Fatalf("Logout() unexpected error: %v", err)
	}

	_, _, err = svc.Refresh(refreshToken)

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("Refresh() after logout error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.Unauthorized {
		t.Errorf("Refresh() after logout Kind = %v, want Unauthorized", appErr.Kind)
	}
}

func TestLogout_UnknownToken(t *testing.T) {
	svc, _ := newTestAuthService(t)

	if err := svc.Logout("does-not-exist"); err != nil {
		t.Errorf("Logout() unexpected error for unknown token: %v", err)
	}
}
