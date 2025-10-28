package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	db "github.com/Mersad-Moghaddam/follow-service/internal/adapters/database"
	"github.com/Mersad-Moghaddam/follow-service/internal/drivers/http"
	"github.com/Mersad-Moghaddam/follow-service/internal/ports"
	"github.com/Mersad-Moghaddam/follow-service/internal/usecases"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/Mersad-Moghaddam/shared/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	// Load global .env file first
	envPath, _ := filepath.Abs("../.env")
	if err := godotenv.Load(envPath); err != nil {
		log.Fatal("Error loading global .env file:", err)
	}
	// Load local .env file to override with service-specific values
	if err := godotenv.Load(".env"); err != nil {
		log.Printf("Warning: could not load local .env file: %v", err)
	}

	config.InitDB(&ports.Follower{})

	// Initialize Redis (used by middleware for rate limiting)
	config.InitRedis()

	repo := db.NewFollowerRepository()
	followUC := usecases.NewFollowUsecase(repo)

	// HTTP API
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	// Apply security middleware
	sec := middleware.NewSecurityMiddleware(nil, config.GetRedisClient())
	app.Use(sec.SecurityMiddleware())

	// Apply rate limiting (global)
	app.Use(middleware.GlobalRateLimit(500, 1*time.Minute))

	// Apply JWT auth middleware
	jwt := middleware.NewJWTMiddleware(os.Getenv("JWT_SECRET"))
	app.Use(jwt.AuthMiddleware())
	http.RegisterRoutes(app, followUC)

	log.Println("Follow service running on :8085")
	if err := app.Listen(":" + os.Getenv("APP_PORT")); err != nil {
		log.Fatal(err)
	}

}
