package repository

import "OmniLink/internal/modules/chat/domain/entity"

type SessionRepository interface {
	GetBySendAndReceive(sendID string, receiveID string) (*entity.Session, error)
	ListUserSessionsBySendID(sendID string) ([]entity.Session, error)
	Create(session *entity.Session) error
	CreateMany(sessions []*entity.Session) error
}
