package stream

import (
	"encoding/json"
	"log"
	"time"

	"github.com/Mersad-Moghaddam/notification-service/internal/ports"
	"github.com/Mersad-Moghaddam/shared/messagequeue"
)

type EventSubscriber struct {
	rabbitMQ       *messagequeue.RabbitMQ
	notificationUC ports.NotificationUseCase
}

type FollowEvent struct {
	FollowerID uint `json:"follower_id"`
	FolloweeID uint `json:"followee_id"`
}

type PostEvent struct {
	PostID   uint `json:"post_id"`
	UserID   uint `json:"user_id"`
	AuthorID uint `json:"author_id"`
}

type CommentEvent struct {
	CommentID uint `json:"comment_id"`
	PostID    uint `json:"post_id"`
	UserID    uint `json:"user_id"`
	AuthorID  uint `json:"author_id"`
}

func NewSubscriber(rabbitMQ *messagequeue.RabbitMQ, notificationUC ports.NotificationUseCase) *EventSubscriber {
	return &EventSubscriber{
		rabbitMQ:       rabbitMQ,
		notificationUC: notificationUC,
	}
}

func (s *EventSubscriber) SubscribeToEvents() error {
	// Subscribe to follow events
	if err := s.rabbitMQ.ConsumeMessages("follow_events", s.handleMessage); err != nil {
		return err
	}

	// Subscribe to post events
	if err := s.rabbitMQ.ConsumeMessages("post_events", s.handleMessage); err != nil {
		return err
	}

	// Subscribe to comment events
	if err := s.rabbitMQ.ConsumeMessages("notification_events", s.handleMessage); err != nil {
		return err
	}

	return nil
}

func (s *EventSubscriber) handleMessage(message messagequeue.Message) error {
	log.Printf("Received message of type: %s", message.Type)

	switch message.Type {
	case messagequeue.FollowCreated:
		var followEvent FollowEvent
		if err := json.Unmarshal(message.Data, &followEvent); err != nil {
			return err
		}

		notification := &ports.Notification{
			UserID:     followEvent.FolloweeID,
			Content:    "You have a new follower",
			Type:       ports.NewFollowNotification,
			ReadStatus: false,
			CreatedAt:  time.Now(),
		}

		return s.notificationUC.CreateNotification(notification)

	case messagequeue.PostCreated:
		var postEvent PostEvent
		if err := json.Unmarshal(message.Data, &postEvent); err != nil {
			return err
		}

		notification := &ports.Notification{
			UserID:     postEvent.UserID,
			Content:    "New post from someone you follow",
			Type:       ports.NewPostNotification,
			ReadStatus: false,
			CreatedAt:  time.Now(),
		}

		return s.notificationUC.CreateNotification(notification)

	case messagequeue.CommentCreated:
		var commentEvent CommentEvent
		if err := json.Unmarshal(message.Data, &commentEvent); err != nil {
			return err
		}

		notification := &ports.Notification{
			UserID:     commentEvent.AuthorID,
			Content:    "Someone commented on your post",
			Type:       ports.NewCommentNotification,
			ReadStatus: false,
			CreatedAt:  time.Now(),
		}

		return s.notificationUC.CreateNotification(notification)

	default:
		log.Printf("Unknown message type: %s", message.Type)
		return nil
	}
}