package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/joho/godotenv"

	"url_shortener/internal/cache"
	"url_shortener/internal/database"
	"url_shortener/internal/handlers"
	"url_shortener/internal/middleware"
	"url_shortener/internal/migrations"
	"url_shortener/internal/router"
	"url_shortener/internal/service"
	"url_shortener/internal/storage"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on environment variables already set")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatal(err)
	}

	m := gormigrate.New(db, gormigrate.DefaultOptions, migrations.All)
	if err := m.Migrate(); err != nil {
		log.Fatal(err)
	}

	redisClient, err := cache.Connect()
	if err != nil {
		log.Fatal(err)
	}

	repo := storage.NewGormURLRepository(db)
	cachedRepo := storage.NewCachedURLRepository(repo, redisClient, 10*time.Minute)
	svc := service.NewService(cachedRepo)
	h := handlers.NewHandler(svc)

	r := gin.Default()
	r.Use(middleware.RateLimit(redisClient, 10, time.Minute))
	router.Setup(r, h)
	r.Run(":8080")
}
