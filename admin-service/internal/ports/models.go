package ports

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type Post struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	AuthorID  uint           `json:"author_id"`
	Content   string         `json:"content"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-"`
	Author    User           `json:"author" gorm:"foreignKey:AuthorID"`
}
