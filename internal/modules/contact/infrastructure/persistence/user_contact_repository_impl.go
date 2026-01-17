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

func (r *userContactRepositoryImpl) ListContactsWithInfo(userID string) ([]entity.ContactWithUserInfo, error) {
	var contacts []entity.ContactWithUserInfo
	// Join user_contact and user_info
	// user_contact.contact_id -> user_info.uuid
	err := r.db.Table("user_contact").
		Select("user_contact.*, user_info.nickname, user_info.avatar, user_info.signature").
		Joins("JOIN user_info ON user_contact.contact_id = user_info.uuid").
		Where("user_contact.user_id = ? AND user_contact.contact_type = 0", userID).
		Find(&contacts).Error
	if err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *userContactRepositoryImpl) GetGroupMembers(groupID string) ([]entity.UserContact, error) {
	var contacts []entity.UserContact
	err := r.db.Where("contact_id = ? AND contact_type = ? AND deleted_at IS NULL AND status NOT IN ?", groupID, 1, []int8{6, 7}).Find(&contacts).Error
	if err != nil {
		return nil, err
	}
	return contacts, nil
}

func (r *userContactRepositoryImpl) GetGroupMembersWithInfo(groupID string) ([]entity.ContactWithUserInfo, error) {
	var members []entity.ContactWithUserInfo
	// Join user_contact and user_info
	// user_contact.user_id -> user_info.uuid (because in group, contact_id is group_id, user_id is member_id)
	err := r.db.Table("user_contact").
		Select("user_contact.*, user_info.nickname, user_info.avatar, user_info.signature").
		Joins("JOIN user_info ON user_contact.user_id = user_info.uuid").
		Where("user_contact.contact_id = ? AND user_contact.contact_type = 1 AND user_contact.deleted_at IS NULL AND user_contact.status NOT IN ?", groupID, []int8{6, 7}).
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
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
