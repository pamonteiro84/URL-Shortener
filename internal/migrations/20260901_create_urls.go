package migrations

import (
      "github.com/go-gormigrate/gormigrate/v2"
      "gorm.io/gorm"
      "url_shortener/internal/models"
)

var CreateURLsTable = &gormigrate.Migration{
      ID: "20260901_create_urls_table",
      Migrate: func(tx *gorm.DB) error {
              return tx.AutoMigrate(&models.URL{})
      },
      Rollback: func(tx *gorm.DB) error {
              return tx.Migrator().DropTable("urls")
      },
}