package repository

import "OmniLink/internal/modules/contact/domain/entity"

type UserContactRepository interface {
	GetUserContactsByUserID(userID string) ([]entity.UserContact, error)
	GetUserContactByUserIDAndContactID(userID string, contactID string) (*entity.UserContact, error)
	GetUserContactByUserIDAndContactIDAndType(userID string, contactID string, contactType int8) (*entity.UserContact, error)
	GetGroupMembers(groupID string) ([]entity.UserContact, error)
	CreateUserContact(contact *entity.UserContact) error
	UpdateUserContact(contact *entity.UserContact) error
}
