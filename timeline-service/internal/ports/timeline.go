package ports

type Post struct {
	ID       uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	AuthorID uint   `gorm:"not null" json:"author_id"`
	Content  string `gorm:"type:text;not null" json:"content"`
}

type Follower struct {
	ID         uint `gorm:"primaryKey;autoIncrement"`
	UserID     uint `gorm:"not null;index:idx_user_follower,unique"`
	FollowerID uint `gorm:"not null;index:idx_user_follower,unique"`
}
