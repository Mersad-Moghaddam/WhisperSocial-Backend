package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/Mersad-Moghaddam/notification-service/internal/adapters/database"
	"github.com/Mersad-Moghaddam/notification-service/internal/adapters/email"
	"github.com/Mersad-Moghaddam/notification-service/internal/adapters/push"
	"github.com/Mersad-Moghaddam/notification-service/internal/adapters/stream"
	"github.com/Mersad-Moghaddam/notification-service/internal/drivers/http"
	"github.com/Mersad-Moghaddam/notification-service/internal/usecases"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/Mersad-Moghaddam/shared/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"
)

func main() {
	envPath, _ := filepath.Abs("../.env")
	if err := godotenv.Load(envPath); err != nil {
		log.Fatal("Error loading global .env file:", err)
	}
	_ = godotenv.Load(".env")

	config.InitDB(nil)
	config.InitRedis()

	sqlDB := database.NewNotificationRepository(config.DB)
	smtpPort, _ := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if smtpPort == 0 {
		smtpPort = 587
	}

	emailService := email.NewEmailService(
		os.Getenv("SMTP_HOST"),
		smtpPort,
		os.Getenv("SMTP_USERNAME"),
		os.Getenv("SMTP_PASSWORD"),
		os.Getenv("SMTP_FROM"),
	)
	pushService := push.NewPushService(os.Getenv("PUSH_API_KEY"))

	notificationUC := usecases.NewNotificationUseCase(sqlDB, emailService, pushService)
	rabbitMQ := stream.NewSubscriber(config.RabbitMQ, notificationUC)
	if err := rabbitMQ.SubscribeToEvents(); err != nil {
		log.Fatalf("Failed to subscribe to events: %v", err)
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			log.Printf("Error occurred: %v", err)
			return c.Status(code).JSON(fiber.Map{"error": true, "message": err.Error()})
		},
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	app.Get("/health", func(c *fiber.Ctx) error {
		if err := sqlDB.Ping(); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "unhealthy", "error": "database connection failed"})
		}
		if err := config.RedisClient.Ping(context.Background()).Err(); err != nil {
			return c.Status(503).JSON(fiber.Map{"status": "unhealthy", "error": "redis connection failed"})
		}
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	protected := app.Group("/notifications")
	// Apply security middleware
	sec := middleware.NewSecurityMiddleware(nil, config.GetRedisClient())
	protected.Use(sec.SecurityMiddleware())

	// Apply rate limiting (global)
	protected.Use(middleware.GlobalRateLimit(1000, 1*time.Minute))

	// Apply JWT auth middleware
	jwt := middleware.NewJWTMiddleware(os.Getenv("JWT_SECRET"))
	protected.Use(jwt.AuthMiddleware())

	http.RegisterRoutes(protected, notificationUC)

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8083"
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Notification service running on port %s", port)
		serverErr <- app.Listen(":" + port)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	case <-quit:
		log.Println("Shutdown signal received...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = app.ShutdownWithContext(ctx)
	_ = sqlDB.Close()
	_ = config.RedisClient.Close()
	_ = config.RabbitMQ.Close()
	log.Println("Notification service shut down completely")
}
