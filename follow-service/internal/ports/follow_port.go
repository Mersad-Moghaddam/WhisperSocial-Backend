package ports

type FollowRequest struct {
	UserID     uint `json:"user_id"`     // the ID of the user being followed or unfollowed
	FollowerID uint `json:"follower_id"` // the ID of the user performing the follow/unfollow
}

// FollowerRepository defines the data layer interface for follower relationships.
type FollowerRepository interface {
	Follow(userID, followerID uint) error
	Unfollow(userID, followerID uint) error
	IsFollowing(userID, followerID uint) (bool, error)
	GetFollowers(authorID uint) ([]uint, error)
	GetFollowing(userID uint) ([]uint, error)
	GetFollowersCount(userID uint) (int64, error)
	GetFollowingCount(userID uint) (int64, error)
}

// FollowUsecase defines the business logic interface for follow/unfollow actions.
type FollowUsecase interface {
	Follow(req FollowRequest) error
	Unfollow(req FollowRequest) error
	GetFollowers(userID uint) ([]uint, error)
	GetFollowing(userID uint) ([]uint, error)
	IsFollowing(userID, targetUserID uint) (bool, error)
	GetFollowStats(userID uint) (followers int64, following int64, error error)
}
