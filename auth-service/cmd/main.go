package main

import (
	"log"
	"os"
	"path/filepath"

	database "github.com/Mersad-Moghaddam/auth-service/internal/adapters/database"
	"github.com/Mersad-Moghaddam/auth-service/internal/driver/http"
	"github.com/Mersad-Moghaddam/auth-service/internal/ports"

	"github.com/Mersad-Moghaddam/auth-service/internal/usecases"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {
	path, err := filepath.Abs("../.env")
	if err != nil {
		log.Fatal(err)
	}
	if err := godotenv.Load(path); err != nil {
		log.Fatal("Error loading .env file:", err)
	}
	config.InitDB(&ports.User{})

	repo := database.NewUserRepository()
	uc := usecases.NewAuthUsecase(repo)

	app := fiber.New()
	http.RegisterRoutes(app, uc)

	if err := app.Listen(":" + os.Getenv("APP_PORT")); err != nil {
		log.Fatal(err)
	}

}
