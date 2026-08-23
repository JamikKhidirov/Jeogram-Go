package repository

import (
	"context"

	"github.com/jeogram/messenger/internal/modules/notification/domain"
	"gorm.io/gorm"
)

// NotificationRepository persists in-app notifications.
type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Create(ctx context.Context, n *domain.Notification) error {
	return r.db.WithContext(ctx).Create(n).Error
}

func (r *NotificationRepository) List(ctx context.Context, userID string, limit, offset int) ([]domain.Notification, error) {
	var ns []domain.Notification
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&ns).Error
	return ns, err
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Model(&domain.Notification{}).
		Where("user_id = ? AND read = ?", userID, false).
		Update("read", true).Error
}
