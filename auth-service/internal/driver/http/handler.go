package http

import (
	"time"

	"github.com/Mersad-Moghaddam/auth-service/internal/ports"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func RegisterRoutes(app *fiber.App, uc ports.AuthUsecase) {
	// Rate limiter for login endpoint to prevent brute-force attacks
	loginLimiter := limiter.New(limiter.Config{
		Max:        10,
		Expiration: 1 * time.Minute,
	})
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Put("/update", func(c *fiber.Ctx) error {
		return updateHandler(c, uc)
	})
	// User registration endpoint
	app.Post("/register", func(c *fiber.Ctx) error {
		return registerHandler(c, uc)
	})

	// User login endpoint
	app.Post("/login", loginLimiter, func(c *fiber.Ctx) error {
		return loginHandler(c, uc)
	})
}
func updateHandler(c *fiber.Ctx, uc ports.AuthUsecase) error {
	var req ports.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return uc.UpdateInfo(c, req)
}
func registerHandler(c *fiber.Ctx, uc ports.AuthUsecase) error {
	var req ports.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return uc.Register(c, req)
}

func loginHandler(c *fiber.Ctx, uc ports.AuthUsecase) error {
	var req ports.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := req.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return uc.Login(c, req)
}
