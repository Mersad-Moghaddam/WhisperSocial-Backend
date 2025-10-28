package database

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Mersad-Moghaddam/notification-service/internal/ports"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (r *NotificationRepository) Create(notification *ports.Notification) error {
	return r.db.Create(notification).Error
}

func (r *NotificationRepository) GetByID(id uint) (*ports.Notification, error) {
	var notification ports.Notification
	if err := r.db.First(&notification, id).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

func (r *NotificationRepository) GetByUserID(userID uint, limit int, cursor string, notificationType string) ([]*ports.Notification, string, error) {
	var notifications []*ports.Notification
	query := r.db.Where("user_id = ?", userID).Order("created_at DESC, id DESC").Limit(limit + 1)

	// Apply cursor-based pagination if cursor is provided
	if cursor != "" {
		cursorParts := []string{}
		for _, part := range []string{cursor} {
			cursorParts = append(cursorParts, part)
		}
		if len(cursorParts) == 2 {
			cursorTime, err := time.Parse(time.RFC3339, cursorParts[0])
			if err == nil {
				cursorID, err := strconv.ParseUint(cursorParts[1], 10, 64)
				if err == nil {
					query = query.Where("(created_at, id) < (?, ?)", cursorTime, cursorID)
				}
			}
		}
	}

	// Apply type filter if provided
	if notificationType != "" {
		query = query.Where("type = ?", notificationType)
	}

	if err := query.Find(&notifications).Error; err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(notifications) > limit {
		lastItem := notifications[limit-1]
		nextCursor = fmt.Sprintf("%s_%d", lastItem.CreatedAt.Format(time.RFC3339), lastItem.ID)
		notifications = notifications[:limit]
	}

	return notifications, nextCursor, nil
}

func (r *NotificationRepository) MarkAsRead(id uint) error {
	return r.db.Model(&ports.Notification{}).Where("id = ?", id).Update("read_status", true).Error
}

func (r *NotificationRepository) Delete(id uint) error {
	return r.db.Delete(&ports.Notification{}, id).Error
}

func (r *NotificationRepository) BatchCreate(notifications []*ports.Notification) error {
	return r.db.CreateInBatches(notifications, 100).Error
}
func (r *NotificationRepository) Ping() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

func (r *NotificationRepository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
