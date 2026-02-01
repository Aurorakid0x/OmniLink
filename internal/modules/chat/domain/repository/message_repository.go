package repository

import (
	"context"
	"time"

	"OmniLink/internal/modules/chat/domain/entity"
)

type MessageRepository interface {
	ListPrivateMessages(userOneID string, userTwoID string, page int, pageSize int) ([]entity.Message, error)
	ListGroupMessages(groupID string, page int, pageSize int) ([]entity.Message, error)
	Create(message *entity.Message) error
	// GetMessagesForUserAfter 获取指定时间后，用户接收到的所有消息（私聊+群聊）
	GetMessagesForUserAfter(ctx context.Context, userID string, groupIDs []string, since time.Time, limit int) ([]entity.Message, error)
}
