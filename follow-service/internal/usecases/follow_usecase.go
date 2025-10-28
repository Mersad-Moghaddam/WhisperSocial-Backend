package usecases

import (
	"fmt"

	"github.com/Mersad-Moghaddam/follow-service/internal/ports"
)

type followUsecase struct {
	repo ports.FollowerRepository
}

func NewFollowUsecase(repo ports.FollowerRepository) ports.FollowUsecase {
	return &followUsecase{
		repo: repo,
	}
}

// Follow adds a follower relationship between FollowerID and UserID.
// Prevents self-following and duplicate follow relationships.
func (uc *followUsecase) Follow(req ports.FollowRequest) error {
	// Prevent self-following
	if req.UserID == req.FollowerID {
		return fmt.Errorf("cannot follow yourself")
	}

	// Check if already following
	isFollowing, err := uc.repo.IsFollowing(req.UserID, req.FollowerID)
	if err != nil {
		return err
	}

	if isFollowing {
		return fmt.Errorf("already following this user")
	}

	return uc.repo.Follow(req.UserID, req.FollowerID)
}

// Unfollow removes a follower relationship between FollowerID and UserID.
// Prevents self-unfollowing and ensures the follow relationship exists.
func (uc *followUsecase) Unfollow(req ports.FollowRequest) error {
	// Prevent self-unfollowing
	if req.UserID == req.FollowerID {
		return fmt.Errorf("cannot unfollow yourself")
	}

	// Check if actually following
	isFollowing, err := uc.repo.IsFollowing(req.UserID, req.FollowerID)
	if err != nil {
		return err
	}

	if !isFollowing {
		return fmt.Errorf("not following this user")
	}

	return uc.repo.Unfollow(req.UserID, req.FollowerID)
}

// GetFollowers returns the IDs of all users following the given userID.
func (uc *followUsecase) GetFollowers(userID uint) ([]uint, error) {
	return uc.repo.GetFollowers(userID)
}

// GetFollowing returns the IDs of all users that the given userID is following.
func (uc *followUsecase) GetFollowing(userID uint) ([]uint, error) {
	return uc.repo.GetFollowing(userID)
}

// IsFollowing checks if targetUserID is following userID.
func (uc *followUsecase) IsFollowing(userID, targetUserID uint) (bool, error) {
	return uc.repo.IsFollowing(targetUserID, userID)
}

// GetFollowStats returns the follower and following counts for a given userID.
func (uc *followUsecase) GetFollowStats(userID uint) (followers int64, following int64, error error) {
	followers, err := uc.repo.GetFollowersCount(userID)
	if err != nil {
		return 0, 0, err
	}

	following, err = uc.repo.GetFollowingCount(userID)
	if err != nil {
		return 0, 0, err
	}

	return followers, following, nil
}
