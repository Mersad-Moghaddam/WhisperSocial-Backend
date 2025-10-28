package usecases

import (
	"log"
	"time"

	"github.com/Mersad-Moghaddam/fanout-worker/internal/ports"
)

type fanoutUC struct {
	repo      ports.FollowerRepository
	updater   ports.TimelineUpdater
	batchSize int
}

func NewFanoutUsecase(r ports.FollowerRepository, u ports.TimelineUpdater) ports.FanoutUsecase {
	return &fanoutUC{
		repo:      r,
		updater:   u,
		batchSize: 100, // Default batch size
	}
}

// ProcessPostCreated handles the fanout process when a new post is created
// This is the enhanced version that uses batching for better performance
func (uc *fanoutUC) ProcessPostCreated(postID uint, authorID uint) error {
	startTime := time.Now()

	// Get total follower count for batching
	followerCount, err := uc.repo.GetFollowerCount(authorID)
	if err != nil {
		log.Printf("Error getting follower count for user %d: %v", authorID, err)
		return err
	}

	if followerCount == 0 {
		log.Printf("User %d has no followers, skipping enhanced fanout", authorID)
		return nil
	}

	log.Printf("Starting enhanced fanout for post %d to %d followers", postID, followerCount)

	// Process in batches to avoid memory issues with large follower counts
	batchCount := (followerCount + uc.batchSize - 1) / uc.batchSize // Ceiling division

	for i := range make([]struct{}, batchCount) {
		offset := i * uc.batchSize

		followerIDs, err := uc.repo.GetFollowerIDsBatch(authorID, uc.batchSize, offset)
		if err != nil {
			log.Printf("Error getting followers batch %d for user %d: %v", i, authorID, err)
			return err
		}

		if len(followerIDs) == 0 {
			continue
		}

		err = uc.updater.AddPostToTimelines(postID, followerIDs)
		if err != nil {
			log.Printf("Error adding post %d to timelines: %v", postID, err)
			return err
		}

		log.Printf("Processed batch %d/%d for post %d (%d followers)",
			i+1, batchCount, postID, len(followerIDs))
	}

	duration := time.Since(startTime)
	log.Printf("Completed enhanced fanout for post %d to %d followers in %v",
		postID, followerCount, duration)

	return nil
}
