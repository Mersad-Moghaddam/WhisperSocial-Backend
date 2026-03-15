package http

import (
	"strings"

	"github.com/Mersad-Moghaddam/post-service/internal/ports"
	"github.com/Mersad-Moghaddam/post-service/internal/usecases"
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all HTTP routes for the post service.
func RegisterRoutes(app *fiber.App, uc ports.CreatePostUsecase) {

	app.Post("/posts", handleCreatePost(uc))
}

// handleCreatePost handles creating a new post for the authenticated user.
func handleCreatePost(uc ports.CreatePostUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req ports.CreatePostRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		uid := c.Locals("userID")
		userID, ok := uid.(uint)
		if !ok || userID == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		req.AuthorID = userID

		if strings.TrimSpace(req.Content) == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "content is required"})
		}
		if len(req.Content) > 2000 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "content too long"})
		}

		post, err := uc.Create(req)
		if err != nil {
			if err == usecases.ErrUserCannotPost {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(post)
	}
}
