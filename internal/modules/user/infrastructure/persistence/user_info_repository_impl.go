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

func (r *userInfoRepositoryImpl) GetUserInfoByUUID(uuid string) (*entity.UserInfo, error) {
	var user entity.UserInfo
	// Omit password for security
	err := r.db.Omit("password").Where("uuid = ?", uuid).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userInfoRepositoryImpl) GetBatchUserInfo(uuids []string) ([]entity.UserInfo, error) {
	if len(uuids) == 0 {
		return []entity.UserInfo{}, nil
	}
	var users []entity.UserInfo
	// Omit password for security
	err := r.db.Omit("password").Where("uuid IN ?", uuids).Find(&users).Error
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
