package migrations

import (
      "github.com/go-gormigrate/gormigrate/v2"
      "gorm.io/gorm"
      "url_shortener/internal/models"
)

var AddUserIDToURLs = &gormigrate.Migration{
      ID: "20260906_add_user_id_to_urls",
      Migrate: func(tx *gorm.DB) error {
              return tx.AutoMigrate(&models.URL{})
      },
      Rollback: func(tx *gorm.DB) error {
              return tx.Migrator().DropColumn(&models.URL{}, "UserID")
      },
}