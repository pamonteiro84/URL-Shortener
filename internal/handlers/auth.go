package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"url_shortener/internal/service"
)

const refreshCookieName = "refresh_token"

type AuthHandler struct {
	service    *service.AuthService
	refreshTTL time.Duration
}

func NewAuthHandler(authService *service.AuthService, refreshTTL time.Duration) *AuthHandler {
	return &AuthHandler{service: authService, refreshTTL: refreshTTL}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.Register(req.Email, req.Password); err != nil {
		status, msg := statusFor(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	c.Status(http.StatusCreated)
}

type loginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	accessToken, refreshToken, err := h.service.Login(req.Email, req.Password)
	if err != nil {
		status, msg := statusFor(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	h.setRefreshCookie(c, refreshToken)
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken})
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing refresh token"})
		return
	}

	accessToken, newRefreshToken, err := h.service.Refresh(refreshToken)
	if err != nil {
		status, msg := statusFor(err)
		c.JSON(status, gin.H{"error": msg})
		return
	}

	h.setRefreshCookie(c, newRefreshToken)
	c.JSON(http.StatusOK, gin.H{"access_token": accessToken})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	refreshToken, err := c.Cookie(refreshCookieName)
	if err == nil {
		h.service.Logout(refreshToken)
	}

	c.SetCookie(refreshCookieName, "", -1, "/", "", false, true)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string) {
	c.SetCookie(refreshCookieName, token, int(h.refreshTTL.Seconds()), "/", "", false, true)
}
