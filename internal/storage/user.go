package storage

import (
	  "gorm.io/gorm"
	  "url_shortener/internal/models"
)

type UserRepository interface {
      Save(u *models.User) error
      GetByEmail(email string) (*models.User, error)
}

type gormUserRepository struct {
      db *gorm.DB
}

func NewGormUserRepository(db *gorm.DB) UserRepository {
      return &gormUserRepository{db: db}
}

func (r *gormUserRepository) Save(u *models.User) error {
      return r.db.Create(u).Error
}

func (r *gormUserRepository) GetByEmail(email string) (*models.User, error) {
      var u models.User
      if err := r.db.Where("email = ?", email).First(&u).Error; err != nil {
              return nil, err
      }
      return &u, nil
}