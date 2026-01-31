package persistence

import (
	contact "OmniLink/internal/modules/contact/domain/entity"
	"OmniLink/internal/modules/user/domain/entity"
	"OmniLink/internal/modules/user/domain/repository"

	"gorm.io/gorm"
)

// userInfoRepositoryImpl 结构体
type userInfoRepositoryImpl struct {
	db *gorm.DB
}

// NewUserInfoRepository 构造函数
func NewUserInfoRepository(db *gorm.DB) repository.UserInfoRepository {
	return &userInfoRepositoryImpl{db: db}
}

// 1. 实现 CreateUserInfo
func (r *userInfoRepositoryImpl) CreateUserInfo(user *entity.UserInfo) error {
	return r.db.Create(user).Error
}

// 2. 实现 GetUserInfoById
func (r *userInfoRepositoryImpl) GetUserInfoById(id int64) (*entity.UserInfo, error) {
	var user entity.UserInfo
	// First 查不到会返回 ErrRecordNotFound
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 3. 实现 GetUserInfoByUsername
func (r *userInfoRepositoryImpl) GetUserInfoByUsername(username string) (*entity.UserInfo, error) {
	var user entity.UserInfo
	err := r.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userInfoRepositoryImpl) GetUserInfoByUUIDWithoutPassword(uuid string) (*entity.UserInfo, error) {
	var user entity.UserInfo
	// Use Select to explicitly fetch safe fields, excluding password
	err := r.db.Select("id, uuid, username, nickname, avatar, gender, signature, birthday, created_at, last_online_at, last_offline_at, is_admin, status").
		Where("uuid = ?", uuid).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userInfoRepositoryImpl) GetBatchUserInfoWithoutPassword(uuids []string) ([]entity.UserInfo, error) {
	if len(uuids) == 0 {
		return []entity.UserInfo{}, nil
	}
	var users []entity.UserInfo
	// Use Select to explicitly fetch safe fields, excluding password
	err := r.db.Select("id, uuid, username, nickname, avatar, gender, signature, birthday, created_at, last_online_at, last_offline_at, is_admin, status").
		Where("uuid IN ?", uuids).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userInfoRepositoryImpl) GetUserBriefByUUIDs(uuids []string) ([]entity.UserBrief, error) {
	if len(uuids) == 0 {
		return []entity.UserBrief{}, nil
	}

	var users []entity.UserBrief
	err := r.db.Model(&entity.UserInfo{}).
		Select("uuid", "username", "nickname", "avatar", "status").
		Where("uuid IN ?", uuids).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *userInfoRepositoryImpl) GetUserContactInfoByUUIDs(uuids []string) ([]contact.UserContactInfo, error) {
	if len(uuids) == 0 {
		return []contact.UserContactInfo{}, nil
	}

	var users []contact.UserContactInfo
	err := r.db.Model(&entity.UserInfo{}).
		Select("uuid", "username", "nickname", "avatar", "signature", "gender", "birthday", "status").
		Where("uuid IN ?", uuids).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// SearchUsersByNickname 根据昵称模糊搜索用户（支持用户名降级）
func (r *userInfoRepositoryImpl) SearchUsersByNickname(keyword string, limit int) ([]entity.UserBrief, error) {
	if keyword == "" {
		return []entity.UserBrief{}, nil
	}
	if limit <= 0 {
		limit = 10 // 默认限制10条
	}

	var users []entity.UserBrief
	// 模糊搜索昵称或用户名，status=0表示正常用户
	err := r.db.Model(&entity.UserInfo{}).
		Select("uuid", "username", "nickname", "avatar", "status").
		Where("status = 0 AND (nickname LIKE ? OR username LIKE ?)", "%"+keyword+"%", "%"+keyword+"%").
		Limit(limit).
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

// FindUserByExactNickname 根据精确昵称查找用户（支持用户名降级）
func (r *userInfoRepositoryImpl) FindUserByExactNickname(nickname string) (*entity.UserBrief, error) {
	if nickname == "" {
		return nil, gorm.ErrRecordNotFound
	}

	var user entity.UserBrief
	// 优先按昵称精确匹配，如果没有则按用户名精确匹配
	err := r.db.Model(&entity.UserInfo{}).
		Select("uuid", "username", "nickname", "avatar", "status").
		Where("status = 0 AND (nickname = ? OR username = ?)", nickname, nickname).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
