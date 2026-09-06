package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"url_shortener/internal/apperrors"
	"url_shortener/internal/service"
)

type Handler struct {
	service *service.Service
}

func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

func statusFor(err error) (int, string) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		switch appErr.Kind {
		case apperrors.NotFound:
			return http.StatusNotFound, "not found"
		case apperrors.AlreadyExists:
			return http.StatusConflict, "already exists"
		case apperrors.Unauthorized:
			return http.StatusUnauthorized, "unauthorized"
		case apperrors.Forbidden:
			return http.StatusForbidden, "forbidden"
		default:
			return http.StatusInternalServerError, "internal error"
		}
	}
	return http.StatusInternalServerError, "internal error"
}

type shortenRequest struct {
	URL string `json:"url" binding:"required"`
}

func (h *Handler) Shorten(c *gin.Context) {
	var req shortenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	userID := c.MustGet("userID").(uint)
	shortCode, err := h.service.ShortenURL(req.URL, userID)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind == apperrors.AlreadyExists {
		c.JSON(http.StatusOK, gin.H{"short_code": shortCode})
		return
	}
	if err != nil {
		status, msg := statusFor(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"short_code": shortCode})
}

func (h *Handler) Redirect(c *gin.Context) {
	code := c.Param("code")

	originalURL, err := h.service.GetOriginalURL(code)
	if err != nil {
		status, msg := statusFor(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.Redirect(http.StatusFound, originalURL)
}

func (h *Handler) Delete(c *gin.Context) {
	code := c.Param("code")
	userID := c.MustGet("userID").(uint)

	if err := h.service.DeleteURL(code, userID); err != nil {
		status, msg := statusFor(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.Status(http.StatusNoContent)
}
