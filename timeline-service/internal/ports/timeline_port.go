package ports

// TimelineResponse represents the paginated response for a user's timeline.
// - Posts: list of posts returned in this page
// - NextCursor: cursor value for fetching the next page of posts
type TimelineResponse struct {
	Posts      []Post `json:"posts"`
	NextCursor int64  `json:"next_cursor"`
}

// defines the interface for interacting with a cached timeline.
type TimelineCache interface {
	// GetPostIDs returns a slice of post IDs for a given user, limited by 'limit',
	// starting from 'cursor'. It also returns the next cursor for pagination.
	GetPostIDs(userID uint, limit int64, cursor int64) ([]string, int64, error)
}

// defines the interface for fetching posts from the persistent store.
type PostRepository interface {
	// GetPostsByIDs returns full Post objects corresponding to the given slice of post IDs.
	GetPostsByIDs(ids []string) ([]Post, error)
	GetFollowers(userID uint) ([]uint, error)
	GetFollowing(userID uint) ([]uint, error)
}

// defines the business logic for fetching a user's timeline.
type GetTimelineUsecase interface {
	// Get returns a TimelineResponse containing posts and the next cursor for pagination.
	Get(userID uint, limit int64, cursor int64) (TimelineResponse, error)
	GetFollowers(userID uint) ([]uint, error)
	GetFollowing(userID uint) ([]uint, error)
}
