package http

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/Mersad-Moghaddam/timeline-service/internal/ports"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RegisterRoutes sets up HTTP routes for the timeline service.
func RegisterRoutes(app *fiber.App, uc ports.GetTimelineUsecase) {
	// Rate limiter to prevent abuse of timeline requests
	timelineLimiter := limiter.New(limiter.Config{
		Max:        200,
		Expiration: 5 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			uid := c.Locals("userID")
			return fmt.Sprintf("%v", uid)
		},
	})

	// GET /timeline
	// Retrieves the authenticated user's timeline with pagination support.
	app.Get("/timeline", timelineLimiter, timelineHandler(uc))
}

// timelineHandler returns a Fiber handler function that closes over the usecase instance.
func timelineHandler(uc ports.GetTimelineUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := c.Locals("userID")
		userID, ok := uid.(uint)
		if !ok || userID == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		// get list of users the current user is following
		following, err := uc.GetFollowing(userID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		if len(following) == 0 {
			return c.JSON(fiber.Map{"posts": []any{}, "next_cursor": 0})
		}

		// Query posts table for recent posts by authors in the following list
		// Using shared DB directly here for simplicity
		type PostRow struct {
			ID       uint   `json:"id"`
			AuthorID uint   `json:"author_id"`
			Content  string `json:"content"`
		}

		var posts []PostRow
		// limit param support
		limitStr := c.Query("limit", "200")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 200
		}

		// Build query
		if err := config.DB.Table("posts").Select("id, author_id, content").Where("author_id IN ?", following).Order("created_at DESC").Limit(limit).Find(&posts).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}

		return c.JSON(fiber.Map{"posts": posts, "next_cursor": 0})
	}
}
