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

func (r *groupInfoRepositoryImpl) ListJoinedGroups(userID string) ([]entity.GroupInfo, error) {
	var groups []entity.GroupInfo
	// Join user_contact and group_info
	// user_contact.contact_id -> group_info.uuid
	err := r.db.Table("group_info").
		Joins("JOIN user_contact ON group_info.uuid = user_contact.contact_id").
		Where("user_contact.user_id = ? AND user_contact.contact_type = 1 AND user_contact.status NOT IN ?", userID, []int8{6, 7}). // 6: quit, 7: kicked
		Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// SearchGroupsByName 根据群名模糊搜索群组
func (r *groupInfoRepositoryImpl) SearchGroupsByName(keyword string, limit int) ([]entity.GroupInfo, error) {
	if keyword == "" {
		return []entity.GroupInfo{}, nil
	}
	if limit <= 0 {
		limit = 10 // 默认限制10条
	}

	var groups []entity.GroupInfo
	// 模糊搜索群名，status=0表示正常群组
	err := r.db.Where("status = 0 AND name LIKE ?", "%"+keyword+"%").
		Limit(limit).
		Find(&groups).Error
	if err != nil {
		return nil, err
	}
	return groups, nil
}

// FindGroupByExactName 根据精确群名查找群组
func (r *groupInfoRepositoryImpl) FindGroupByExactName(name string) (*entity.GroupInfo, error) {
	if name == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var group entity.GroupInfo
	// 精确匹配群名
	err := r.db.Where("status = 0 AND name = ?", name).First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}
