package repository

import "OmniLink/internal/modules/contact/domain/entity"

type ContactApplyRepository interface {
	GetContactApplyByUserIDAndContactID(userID string, contactID string, contactType int8) (*entity.ContactApply, error)
	CreateContactApply(apply *entity.ContactApply) error
	UpdateContactApply(apply *entity.ContactApply) error
}
