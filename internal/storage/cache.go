package storage

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"url_shortener/internal/models"
)

type cachedURLValue struct {
	OriginalURL string `json:"original_url"`
	UserID      uint   `json:"user_id"`
}

type cachedURLRepository struct {
	inner URLRepository
	redis *redis.Client
	ttl   time.Duration
}

func NewCachedURLRepository(inner URLRepository, redisClient *redis.Client, ttl time.Duration) URLRepository {
	return &cachedURLRepository{inner: inner, redis: redisClient, ttl: ttl}
}

func cacheKey(shortCode string) string {
	return "short:" + shortCode
}

func (r *cachedURLRepository) GetByShortURL(shortURL string) (*models.URL, error) {
	ctx := context.Background()
	key := cacheKey(shortURL)

	if raw, err := r.redis.Get(ctx, key).Result(); err == nil {
		var cached cachedURLValue
		if err := json.Unmarshal([]byte(raw), &cached); err == nil {
			return &models.URL{ShortCode: shortURL, OriginalURL: cached.OriginalURL, UserID: cached.UserID}, nil
		}
	}

	u, err := r.inner.GetByShortURL(shortURL)
	if err != nil {
		return nil, err
	}

	if raw, err := json.Marshal(cachedURLValue{OriginalURL: u.OriginalURL, UserID: u.UserID}); err == nil {
		if err := r.redis.Set(ctx, key, raw, r.ttl).Err(); err != nil {
			log.Printf("cache: failed to set %q: %v", key, err)
		}
	}

	return u, nil
}

func (r *cachedURLRepository) Save(u *models.URL) error {
	return r.inner.Save(u)
}

func (r *cachedURLRepository) Delete(shortCode string) error {
	if err := r.inner.Delete(shortCode); err != nil {
		return err
	}

	key := cacheKey(shortCode)
	if err := r.redis.Del(context.Background(), key).Err(); err != nil {
		log.Printf("cache: failed to invalidate %q: %v", key, err)
	}

	return nil
}
