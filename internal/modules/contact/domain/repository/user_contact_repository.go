package repository

import "OmniLink/internal/modules/contact/domain/entity"

type UserContactRepository interface {
	GetUserContactsByUserID(userID string) ([]entity.UserContact, error)
	GetUserContactByUserIDAndContactID(userID string, contactID string) (*entity.UserContact, error)
}
