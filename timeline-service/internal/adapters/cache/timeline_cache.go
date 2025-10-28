package cache

import (
	"fmt"
	"strconv"

	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/redis/go-redis/v9"
)

type timelineCache struct{}

func NewTimelineCache() *timelineCache {
	return &timelineCache{}
}

// GetPostIDs retrieves a slice of post IDs from a user's timeline in Redis.
// Implements cursor-based pagination using ZREVRANGEBYSCORE to get posts in descending order of timestamp.
func (c *timelineCache) GetPostIDs(userID uint, limit int64, cursor int64) ([]string, int64, error) {
	key := fmt.Sprintf("timeline:%d", userID)
	// By default, get all posts from +inf (latest posts)
	max := "+inf"
	if cursor > 0 {
		// Use exclusive upper bound to avoid repeating the last post from previous page
		max = "(" + strconv.FormatInt(cursor, 10)
	}
	// Query Redis sorted set for post IDs in descending order of score (timestamp)
	result, err := config.RedisClient.ZRevRangeByScore(config.Ctx, key, &redis.ZRangeBy{
		Max:    max,
		Min:    "-inf",
		Offset: 0,
		Count:  limit,
	}).Result()
	if err != nil {
		return nil, 0, err
	}

	var nextCursor int64 = 0
	if len(result) > 0 {
		// Set nextCursor to the timestamp of the last post returned
		ts := config.RedisClient.ZScore(config.Ctx, key, result[len(result)-1]).Val()
		nextCursor = int64(ts)
	}

	return result, nextCursor, nil
}
