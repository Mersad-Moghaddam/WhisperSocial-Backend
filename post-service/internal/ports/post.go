package ports

import (
	"time"

	"gorm.io/gorm"
)

type Post struct {
	ID        uint           `gorm:"primaryKey;autoIncrement"`
	AuthorID  uint           `gorm:"not null;index"`
	Content   string         `gorm:"type:text;not null"`
	CreatedAt time.Time      `gorm:"autoCreatedTime;index"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
