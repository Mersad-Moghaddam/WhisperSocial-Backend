package db

import (
	"github.com/Mersad-Moghaddam/fanout-worker/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type followerRepo struct{}

func NewFollowerRepository() ports.FollowerRepository {
	return &followerRepo{}
}

// GetFollowers retrieves the IDs of all users who follow a given author.
// This is kept for backward compatibility
func (r *followerRepo) GetFollowers(authorID uint) ([]uint, error) {
	return r.GetFollowerIDs(authorID)
}

// GetFollowerIDs retrieves the IDs of all users who follow a given author.
func (r *followerRepo) GetFollowerIDs(authorID uint) ([]uint, error) {
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

// GetFollowerIDsBatch retrieves a batch of follower IDs with pagination
func (r *followerRepo) GetFollowerIDsBatch(authorID uint, limit, offset int) ([]uint, error) {
	var followers []ports.Follower
	if err := config.DB.Where("user_id = ?", authorID).Limit(limit).Offset(offset).Find(&followers).Error; err != nil {
		return nil, err
	}

	var ids []uint
	for _, f := range followers {
		ids = append(ids, f.FollowerID)
	}
	return ids, nil
}

// GetFollowerCount returns the total number of followers for a given user
func (r *followerRepo) GetFollowerCount(authorID uint) (int, error) {
	var count int64
	if err := config.DB.Model(&ports.Follower{}).Where("user_id = ?", authorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}
