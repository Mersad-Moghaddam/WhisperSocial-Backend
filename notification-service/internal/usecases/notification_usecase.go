package usecases

import (
	"encoding/json"
	"log"

	"github.com/Mersad-Moghaddam/notification-service/internal/ports"
)

type NotificationUseCase struct {
	repo  ports.NotificationRepository
	email ports.EmailDelivery
	push  ports.PushDelivery
}

func NewNotificationUseCase(repo ports.NotificationRepository, email ports.EmailDelivery, push ports.PushDelivery) *NotificationUseCase {
	return &NotificationUseCase{repo: repo, email: email, push: push}
}

func (uc *NotificationUseCase) CreateNotification(n *ports.Notification) error {
	if err := uc.repo.Create(n); err != nil {
		return err
	}
	go uc.deliverNotification(n)
	return nil
}

func (uc *NotificationUseCase) GetNotificationByID(id uint) (*ports.Notification, error) {
	return uc.repo.GetByID(id)
}

func (uc *NotificationUseCase) GetNotificationsByUserID(userID uint, limit int, cursor, ntype string) ([]*ports.Notification, string, error) {
	return uc.repo.GetByUserID(userID, limit, cursor, ntype)
}

func (uc *NotificationUseCase) MarkNotificationAsRead(id uint) error {
	return uc.repo.MarkAsRead(id)
}

func (uc *NotificationUseCase) DeleteNotification(id uint) error {
	return uc.repo.Delete(id)
}

func (uc *NotificationUseCase) BatchCreateNotifications(ns []*ports.Notification) error {
	return uc.repo.BatchCreate(ns)
}

func (uc *NotificationUseCase) ProcessEvent(eventType string, data []byte) error {
	var n *ports.Notification
	switch eventType {
	case "follow_created":
		var e struct{ FollowerID, FolloweeID uint }
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		n = &ports.Notification{UserID: e.FolloweeID, Content: "You have a new follower", Type: ports.NewFollowNotification}
	case "post_created":
		var e struct{ PostID, UserID, AuthorID uint }
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		n = &ports.Notification{UserID: e.UserID, Content: "New post from someone you follow", Type: ports.NewPostNotification}
	case "comment_created":
		var e struct{ CommentID, PostID, UserID, AuthorID uint }
		if err := json.Unmarshal(data, &e); err != nil {
			return err
		}
		n = &ports.Notification{UserID: e.AuthorID, Content: "Someone commented on your post", Type: ports.NewCommentNotification}
	default:
		log.Printf("Unknown event type: %s", eventType)
		return nil
	}
	return uc.CreateNotification(n)
}

func (uc *NotificationUseCase) deliverNotification(n *ports.Notification) {
	if uc.email != nil {
		_ = uc.email.SendEmail(n.UserID, "New Notification", n.Content)
	}
	if uc.push != nil {
		_ = uc.push.SendPush(n.UserID, "New Notification", n.Content)
	}
}
