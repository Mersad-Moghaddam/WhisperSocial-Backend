package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Mersad-Moghaddam/fanout-worker/internal/adapters/cache"
	db "github.com/Mersad-Moghaddam/fanout-worker/internal/adapters/database"
	"github.com/Mersad-Moghaddam/fanout-worker/internal/drivers/stream"
	"github.com/Mersad-Moghaddam/fanout-worker/internal/ports"
	"github.com/Mersad-Moghaddam/fanout-worker/internal/usecases"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	// Load global .env file first
	if err := godotenv.Load("../.env"); err != nil {
		log.Printf("Warning: Error loading global .env file: %v", err)
	}
	// Load local .env file to override with service-specific values
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: Error loading local .env file: %v", err)
	}

	// Initialize database
	config.InitDB(&ports.Follower{})

	// Initialize Redis for streaming
	config.InitRedis()

	// Initialize repositories and services
	followerRepo := db.NewFollowerRepository()
	timelineUpdater := cache.NewTimelineUpdater()

	// Initialize use cases
	fanoutUC := usecases.NewFanoutUsecase(followerRepo, timelineUpdater)

	log.Println("Starting Redis Streams fanout worker...")
	log.Println("Listening for events on stream: post_created_stream")

	// Start the Redis Streams consumer in a goroutine
	go stream.StartConsumer(fanoutUC)

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Shutdown signal received...")

	log.Println("Shutting down fanout worker...")

	// Close database connections
	if config.DB != nil {
		if sqlDB, err := config.DB.DB(); err == nil {
			sqlDB.Close()
		}
	}

	// Close Redis connections
	if config.RedisClient != nil {
		config.RedisClient.Close()
	}

	log.Println("Fanout worker has been shut down completely")
}
