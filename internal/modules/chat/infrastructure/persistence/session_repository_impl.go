package persistence

import (
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	chatRepository "OmniLink/internal/modules/chat/domain/repository"
	"time"

	"gorm.io/gorm"
)

type sessionRepositoryImpl struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) chatRepository.SessionRepository {
	return &sessionRepositoryImpl{db: db}
}

func (r *sessionRepositoryImpl) GetBySendAndReceive(sendID string, receiveID string) (*chatEntity.Session, error) {
	var sess chatEntity.Session
	if err := r.db.Where("send_id = ? AND receive_id = ?", sendID, receiveID).First(&sess).Error; err != nil {
		return nil, err
	}
	return &sess, nil
}
func (r *sessionRepositoryImpl) ListUserSessionsBySendID(sendID string) ([]chatEntity.Session, error) {
	var sessions []chatEntity.Session
	err := r.db.
		Where("send_id = ? AND deleted_at IS NULL AND receive_id LIKE ?", sendID, "U%").
		Order("IFNULL(last_message_at, created_at) DESC").
		Find(&sessions).Error
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *sessionRepositoryImpl) Create(session *chatEntity.Session) error {
	return r.db.Create(session).Error
}

func (r *sessionRepositoryImpl) CreateMany(sessions []*chatEntity.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		return tx.Create(&sessions).Error
	})
}

func (r *sessionRepositoryImpl) UpdateLastMessageBySendAndReceive(sendID string, receiveID string, lastMessage string, lastMessageAt time.Time) error {
	return r.db.Model(&chatEntity.Session{}).
		Where("send_id = ? AND receive_id = ? AND deleted_at IS NULL", sendID, receiveID).
		Updates(map[string]interface{}{
			"last_message":    lastMessage,
			"last_message_at": lastMessageAt,
		}).Error
}
