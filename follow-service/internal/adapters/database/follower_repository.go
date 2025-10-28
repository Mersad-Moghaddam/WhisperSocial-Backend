package database

import (
	"github.com/Mersad-Moghaddam/follow-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type followerRepo struct{}

func NewFollowerRepository() ports.FollowerRepository {
	return &followerRepo{}
}

// Follow adds a follower relationship where followerID follows userID.
func (r *followerRepo) Follow(userID, followerID uint) error {
	return config.DB.Create(&ports.Follower{
		UserID:     userID,
		FollowerID: followerID,
	}).Error
}

// Unfollow removes the follower relationship where followerID follows userID.
func (r *followerRepo) Unfollow(userID, followerID uint) error {
	return config.DB.Where("user_id = ? AND follower_id = ?", userID, followerID).
		Delete(&ports.Follower{}).Error
}

// IsFollowing checks if followerID is following userID.
func (r *followerRepo) IsFollowing(userID, followerID uint) (bool, error) {
	var count int64
	err := config.DB.Model(&ports.Follower{}).
		Where("user_id = ? AND follower_id = ?", userID, followerID).
		Count(&count).Error
	return count > 0, err
}

// GetFollowers returns a slice of user IDs who are following the given authorID.
func (r *followerRepo) GetFollowers(authorID uint) ([]uint, error) {
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
func (r *followerRepo) GetFollowing(userID uint) ([]uint, error) {
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

// GetFollowersCount returns the total number of followers a user has.
func (r *followerRepo) GetFollowersCount(userID uint) (int64, error) {
	var count int64
	err := config.DB.Model(&ports.Follower{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// GetFollowingCount returns the total number of users that the given user is following.
func (r *followerRepo) GetFollowingCount(userID uint) (int64, error) {
	var count int64
	err := config.DB.Model(&ports.Follower{}).
		Where("follower_id = ?", userID).
		Count(&count).Error
	return count, err
}
