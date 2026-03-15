package ports

import "time"

type UserFilters struct {
	Status        string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	Limit         int
	Offset        int
}

type PostFilters struct {
	StartDate *time.Time
	EndDate   *time.Time
	UserID    uint
	Limit     int
	Offset    int
}

type AdminRepository interface {
	ListUsers(filter UserFilters) ([]User, error)
	GetUserByID(id uint) (*User, error)
	UpdateUserStatus(id uint, status string) error
	ListPosts(filter PostFilters) ([]Post, error)
	GetPostByID(id uint) (*Post, error)
	DeletePost(id uint) error
	CountStats() (map[string]int64, error)
	ListFollowerIDs(authorID uint) ([]uint, error)
}

type AdminUsecase interface {
	ListUsers(filter UserFilters) ([]User, error)
	GetUserByID(id uint) (*User, error)
	UpdateUserStatus(id uint, status string) error
	ListPosts(filter PostFilters) ([]Post, error)
	GetPostByID(id uint) (*Post, error)
	DeletePost(id uint) error
	Stats() (map[string]int64, error)
}
