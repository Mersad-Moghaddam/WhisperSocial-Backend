package database

import (
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/Mersad-Moghaddam/timeline-service/internal/ports"
)

type postRepo struct{}

func NewPostRepository() ports.PostRepository {
	return &postRepo{}
}

// GetPostsByIDs fetches posts from the database whose IDs match the given slice.
func (r *postRepo) GetPostsByIDs(ids []string) ([]ports.Post, error) {
	var posts []ports.Post
	//query all posts with IDs in the provided slice
	if err := config.DB.Where("id IN ?", ids).Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

// GetFollowers returns a slice of user IDs who are following the given authorID.
func (r *postRepo) GetFollowers(authorID uint) ([]uint, error) {
	var followers []ports.Follower
	if err := config.DB.Where("user_id = ?", authorID).Find(&followers).Error; err != nil {
		return nil, err
	}

	var ids []uint
	for _, f := range followers {
		ids = append(ids, f.FollowerID)
	}
	return ids, nil
}

// GetFollowing returns a slice of user IDs whom the given userID is following.
func (r *postRepo) GetFollowing(userID uint) ([]uint, error) {
	var following []ports.Follower
	if err := config.DB.Where("follower_id = ?", userID).Find(&following).Error; err != nil {
		return nil, err
	}

	var ids []uint
	for _, f := range following {
		ids = append(ids, f.UserID)
	}
	return ids, nil
}
