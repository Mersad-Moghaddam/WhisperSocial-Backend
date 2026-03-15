package http

import (
	"strconv"
	"time"

	"github.com/Mersad-Moghaddam/admin-service/internal/ports"
	"github.com/gofiber/fiber/v2"
)

func RegisterRoutes(app *fiber.App, uc ports.AdminUsecase) {
	app.Get("/admin/users", listUsers(uc))
	app.Get("/admin/users/:userId", getUser(uc))
	app.Patch("/admin/users/:userId", patchUser(uc))
	app.Get("/admin/posts", listPosts(uc))
	app.Get("/admin/posts/:postId", getPost(uc))
	app.Delete("/admin/posts/:postId", deletePost(uc))
	app.Get("/admin/stats", stats(uc))
}

func listUsers(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		if offset < 0 {
			offset = 0
		}
		f := ports.UserFilters{Status: c.Query("status"), Limit: limit, Offset: offset}
		if s := c.Query("created_after"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				f.CreatedAfter = &t
			}
		}
		if s := c.Query("created_before"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				f.CreatedBefore = &t
			}
		}
		users, err := uc.ListUsers(f)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(users)
	}
}
func getUser(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, _ := strconv.Atoi(c.Params("userId"))
		user, err := uc.GetUserByID(uint(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		return c.JSON(user)
	}
}
func patchUser(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, _ := strconv.Atoi(c.Params("userId"))
		var body struct {
			Status string `json:"status"`
		}
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		switch body.Status {
		case "active", "deactivated", "restricted":
		default:
			return c.Status(400).JSON(fiber.Map{"error": "invalid status"})
		}
		if err := uc.UpdateUserStatus(uint(id), body.Status); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"message": "user status updated"})
	}
}
func listPosts(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		limit, _ := strconv.Atoi(c.Query("limit", "20"))
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		offset, _ := strconv.Atoi(c.Query("offset", "0"))
		if offset < 0 {
			offset = 0
		}
		f := ports.PostFilters{Limit: limit, Offset: offset}
		if v, _ := strconv.Atoi(c.Query("user_id", "0")); v > 0 {
			f.UserID = uint(v)
		}
		if s := c.Query("start_date"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				f.StartDate = &t
			}
		}
		if s := c.Query("end_date"); s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				f.EndDate = &t
			}
		}
		posts, err := uc.ListPosts(f)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(posts)
	}
}
func getPost(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, _ := strconv.Atoi(c.Params("postId"))
		post, err := uc.GetPostByID(uint(id))
		if err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "post not found"})
		}
		return c.JSON(post)
	}
}
func deletePost(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		id, _ := strconv.Atoi(c.Params("postId"))
		if err := uc.DeletePost(uint(id)); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.SendStatus(204)
	}
}
func stats(uc ports.AdminUsecase) fiber.Handler {
	return func(c *fiber.Ctx) error {
		st, err := uc.Stats()
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(st)
	}
}
