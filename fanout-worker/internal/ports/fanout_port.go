package ports

type FollowerRepository interface {
	GetFollowers(authorID uint) ([]uint, error)
	GetFollowerIDs(userID uint) ([]uint, error)
	GetFollowerIDsBatch(userID uint, limit, offset int) ([]uint, error)
	GetFollowerCount(userID uint) (int, error)
}

type TimelineUpdater interface {
	AddPostToTimelines(postID uint, userIDs []uint) error
}

type FanoutUsecase interface {
	ProcessPostCreated(postID uint, authorID uint) error
}
