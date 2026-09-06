package models

import "time"

type URL struct {
	ID          uint
	ShortCode   string    `gorm:"uniqueIndex;not null;size:10"`
	OriginalURL string    `gorm:"not null"`
	UserID uint `gorm:"index"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
}
