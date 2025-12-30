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
