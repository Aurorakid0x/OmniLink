package repository

import "OmniLink/internal/modules/chat/domain/entity"

type MessageRepository interface {
	ListPrivateMessages(userOneID string, userTwoID string, page int, pageSize int) ([]entity.Message, error)
}
