package repository

import (
	"OmniLink/internal/modules/ai/domain/notification"
	"context"
)

// NotificationRepository 系统通知仓储接口（预留，暂不实现业务逻辑）
type NotificationRepository interface {
	// CreateNotification 创建通知
	CreateNotification(ctx context.Context, notif *notification.AISystemNotification) error

	// GetPendingNotifications 获取待推送的通知列表
	GetPendingNotifications(ctx context.Context, tenantUserID string, limit int) ([]*notification.AISystemNotification, error)

	// UpdateNotificationStatus 更新通知状态
	UpdateNotificationStatus(ctx context.Context, notificationID string, status int8) error
}
