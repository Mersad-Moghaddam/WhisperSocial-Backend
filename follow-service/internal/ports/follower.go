package ports

type Follower struct {
	ID         uint `gorm:"primaryKey;autoIncrement"`
	UserID     uint `gorm:"not null;index:idx_user_follower,unique"`
	FollowerID uint `gorm:"not null;index:idx_user_follower,unique"`
}
