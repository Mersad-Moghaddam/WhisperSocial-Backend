package ports

import "time"

type Post struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	AuthorID  uint      `gorm:"not null"`
	Content   string    `gorm:"type:text;not null"`
	CreatedAt time.Time `gorm:"autoCreatedTime"`
}
