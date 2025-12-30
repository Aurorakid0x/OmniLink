package persistence

import (
	"OmniLink/internal/modules/contact/domain/entity"
	"OmniLink/internal/modules/contact/domain/repository"

	"gorm.io/gorm"
)

type contactApplyRepositoryImpl struct {
	db *gorm.DB
}

func NewContactApplyRepository(db *gorm.DB) repository.ContactApplyRepository {
	return &contactApplyRepositoryImpl{db: db}
}

func (r *contactApplyRepositoryImpl) GetContactApplyByUserIDAndContactID(userID string, contactID string, contactType int8) (*entity.ContactApply, error) {
	var apply entity.ContactApply
	err := r.db.
		Where("user_id = ? AND contact_id = ? AND contact_type = ?", userID, contactID, contactType).
		Order("id DESC").
		First(&apply).Error
	if err != nil {
		return nil, err
	}
	return &apply, nil
}

func (r *contactApplyRepositoryImpl) CreateContactApply(apply *entity.ContactApply) error {
	return r.db.Create(apply).Error
}

func (r *contactApplyRepositoryImpl) UpdateContactApply(apply *entity.ContactApply) error {
	return r.db.Model(&entity.ContactApply{}).
		Where("id = ?", apply.Id).
		Updates(map[string]interface{}{
			"uuid":          apply.Uuid,
			"status":        apply.Status,
			"message":       apply.Message,
			"last_apply_at": apply.LastApplyAt,
		}).Error
}
