package ports

import "time"

// Post represents a post in the system
type Post struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index:idx_user"`
	Content   string
	CreatedAt time.Time
}

// Timeline represents a user's timeline entry
type Timeline struct {
	ID        uint      `gorm:"primaryKey"`
	UserID    uint      `gorm:"index:idx_timeline_user"`
	PostID    uint      `gorm:"index:idx_timeline_post"`
	CreatedAt time.Time
}