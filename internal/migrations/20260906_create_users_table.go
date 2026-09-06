package migrations

import (
	  "github.com/go-gormigrate/gormigrate/v2"
	  "gorm.io/gorm"
	  "url_shortener/internal/models"
)

var CreateUsersTable = &gormigrate.Migration{
	  ID: "20260906_create_users_table",
	  Migrate: func(tx *gorm.DB) error {
			  return tx.AutoMigrate(&models.User{})
	  },
	  Rollback: func(tx *gorm.DB) error {
			  return tx.Migrator().DropTable("users")
	  },
}