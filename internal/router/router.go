package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"url_shortener/internal/handlers"
)

func Setup(r *gin.Engine, h *handlers.Handler, ah *handlers.AuthHandler, requireAuth gin.HandlerFunc) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.POST("/register", ah.Register)
	r.POST("/login", ah.Login)
	r.POST("/refresh", ah.Refresh)
	r.POST("/logout", ah.Logout)

	r.GET("/:code", h.Redirect)

	protected := r.Group("/")
	protected.Use(requireAuth)
	protected.POST("/shorten", h.Shorten)
	protected.DELETE("/:code", h.Delete)
}
