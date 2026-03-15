package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Mersad-Moghaddam/admin-service/internal/adapters/database"
	http "github.com/Mersad-Moghaddam/admin-service/internal/drivers/http"
	"github.com/Mersad-Moghaddam/admin-service/internal/usecases"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/Mersad-Moghaddam/shared/middleware"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	envPath, _ := filepath.Abs("../.env")
	_ = godotenv.Load(envPath)
	_ = godotenv.Load(".env")

	// Admin service should not auto-migrate shared tables.
	// Other owning services (auth/post/follow) are responsible for schema evolution.
	config.InitDB(nil)
	config.InitRedis()

	repo := database.NewAdminRepository()
	uc := usecases.NewAdminUsecase(repo)

	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })
	app.Use(middleware.NewSecurityMiddleware(nil, config.GetRedisClient()).SecurityMiddleware())
	app.Use(middleware.GlobalRateLimit(300, time.Minute))
	jwt := middleware.NewJWTMiddleware(os.Getenv("JWT_SECRET"))
	app.Use(jwt.AuthMiddleware())
	app.Use(jwt.RoleMiddleware("admin"))
	http.RegisterRoutes(app, uc)

	log.Fatal(app.Listen(":" + os.Getenv("APP_PORT")))
}
