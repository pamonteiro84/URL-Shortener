package service

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"url_shortener/internal/apperrors"
	"url_shortener/internal/auth"
	"url_shortener/internal/models"
	"url_shortener/internal/storage"
)

var (
      errEmailAlreadyExists  = errors.New("email já registado")
      errInvalidCredentials  = errors.New("credenciais inválidas")
      errInvalidRefreshToken = errors.New("refresh token inválido ou expirado")
)

type AuthService struct {
      users      storage.UserRepository
      redis      *redis.Client
      jwtSecret  []byte
      accessTTL  time.Duration
      refreshTTL time.Duration
}

func NewAuthService(users storage.UserRepository, redisClient *redis.Client, jwtSecret []byte, accessTTL, refreshTTL time.Duration) *AuthService {
      return &AuthService{
              users:      users,
              redis:      redisClient,
              jwtSecret:  jwtSecret,
              accessTTL:  accessTTL,
              refreshTTL: refreshTTL,
      }
}

func (s *AuthService) Register(email, password string) error {
      if _, err := s.users.GetByEmail(email); err == nil {
              return &apperrors.AppError{Kind: apperrors.AlreadyExists, Err: errEmailAlreadyExists}
      }

      hash, err := auth.HashPassword(password)
      if err != nil {
              return err
      }

      return s.users.Save(&models.User{Email: email, PasswordHash: hash})
}

func (s *AuthService) Login(email, password string) (string, string, error) {
      user, err := s.users.GetByEmail(email)
      if err != nil {
              return "", "", &apperrors.AppError{Kind: apperrors.Unauthorized, Err: errInvalidCredentials}
      }

      ok, err := auth.VerifyPassword(user.PasswordHash, password)
      if err != nil {
              return "", "", err
      }
      if !ok {
              return "", "", &apperrors.AppError{Kind: apperrors.Unauthorized, Err: errInvalidCredentials}
      }

      accessToken, err := auth.GenerateAccessToken(user.ID, s.jwtSecret, s.accessTTL)
      if err != nil {
              return "", "", err
      }

      refreshToken, err := auth.GenerateRefreshToken()
      if err != nil {
              return "", "", err
      }

      if err := s.redis.Set(context.Background(), "refresh:"+refreshToken, user.ID, s.refreshTTL).Err(); err != nil {
      return "", "", err
}

      return accessToken, refreshToken, nil
}

func (s *AuthService) Refresh(refreshToken string) (string, string, error) {
      ctx := context.Background()
      key := "refresh:" + refreshToken

      userIDStr, err := s.redis.Get(ctx, key).Result()
      if err != nil {
              return "", "", &apperrors.AppError{Kind: apperrors.Unauthorized, Err: errInvalidRefreshToken}
      }

      userID, err := strconv.ParseUint(userIDStr, 10, 64)
      if err != nil {
              return "", "", err
      }

      s.redis.Del(ctx, key)

      newRefreshToken, err := auth.GenerateRefreshToken()
      if err != nil {
              return "", "", err
      }
      if err := s.redis.Set(ctx, "refresh:"+newRefreshToken, uint(userID), s.refreshTTL).Err(); err != nil {
              return "", "", err
      }

      newAccessToken, err := auth.GenerateAccessToken(uint(userID), s.jwtSecret, s.accessTTL)
      if err != nil {
              return "", "", err
      }

      return newAccessToken, newRefreshToken, nil
}

func (s *AuthService) Logout(refreshToken string) error {
      return s.redis.Del(context.Background(), "refresh:"+refreshToken).Err()
}