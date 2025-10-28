package http

import (
	"strconv"

	"github.com/Mersad-Moghaddam/follow-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/config"
	"github.com/gofiber/fiber/v2"
)

// RegisterRoutes registers all HTTP routes for the follow service.
func RegisterRoutes(app *fiber.App, followUC ports.FollowUsecase) {

	app.Post("/follow", handleFollow(followUC))
	app.Post("/unfollow", handleUnfollow(followUC))
	app.Get("/followers/:userId", handleGetFollowers(followUC))
	app.Get("/following/:userId", handleGetFollowing(followUC))
	app.Get("/is-following/:userId", handleIsFollowing(followUC))
	app.Get("/stats/:userId", handleGetStats(followUC))

	// GET /feed - returns recent posts from users the authenticated user is following
	app.Get("/feed", handleGetFeed(followUC))
}

// handleGetFeed returns recent posts authored by users the current user follows.
// It queries the follow usecase for the following list and then reads the posts table
// directly from the shared DB to assemble a simple feed.
func handleGetFeed(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		uid := c.Locals("userID")
		userID, ok := uid.(uint)
		if !ok || userID == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}

		// get list of users the current user is following
		following, err := followUC.GetFollowing(userID)
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

// handleFollow processes a follow request from the authenticated user.
func handleFollow(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req ports.FollowRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		userID := c.Locals("userID").(uint)
		req.FollowerID = userID
		if req.UserID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
		}
		if err := followUC.Follow(req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "followed successfully"})
	}
}

// handleUnfollow processes an unfollow request from the authenticated user.
func handleUnfollow(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req ports.FollowRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
		}
		userID := c.Locals("userID").(uint)
		req.FollowerID = userID
		if req.UserID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
		}
		if err := followUC.Unfollow(req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "unfollowed successfully"})
	}
}

// handleGetFollowers returns all followers of a specific user.
func handleGetFollowers(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := c.ParamsInt("userId")
		if err != nil || userID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
		}
		followers, err := followUC.GetFollowers(uint(userID))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"followers": followers})
	}
}

// handleGetFollowing returns all users that the given user is following.
func handleGetFollowing(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := c.ParamsInt("userId")
		if err != nil || userID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
		}
		following, err := followUC.GetFollowing(uint(userID))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"following": following})
	}
}

// handleIsFollowing checks if the authenticated user follows another user.
func handleIsFollowing(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		targetUserID, err := c.ParamsInt("userId")
		if err != nil || targetUserID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
		}
		currentUserID := c.Locals("userID").(uint)
		isFollowing, err := followUC.IsFollowing(currentUserID, uint(targetUserID))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"is_following": isFollowing})
	}
}

// handleGetStats returns the follower and following counts for a given user.
func handleGetStats(followUC ports.FollowUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		userID, err := c.ParamsInt("userId")
		if err != nil || userID <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid user ID"})
		}
		followers, following, err := followUC.GetFollowStats(uint(userID))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{
			"followers_count": followers,
			"following_count": following,
		})
	}
}
