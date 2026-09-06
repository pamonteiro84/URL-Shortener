package service

import (
	"errors"
	"fmt"
	"url_shortener/internal/apperrors"
	"url_shortener/internal/models"
	"url_shortener/internal/shortcode"
	"url_shortener/internal/storage"

	"gorm.io/gorm"
)

var (
	errAlreadyExists = errors.New("url já tem shortcode")
	errNotOwner      = errors.New("não é o dono deste URL")
)

type Service struct {
	storage storage.URLRepository
}

func NewService(storage storage.URLRepository) *Service {
	return &Service{storage: storage}
}

func (s *Service) ShortenURL(originalURL string, userID uint) (string, error) {
	shortCode := shortcode.Generate(fmt.Sprintf("%d:%s", userID, originalURL))

	if existingURL, err := s.GetOriginalURL(shortCode); err == nil &&
		existingURL == originalURL {
		return shortCode, &apperrors.AppError{
			Kind: apperrors.AlreadyExists,
			Err:  errAlreadyExists,
		}
	}
	url := &models.URL{
		OriginalURL: originalURL,
		ShortCode:   shortCode,
		UserID:      userID,
	}
	if err := s.storage.Save(url); err != nil {
		return "", err
	}
	return shortCode, nil
}

func (s *Service) GetOriginalURL(shortCode string) (string, error) {
	url, err := s.storage.GetByShortURL(shortCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", &apperrors.AppError{
				Kind: apperrors.NotFound,
				Err:  err,
			}
		}
		return "", err
	}
	return url.OriginalURL, nil
}

func (s *Service) DeleteURL(shortCode string, userID uint) error {
	url, err := s.storage.GetByShortURL(shortCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &apperrors.AppError{
				Kind: apperrors.NotFound,
				Err:  err,
			}
		}
		return err
	}

	if url.UserID != userID {
		return &apperrors.AppError{
			Kind: apperrors.Forbidden,
			Err:  errNotOwner,
		}
	}

	if err := s.storage.Delete(shortCode); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &apperrors.AppError{
				Kind: apperrors.NotFound,
				Err:  err,
			}
		}
		return err
	}
	return nil
}
