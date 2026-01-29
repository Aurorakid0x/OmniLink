package persistence

import (
	"OmniLink/internal/modules/ai/domain/notification"
	"OmniLink/internal/modules/ai/domain/repository"
	"context"

	"gorm.io/gorm"
)

type notificationRepositoryImpl struct {
	db *gorm.DB
}

// NewNotificationRepository 创建通知仓储实现（预留）
func NewNotificationRepository(db *gorm.DB) repository.NotificationRepository {
	return &notificationRepositoryImpl{db: db}
}

func (r *notificationRepositoryImpl) CreateNotification(ctx context.Context, notif *notification.AISystemNotification) error {
	return r.db.WithContext(ctx).Create(notif).Error
}

func (r *notificationRepositoryImpl) GetPendingNotifications(ctx context.Context, tenantUserID string, limit int) ([]*notification.AISystemNotification, error) {
	var notifs []*notification.AISystemNotification
	err := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND status = ?", tenantUserID, notification.StatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&notifs).Error
	return notifs, err
}

func (r *notificationRepositoryImpl) UpdateNotificationStatus(ctx context.Context, notificationID string, status int8) error {
	return r.db.WithContext(ctx).
		Model(&notification.AISystemNotification{}).
		Where("notification_id = ?", notificationID).
		Update("status", status).Error
}
