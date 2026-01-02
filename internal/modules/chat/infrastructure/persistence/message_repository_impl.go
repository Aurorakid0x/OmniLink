package persistence

import (
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	chatRepository "OmniLink/internal/modules/chat/domain/repository"

	"gorm.io/gorm"
)

type messageRepositoryImpl struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) chatRepository.MessageRepository {
	return &messageRepositoryImpl{db: db}
}

func (r *messageRepositoryImpl) ListPrivateMessages(userOneID string, userTwoID string, page int, pageSize int) ([]chatEntity.Message, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var msgs []chatEntity.Message
	err := r.db.
		Where("(send_id = ? AND receive_id = ?) OR (send_id = ? AND receive_id = ?)", userOneID, userTwoID, userTwoID, userOneID).
		Order("created_at DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&msgs).Error
	if err != nil {
		return nil, err
	}
	return msgs, nil
}

func (r *messageRepositoryImpl) Create(message *chatEntity.Message) error {
	return r.db.Create(message).Error
}
