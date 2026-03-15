package ports

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Email     string    `gorm:"type:varchar(100);unique;not null"`
	Password  string    `gorm:"type:varchar(100);not null"`
	Role      string    `gorm:"type:varchar(20);not null;default:user"`
	Status    string    `gorm:"type:varchar(20);not null;default:active"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}
