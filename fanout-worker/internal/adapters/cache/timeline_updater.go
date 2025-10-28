package cache

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Mersad-Moghaddam/fanout-worker/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
)

type timelineUpdater struct {
	ctx context.Context
}

func NewTimelineUpdater() ports.TimelineUpdater {
	return &timelineUpdater{
		ctx: context.Background(),
	}
}

// AddPostToTimelines adds a post to multiple user timelines efficiently
func (t *timelineUpdater) AddPostToTimelines(postID uint, userIDs []uint) error {
	if len(userIDs) == 0 {
		return nil
	}

	timestamp := time.Now().Unix()
	pipe := config.RedisClient.Pipeline()

	for _, userID := range userIDs {
		key := fmt.Sprintf("timeline:%d", userID)

		// Add to sorted set with timestamp as score
		pipe.ZAdd(t.ctx, key, config.Z(postID, timestamp))

		// Trim timeline to keep only recent posts (last 1000)
		pipe.ZRemRangeByRank(t.ctx, key, 0, -1001)

		// Set expiration (7 days)
		pipe.Expire(t.ctx, key, 7*24*time.Hour)
	}

	// Execute all Redis commands in a single pipeline
	_, err := pipe.Exec(t.ctx)
	if err != nil {
		log.Printf("Error updating Redis timeline cache: %v", err)
		return err
	}

	return nil
}
