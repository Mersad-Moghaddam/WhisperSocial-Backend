package usecases

import "github.com/Mersad-Moghaddam/timeline-service/internal/ports"

type getTimelineUC struct {
	cache ports.TimelineCache
	repo  ports.PostRepository
}

func NewGetTimelineUsecase(c ports.TimelineCache, r ports.PostRepository) ports.GetTimelineUsecase {
	return &getTimelineUC{cache: c, repo: r}
}

// Get retrieves a user's timeline posts with cursor-based pagination.
// It first fetches post IDs from the cache and then retrieves the full post details from the repository.
func (uc *getTimelineUC) Get(userID uint, limit int64, cursor int64) (ports.TimelineResponse, error) {
	// Fetch post IDs from Redis cache
	postIDs, nextCursor, err := uc.cache.GetPostIDs(userID, limit, cursor)
	if err != nil {
		return ports.TimelineResponse{}, err
	}
	// If no post IDs are returned, return an empty timeline response
	if len(postIDs) == 0 {
		return ports.TimelineResponse{Posts: []ports.Post{}, NextCursor: nextCursor}, nil
	}

	// Fetch full post objects from the repository
	posts, err := uc.repo.GetPostsByIDs(postIDs)
	if err != nil {
		return ports.TimelineResponse{}, err
	}

	return ports.TimelineResponse{Posts: posts, NextCursor: nextCursor}, nil
}
func (uc *getTimelineUC) GetFollowers(userID uint) ([]uint, error) {
	return uc.repo.GetFollowers(userID)
}
func (uc *getTimelineUC) GetFollowing(userID uint) ([]uint, error) {
	return uc.repo.GetFollowing(userID)
}
