package storage

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"url_shortener/internal/models"
)

type fakeInnerRepo struct {
	urls        map[string]*models.URL
	getCalls    int
	saveCalls   int
	deleteCalls int
}

func newFakeInnerRepo() *fakeInnerRepo {
	return &fakeInnerRepo{urls: make(map[string]*models.URL)}
}

func (f *fakeInnerRepo) Save(u *models.URL) error {
	f.saveCalls++
	f.urls[u.ShortCode] = u
	return nil
}

func (f *fakeInnerRepo) GetByShortURL(shortURL string) (*models.URL, error) {
	f.getCalls++
	u, ok := f.urls[shortURL]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *fakeInnerRepo) Delete(shortCode string) error {
	f.deleteCalls++
	if _, ok := f.urls[shortCode]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.urls, shortCode)
	return nil
}

func newTestCachedRepo(t *testing.T) (URLRepository, *fakeInnerRepo) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	inner := newFakeInnerRepo()

	repo := NewCachedURLRepository(inner, client, time.Minute)
	return repo, inner
}

func TestCachedGetByShortURL_MissThenHit(t *testing.T) {
	repo, inner := newTestCachedRepo(t)
	inner.urls["abc123"] = &models.URL{ShortCode: "abc123", OriginalURL: "https://example.com"}

	got, err := repo.GetByShortURL("abc123")
	if err != nil {
		t.Fatalf("first GetByShortURL() unexpected error: %v", err)
	}
	if got.OriginalURL != "https://example.com" {
		t.Errorf("OriginalURL = %q, want %q", got.OriginalURL, "https://example.com")
	}
	if inner.getCalls != 1 {
		t.Fatalf("inner.getCalls after miss = %d, want 1", inner.getCalls)
	}

	got, err = repo.GetByShortURL("abc123")
	if err != nil {
		t.Fatalf("second GetByShortURL() unexpected error: %v", err)
	}
	if got.OriginalURL != "https://example.com" {
		t.Errorf("OriginalURL = %q, want %q", got.OriginalURL, "https://example.com")
	}
	if inner.getCalls != 1 {
		t.Errorf("inner.getCalls after hit = %d, want 1 (should not touch inner on cache hit)", inner.getCalls)
	}
}

func TestCachedGetByShortURL_NotFound(t *testing.T) {
	repo, inner := newTestCachedRepo(t)

	_, err := repo.GetByShortURL("doesnotexist")
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("GetByShortURL() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
	if inner.getCalls != 1 {
		t.Errorf("inner.getCalls = %d, want 1", inner.getCalls)
	}
}

func TestCachedSave_DoesNotPopulateCache(t *testing.T) {
	repo, inner := newTestCachedRepo(t)

	if err := repo.Save(&models.URL{ShortCode: "abc123", OriginalURL: "https://example.com"}); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}
	if inner.saveCalls != 1 {
		t.Fatalf("inner.saveCalls = %d, want 1", inner.saveCalls)
	}

	if _, err := repo.GetByShortURL("abc123"); err != nil {
		t.Fatalf("GetByShortURL() unexpected error: %v", err)
	}
	if inner.getCalls != 1 {
		t.Errorf("inner.getCalls = %d, want 1 (Save should not have pre-populated the cache)", inner.getCalls)
	}
}

func TestCachedDelete_InvalidatesCache(t *testing.T) {
	repo, inner := newTestCachedRepo(t)
	inner.urls["abc123"] = &models.URL{ShortCode: "abc123", OriginalURL: "https://example.com"}

	if _, err := repo.GetByShortURL("abc123"); err != nil {
		t.Fatalf("GetByShortURL() unexpected error: %v", err)
	}
	if inner.getCalls != 1 {
		t.Fatalf("inner.getCalls after warmup = %d, want 1", inner.getCalls)
	}

	if err := repo.Delete("abc123"); err != nil {
		t.Fatalf("Delete() unexpected error: %v", err)
	}

	_, err := repo.GetByShortURL("abc123")
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("GetByShortURL() after delete error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
	if inner.getCalls != 2 {
		t.Errorf("inner.getCalls after delete = %d, want 2 (cache should have been invalidated, forcing a miss)", inner.getCalls)
	}
}

func TestCachedDelete_NotFound(t *testing.T) {
	repo, inner := newTestCachedRepo(t)

	err := repo.Delete("doesnotexist")
	if err != gorm.ErrRecordNotFound {
		t.Fatalf("Delete() error = %v, want %v", err, gorm.ErrRecordNotFound)
	}
	if inner.deleteCalls != 1 {
		t.Errorf("inner.deleteCalls = %d, want 1", inner.deleteCalls)
	}
}
