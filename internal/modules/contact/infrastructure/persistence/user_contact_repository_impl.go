package persistence

import (
	"OmniLink/internal/modules/contact/domain/entity"
	"OmniLink/internal/modules/contact/domain/repository"

	"gorm.io/gorm"
)

type userContactRepositoryImpl struct {
	db *gorm.DB
}

func NewUserContactRepository(db *gorm.DB) repository.UserContactRepository {
	return &userContactRepositoryImpl{db: db}
}

// 1. 实现 GetUserContactsByUserID
func (r *userContactRepositoryImpl) GetUserContactsByUserID(userID string) ([]entity.UserContact, error) {
	var contacts []entity.UserContact
	err := r.db.Where("user_id = ?", userID).Find(&contacts).Error
	if err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *userContactRepositoryImpl) GetUserContactByUserIDAndContactID(userID string, contactID string) (*entity.UserContact, error) {
	var contact entity.UserContact
	err := r.db.Where("user_id = ? AND contact_id = ?", userID, contactID).First(&contact).Error
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *userContactRepositoryImpl) GetUserContactByUserIDAndContactIDAndType(userID string, contactID string, contactType int8) (*entity.UserContact, error) {
	var contact entity.UserContact
	err := r.db.Where("user_id = ? AND contact_id = ? AND contact_type = ?", userID, contactID, contactType).First(&contact).Error
	if err != nil {
		return nil, err
	}
	return &contact, nil
}

func (r *userContactRepositoryImpl) CreateUserContact(contact *entity.UserContact) error {
	return r.db.Create(contact).Error
}

func (r *userContactRepositoryImpl) UpdateUserContact(contact *entity.UserContact) error {
	return r.db.Model(&entity.UserContact{}).
		Where("id = ?", contact.Id).
		Updates(map[string]interface{}{
			"user_id":      contact.UserId,
			"contact_id":   contact.ContactId,
			"contact_type": contact.ContactType,
			"status":       contact.Status,
			"update_at":    contact.UpdateAt,
		}).Error
}
