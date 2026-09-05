package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func newTestRouter(t *testing.T, limit int64, window time.Duration) (*gin.Engine, *miniredis.Miniredis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	r := gin.New()
	r.Use(RateLimit(client, limit, window))
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return r, mr
}

func request(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.RemoteAddr = ip + ":12345"

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRateLimit_AllowsUpToLimit(t *testing.T) {
	r, _ := newTestRouter(t, 3, time.Minute)

	for i := 1; i <= 3; i++ {
		w := request(r, "1.2.3.4")
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i, w.Code, http.StatusOK)
		}
	}
}

func TestRateLimit_BlocksOverLimit(t *testing.T) {
	r, _ := newTestRouter(t, 3, time.Minute)

	for i := 1; i <= 3; i++ {
		request(r, "1.2.3.4")
	}

	w := request(r, "1.2.3.4")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("4th request: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimit_ResetsAfterWindow(t *testing.T) {
	r, mr := newTestRouter(t, 1, time.Minute)

	request(r, "1.2.3.4")
	w := request(r, "1.2.3.4")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("2nd request within window: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	mr.FastForward(time.Minute + time.Second)

	w = request(r, "1.2.3.4")
	if w.Code != http.StatusOK {
		t.Fatalf("request after window reset: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimit_TracksIPsIndependently(t *testing.T) {
	r, _ := newTestRouter(t, 1, time.Minute)

	w := request(r, "1.2.3.4")
	if w.Code != http.StatusOK {
		t.Fatalf("first IP's request: status = %d, want %d", w.Code, http.StatusOK)
	}

	w = request(r, "5.6.7.8")
	if w.Code != http.StatusOK {
		t.Fatalf("second IP's request: status = %d, want %d (should have its own quota)", w.Code, http.StatusOK)
	}
}
