package persistence

import (
	"OmniLink/internal/modules/chat/domain/entity"
	"OmniLink/internal/modules/chat/domain/repository"

	"gorm.io/gorm"
)

type messageMentionRepositoryImpl struct {
	db *gorm.DB
}

func NewMessageMentionRepository(db *gorm.DB) repository.MessageMentionRepository {
	return &messageMentionRepositoryImpl{db: db}
}

func (r *messageMentionRepositoryImpl) CreateBatch(mentions []*entity.MessageMention) error {
	if len(mentions) == 0 {
		return nil
	}
	return r.db.Create(&mentions).Error
}

func (r *messageMentionRepositoryImpl) GetMentionsByMessageUUID(messageUUID string) ([]entity.MessageMention, error) {
	var mentions []entity.MessageMention
	err := r.db.Where("message_uuid = ?", messageUUID).Find(&mentions).Error
	return mentions, err
}

func (r *messageMentionRepositoryImpl) GetMentionsByMessageUUIDs(messageUUIDs []string) (map[string][]entity.MessageMention, error) {
	if len(messageUUIDs) == 0 {
		return nil, nil
	}
	var mentions []entity.MessageMention
	err := r.db.Where("message_uuid IN ?", messageUUIDs).Find(&mentions).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string][]entity.MessageMention)
	for _, m := range mentions {
		result[m.MessageUuid] = append(result[m.MessageUuid], m)
	}
	return result, nil
}
