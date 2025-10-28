package ports

// NotificationUseCase defines the interface for notification use cases
type NotificationUseCase interface {
	CreateNotification(notification *Notification) error
	GetNotificationByID(id uint) (*Notification, error)
	GetNotificationsByUserID(userID uint, limit int, cursor string, notificationType string) ([]*Notification, string, error)
	MarkNotificationAsRead(id uint) error
	DeleteNotification(id uint) error
	BatchCreateNotifications(notifications []*Notification) error
	ProcessEvent(eventType string, data []byte) error
}