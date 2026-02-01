package repository

import "OmniLink/internal/modules/chat/domain/entity"

type MessageMentionRepository interface {
	CreateBatch(mentions []*entity.MessageMention) error
	GetMentionsByMessageUUID(messageUUID string) ([]entity.MessageMention, error)
	GetMentionsByMessageUUIDs(messageUUIDs []string) (map[string][]entity.MessageMention, error)
}
