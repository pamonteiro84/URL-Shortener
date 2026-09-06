package service

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"url_shortener/internal/apperrors"
	"url_shortener/internal/models"
)

type fakeStorage struct {
	urls      map[string]*models.URL
	saveErr   error
	getErr    error
	deleteErr error
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{urls: make(map[string]*models.URL)}
}

func (f *fakeStorage) Save(u *models.URL) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.urls[u.ShortCode] = u
	return nil
}

func (f *fakeStorage) GetByShortURL(shortURL string) (*models.URL, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	u, ok := f.urls[shortURL]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *fakeStorage) Delete(shortCode string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.urls[shortCode]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.urls, shortCode)
	return nil
}

func TestShortenURL_NewURL(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	shortCode, err := svc.ShortenURL("https://example.com", 1)
	if err != nil {
		t.Fatalf("ShortenURL() unexpected error: %v", err)
	}
	if shortCode == "" {
		t.Fatal("ShortenURL() returned empty short code")
	}

	stored, ok := storage.urls[shortCode]
	if !ok {
		t.Fatal("ShortenURL() did not save the URL in storage")
	}
	if stored.OriginalURL != "https://example.com" {
		t.Errorf("stored OriginalURL = %q, want %q", stored.OriginalURL, "https://example.com")
	}
	if stored.UserID != 1 {
		t.Errorf("stored UserID = %d, want 1", stored.UserID)
	}
}

func TestShortenURL_Dedupe(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	first, err := svc.ShortenURL("https://example.com", 1)
	if err != nil {
		t.Fatalf("first ShortenURL() unexpected error: %v", err)
	}

	second, err := svc.ShortenURL("https://example.com", 1)

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("second ShortenURL() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.AlreadyExists {
		t.Errorf("second ShortenURL() Kind = %v, want AlreadyExists", appErr.Kind)
	}
	if second != first {
		t.Errorf("second ShortenURL() short code = %q, want %q (same as first)", second, first)
	}
}

func TestShortenURL_DifferentUsersGetDifferentCodes(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	first, err := svc.ShortenURL("https://example.com", 1)
	if err != nil {
		t.Fatalf("first ShortenURL() unexpected error: %v", err)
	}

	second, err := svc.ShortenURL("https://example.com", 2)
	if err != nil {
		t.Fatalf("second ShortenURL() unexpected error: %v", err)
	}

	if first == second {
		t.Errorf("ShortenURL() gave the same code %q to two different users for the same URL", first)
	}
}

func TestGetOriginalURL_Found(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	shortCode, err := svc.ShortenURL("https://example.com", 1)
	if err != nil {
		t.Fatalf("ShortenURL() unexpected error: %v", err)
	}

	got, err := svc.GetOriginalURL(shortCode)
	if err != nil {
		t.Fatalf("GetOriginalURL() unexpected error: %v", err)
	}
	if got != "https://example.com" {
		t.Errorf("GetOriginalURL() = %q, want %q", got, "https://example.com")
	}
}

func TestGetOriginalURL_NotFound(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	_, err := svc.GetOriginalURL("doesnotexist")

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("GetOriginalURL() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.NotFound {
		t.Errorf("GetOriginalURL() Kind = %v, want NotFound", appErr.Kind)
	}
}

func TestShortenURL_SaveFails(t *testing.T) {
	storage := newFakeStorage()
	storage.saveErr = errors.New("connection refused")
	svc := NewService(storage)

	shortCode, err := svc.ShortenURL("https://example.com", 1)

	if !errors.Is(err, storage.saveErr) {
		t.Fatalf("ShortenURL() error = %v, want %v", err, storage.saveErr)
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		t.Errorf("ShortenURL() error is *apperrors.AppError, want the raw storage error unwrapped")
	}
	if shortCode != "" {
		t.Errorf("ShortenURL() short code = %q, want empty string on error", shortCode)
	}
}

func TestGetOriginalURL_StorageErrorPassthrough(t *testing.T) {
	storage := newFakeStorage()
	storage.getErr = errors.New("connection refused")
	svc := NewService(storage)

	_, err := svc.GetOriginalURL("anycode")

	if !errors.Is(err, storage.getErr) {
		t.Fatalf("GetOriginalURL() error = %v, want %v", err, storage.getErr)
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		t.Errorf("GetOriginalURL() error is *apperrors.AppError, want the raw storage error unwrapped")
	}
}

func TestDeleteURL_Success(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	shortCode, err := svc.ShortenURL("https://example.com", 1)
	if err != nil {
		t.Fatalf("ShortenURL() unexpected error: %v", err)
	}

	if err := svc.DeleteURL(shortCode, 1); err != nil {
		t.Fatalf("DeleteURL() unexpected error: %v", err)
	}

	if _, ok := storage.urls[shortCode]; ok {
		t.Error("DeleteURL() did not remove the URL from storage")
	}
}

func TestDeleteURL_NotFound(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	err := svc.DeleteURL("doesnotexist", 1)

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("DeleteURL() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.NotFound {
		t.Errorf("DeleteURL() Kind = %v, want NotFound", appErr.Kind)
	}
}

func TestDeleteURL_NotOwner(t *testing.T) {
	storage := newFakeStorage()
	svc := NewService(storage)

	shortCode, err := svc.ShortenURL("https://example.com", 1)
	if err != nil {
		t.Fatalf("ShortenURL() unexpected error: %v", err)
	}

	err = svc.DeleteURL(shortCode, 2)

	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("DeleteURL() error = %v, want *apperrors.AppError", err)
	}
	if appErr.Kind != apperrors.Forbidden {
		t.Errorf("DeleteURL() Kind = %v, want Forbidden", appErr.Kind)
	}

	if _, ok := storage.urls[shortCode]; !ok {
		t.Error("DeleteURL() removed the URL even though the caller wasn't the owner")
	}
}
