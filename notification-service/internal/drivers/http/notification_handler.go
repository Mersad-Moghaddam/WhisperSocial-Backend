package http

import (
	"strconv"

	"github.com/Mersad-Moghaddam/notification-service/internal/ports"
	"github.com/gofiber/fiber/v2"
)

type NotificationHandler struct {
	uc ports.NotificationUseCase
}

func NewNotificationHandler(uc ports.NotificationUseCase) *NotificationHandler {
	return &NotificationHandler{uc: uc}
}

func RegisterRoutes(group fiber.Router, uc ports.NotificationUseCase) {
	handler := NewNotificationHandler(uc)
	group.Get("/", func(c *fiber.Ctx) error { return handler.GetNotifications(c) })
	group.Put("/:id/read", func(c *fiber.Ctx) error { return handler.MarkAsRead(c) })
	group.Delete("/:id", func(c *fiber.Ctx) error { return handler.DeleteNotification(c) })
}

func (h *NotificationHandler) GetNotifications(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	cursor := c.Query("cursor", "")
	ntype := c.Query("type", "")

	data, nextCursor, err := h.uc.GetNotificationsByUserID(userID, limit, cursor, ntype)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch notifications"})
	}
	return c.JSON(fiber.Map{"data": data, "pagination": fiber.Map{"next_cursor": nextCursor}})
}

func (h *NotificationHandler) MarkAsRead(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID"})
	}
	notification, err := h.uc.GetNotificationByID(uint(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Notification not found"})
	}
	if notification.UserID != userID {
		return c.Status(403).JSON(fiber.Map{"error": "No permission"})
	}
	_ = h.uc.MarkNotificationAsRead(uint(id))
	return c.JSON(fiber.Map{"message": "Notification marked as read"})
}

func (h *NotificationHandler) DeleteNotification(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(uint)
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid ID"})
	}
	notification, err := h.uc.GetNotificationByID(uint(id))
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Notification not found"})
	}
	if notification.UserID != userID {
		return c.Status(403).JSON(fiber.Map{"error": "No permission"})
	}
	_ = h.uc.DeleteNotification(uint(id))
	return c.JSON(fiber.Map{"message": "Notification deleted"})
}
