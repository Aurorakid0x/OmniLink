package persistence

import (
	"OmniLink/internal/modules/contact/domain/entity"
	"OmniLink/internal/modules/contact/domain/repository"

	"gorm.io/gorm"
)

type groupInfoRepositoryImpl struct {
	db *gorm.DB
}

func NewGroupInfoRepository(db *gorm.DB) repository.GroupInfoRepository {
	return &groupInfoRepositoryImpl{db: db}
}

func (r *groupInfoRepositoryImpl) CreateGroupInfo(group *entity.GroupInfo) error {
	return r.db.Create(group).Error
}

func (r *groupInfoRepositoryImpl) UpdateGroupInfo(group *entity.GroupInfo) error {
	return r.db.Save(group).Error
}

func (r *groupInfoRepositoryImpl) GetGroupInfoByUUID(uuid string) (*entity.GroupInfo, error) {
	var g entity.GroupInfo
	err := r.db.Where("uuid = ?", uuid).First(&g).Error
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *groupInfoRepositoryImpl) ListByOwnerID(ownerID string) ([]entity.GroupInfo, error) {
	var groups []entity.GroupInfo
	err := r.db.Where("owner_id = ? AND status = 0", ownerID).Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}