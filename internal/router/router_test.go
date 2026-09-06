package router

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"url_shortener/internal/handlers"
	"url_shortener/internal/middleware"
	"url_shortener/internal/models"
	"url_shortener/internal/service"
)

type fakeStorage struct {
	urls map[string]*models.URL
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{urls: make(map[string]*models.URL)}
}

func (f *fakeStorage) Save(u *models.URL) error {
	f.urls[u.ShortCode] = u
	return nil
}

func (f *fakeStorage) GetByShortURL(shortURL string) (*models.URL, error) {
	u, ok := f.urls[shortURL]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (f *fakeStorage) Delete(shortCode string) error {
	if _, ok := f.urls[shortCode]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.urls, shortCode)
	return nil
}

type fakeUserRepository struct {
	users  map[string]*models.User
	nextID uint
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{users: make(map[string]*models.User)}
}

func (f *fakeUserRepository) Save(u *models.User) error {
	f.nextID++
	u.ID = f.nextID
	f.users[u.Email] = u
	return nil
}

func (f *fakeUserRepository) GetByEmail(email string) (*models.User, error) {
	u, ok := f.users[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	jwtSecret := []byte("test-secret")
	accessTTL := 15 * time.Minute
	refreshTTL := 7 * 24 * time.Hour

	storage := newFakeStorage()
	svc := service.NewService(storage)
	h := handlers.NewHandler(svc)

	users := newFakeUserRepository()
	authSvc := service.NewAuthService(users, redisClient, jwtSecret, accessTTL, refreshTTL)
	ah := handlers.NewAuthHandler(authSvc, refreshTTL)

	requireAuth := middleware.RequireAuth(jwtSecret)

	r := gin.New()
	Setup(r, h, ah, requireAuth)
	return r
}

func registerAndLogin(t *testing.T, r *gin.Engine, email, password string) string {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"email": email, "password": password})

	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want %d, body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d, body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid login JSON response: %v", err)
	}
	return resp["access_token"]
}

func shorten(t *testing.T, r *gin.Engine, token, url string) *httptest.ResponseRecorder {
	t.Helper()

	body, _ := json.Marshal(map[string]string{"url": url})
	req := httptest.NewRequest(http.MethodPost, "/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestShorten_NewURL(t *testing.T) {
	r := newTestRouter(t)
	token := registerAndLogin(t, r, "alice@example.com", "hunter2")

	w := shorten(t, r, token, "https://example.com")

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if resp["short_code"] == "" {
		t.Error("response missing short_code")
	}
}

func TestShorten_Repeated(t *testing.T) {
	r := newTestRouter(t)
	token := registerAndLogin(t, r, "alice@example.com", "hunter2")

	first := shorten(t, r, token, "https://example.com")
	second := shorten(t, r, token, "https://example.com")

	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", second.Code, http.StatusOK, second.Body.String())
	}

	var firstResp, secondResp map[string]string
	json.Unmarshal(first.Body.Bytes(), &firstResp)
	json.Unmarshal(second.Body.Bytes(), &secondResp)

	if secondResp["short_code"] != firstResp["short_code"] {
		t.Errorf("short_code = %q, want %q (same as first)", secondResp["short_code"], firstResp["short_code"])
	}
}

func TestShorten_WithoutToken(t *testing.T) {
	r := newTestRouter(t)

	body, _ := json.Marshal(map[string]string{"url": "https://example.com"})
	req := httptest.NewRequest(http.MethodPost, "/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRedirect_Found(t *testing.T) {
	r := newTestRouter(t)
	token := registerAndLogin(t, r, "alice@example.com", "hunter2")

	shortenResp := shorten(t, r, token, "https://example.com")
	var resp map[string]string
	json.Unmarshal(shortenResp.Body.Bytes(), &resp)
	shortCode := resp["short_code"]

	req := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if got := w.Header().Get("Location"); got != "https://example.com" {
		t.Errorf("Location = %q, want %q", got, "https://example.com")
	}
}

func TestRedirect_NotFound(t *testing.T) {
	r := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/doesnotexist", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDelete_Found(t *testing.T) {
	r := newTestRouter(t)
	token := registerAndLogin(t, r, "alice@example.com", "hunter2")

	shortenResp := shorten(t, r, token, "https://example.com")
	var resp map[string]string
	json.Unmarshal(shortenResp.Body.Bytes(), &resp)
	shortCode := resp["short_code"]

	req := httptest.NewRequest(http.MethodDelete, "/"+shortCode, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNoContent)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/"+shortCode, nil)
	getW := httptest.NewRecorder()
	r.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusNotFound {
		t.Fatalf("after delete, GET status = %d, want %d", getW.Code, http.StatusNotFound)
	}
}

func TestDelete_NotFound(t *testing.T) {
	r := newTestRouter(t)
	token := registerAndLogin(t, r, "alice@example.com", "hunter2")

	req := httptest.NewRequest(http.MethodDelete, "/doesnotexist", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDelete_NotOwner(t *testing.T) {
	r := newTestRouter(t)
	ownerToken := registerAndLogin(t, r, "alice@example.com", "hunter2")
	otherToken := registerAndLogin(t, r, "bob@example.com", "hunter3")

	shortenResp := shorten(t, r, ownerToken, "https://example.com")
	var resp map[string]string
	json.Unmarshal(shortenResp.Body.Bytes(), &resp)
	shortCode := resp["short_code"]

	req := httptest.NewRequest(http.MethodDelete, "/"+shortCode, nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}
