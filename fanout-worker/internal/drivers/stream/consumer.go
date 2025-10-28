package stream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Mersad-Moghaddam/fanout-worker/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/redis/go-redis/v9"
)

func StartConsumer(uc ports.FanoutUsecase) {
	stream := "post_created_stream"
	group := "fanout_group"
	consumer := fmt.Sprintf("fanout_worker_%d", time.Now().UnixNano())

	// Create consumer group if it doesn’t exist yet
	if err := config.RedisClient.XGroupCreateMkStream(config.Ctx, stream, group, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			log.Fatalf("failed to create consumer group: %v", err)
		}
	}

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Listen for OS shutdown signals (Ctrl+C or SIGTERM)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Goroutine to handle shutdown signal
	go func() {
		<-sigChan
		log.Println("Received shutdown signal, stopping consumer gracefully...")
		cancel()
	}()

	// Continuous loop to read from Redis Stream
	for {
		select {
		// Exit loop when context is canceled
		case <-ctx.Done():
			log.Println("Consumer stopped.")
			return

		default:
			// Read messages from Redis Stream
			res, err := config.RedisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    group,
				Consumer: consumer,
				Streams:  []string{stream, ">"},
				Count:    10,
				Block:    0,
			}).Result()

			// Handle read errors
			if err != nil {
				if errors.Is(err, context.Canceled) {
					log.Println("Context canceled, stopping reader loop.")
					return
				}
				log.Println("Stream read error:", err)
				continue
			}

			// Skip if no messages returned
			if len(res) == 0 || len(res[0].Messages) == 0 {
				continue
			}

			// Process each message from the stream
			for _, msg := range res[0].Messages {
				// Extract fields from message
				pidRaw, ok1 := msg.Values["post_id"]
				aidRaw, ok2 := msg.Values["author_id"]

				postID, err1 := parseUintFlexible(pidRaw)
				authorID, err2 := parseUintFlexible(aidRaw)

				// If fields are missing or invalid, acknowledge and skip
				if !ok1 || !ok2 || err1 != nil || err2 != nil {
					log.Println("invalid or missing fields, acknowledging:", msg.ID)
					_ = config.RedisClient.XAck(config.Ctx, stream, group, msg.ID).Err()
					continue
				}

				// Run business logic: fanout post to followers
				if err := uc.ProcessPostCreated(uint(postID), uint(authorID)); err != nil {
					log.Println("Fanout error:", err)
					continue
				}

				// Acknowledge message only after successful processing
				if err := config.RedisClient.XAck(ctx, stream, group, msg.ID).Err(); err != nil {
					log.Println("XAck failed:", err)
				}
			}
		}
	}
}

// parseUintFlexible converts different numeric types into uint64 safely.
func parseUintFlexible(v any) (uint64, error) {
	switch t := v.(type) {
	case string:
		return strconv.ParseUint(t, 10, 64)
	case int64:
		if t < 0 {
			return 0, fmt.Errorf("negative int64")
		}
		return uint64(t), nil
	case float64:
		if t < 0 {
			return 0, fmt.Errorf("negative float64")
		}
		return uint64(t), nil
	default:
		return 0, fmt.Errorf("unsupported type: %T", v)
	}
}
