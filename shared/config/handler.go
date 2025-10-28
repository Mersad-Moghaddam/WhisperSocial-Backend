package config

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
)

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	log.Printf("HTTP Error: %d - %s - Path: %s - Method: %s - IP: %s",
		code, message, c.Path(), c.Method(), c.IP())

	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Frame-Options", "DENY")
	c.Set("X-XSS-Protection", "1; mode=block")

	return c.Status(code).JSON(fiber.Map{
		"error":     true,
		"message":   message,
		"code":      code,
		"timestamp": time.Now(),
		"path":      c.Path(),
	})
}

func NotFoundHandler(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
		"error":     true,
		"message":   "Route not found",
		"code":      404,
		"timestamp": time.Now(),
		"path":      c.Path(),
	})
}
