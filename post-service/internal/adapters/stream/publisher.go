package stream

import (
	"strconv"

	"github.com/Mersad-Moghaddam/post-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/redis/go-redis/v9"
)

type publisher struct{}

func NewPublisher() ports.EventPublisher {
	return &publisher{}
}

// PublishPostCreated publishes a "post created" event to the Redis stream.
// Each event contains:
// - post_id: the unique identifier of the post (as a string)
// - author_id: the ID of the post's author (as a string)
// - timestamp: the creation time of the post (Unix timestamp)
// The event is added to the "post_created_stream".
func (p *publisher) PublishPostCreated(post ports.Post) error {
	data := map[string]any{
		"post_id":   strconv.FormatUint(uint64(post.ID), 10),
		"author_id": strconv.FormatUint(uint64(post.AuthorID), 10),
		"timestamp": post.CreatedAt.Unix(),
	}

	return config.RedisClient.XAdd(config.Ctx, &redis.XAddArgs{
		Stream: "post_created_stream",
		Values: data,
	}).Err()
}
