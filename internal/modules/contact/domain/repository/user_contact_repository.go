package repository

import (
	"time"

	"OmniLink/internal/modules/contact/domain/entity"
)

type UserContactRepository interface {
	GetUserContactsByUserID(userID string) ([]entity.UserContact, error)
	GetUserContactByUserIDAndContactID(userID string, contactID string) (*entity.UserContact, error)
	GetUserContactByUserIDAndContactIDAndType(userID string, contactID string, contactType int8) (*entity.UserContact, error)
	ListContactsWithInfo(userID string) ([]entity.ContactWithUserInfo, error)
	GetGroupMembers(groupID string) ([]entity.UserContact, error)
	GetGroupMembersWithInfo(groupID string) ([]entity.ContactWithUserInfo, error)
	CreateUserContact(contact *entity.UserContact) error
	UpdateUserContact(contact *entity.UserContact) error
	UpdateGroupContactsStatus(groupID string, status int8, updateAt time.Time) error
}
