package ports

import "time"

// NotificationType represents the type of notification
type NotificationType string

const (
	NewFollowNotification NotificationType = "new_follow"
	NewPostNotification   NotificationType = "new_post"
	NewCommentNotification NotificationType = "new_comment"
)

// Notification represents the notification entity
type Notification struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index"`
	Content   string    `json:"content"`
	Type      NotificationType `json:"type" gorm:"type:varchar(50)"`
	ReadStatus bool      `json:"read_status" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at"`
}

// NotificationRepository defines the interface for notification storage operations
type NotificationRepository interface {
	Create(notification *Notification) error
	GetByID(id uint) (*Notification, error)
	GetByUserID(userID uint, limit int, cursor string, notificationType string) ([]*Notification, string, error)
	MarkAsRead(id uint) error
	Delete(id uint) error
	BatchCreate(notifications []*Notification) error
}

// NotificationPublisher defines the interface for publishing notification events
type NotificationPublisher interface {
	PublishNotification(notification *Notification) error
}

// NotificationSubscriber defines the interface for subscribing to events
type NotificationSubscriber interface {
	SubscribeToEvents() error
}

// NotificationDelivery defines the interface for notification delivery methods
type NotificationDelivery interface {
	DeliverNotification(notification *Notification) error
}

// EmailDelivery defines the interface for email notification delivery
type EmailDelivery interface {
	SendEmail(userID uint, subject, content string) error
}

// PushDelivery defines the interface for push notification delivery
type PushDelivery interface {
	SendPush(userID uint, title, body string) error
}