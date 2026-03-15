package ports

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string    `json:"email" gorm:"type:varchar(100);unique;not null"`
	Password  string    `json:"-" gorm:"type:varchar(100);not null"`
	Role      string    `json:"role" gorm:"type:varchar(20);not null;default:user"`
	Status    string    `json:"status" gorm:"type:varchar(20);not null;default:active"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

type Post struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	AuthorID  uint           `json:"author_id"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
	Author    User           `json:"author" gorm:"foreignKey:AuthorID"`
}
