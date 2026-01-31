package repository

import (
	contact "OmniLink/internal/modules/contact/domain/entity"
	"OmniLink/internal/modules/user/domain/entity"
)

// UserInfoRepository 接口定义
type UserInfoRepository interface {
	CreateUserInfo(user *entity.UserInfo) error
	GetUserInfoById(id int64) (*entity.UserInfo, error)
	GetUserInfoByUsername(username string) (*entity.UserInfo, error)
	GetUserInfoByUUIDWithoutPassword(uuid string) (*entity.UserInfo, error)
	GetBatchUserInfoWithoutPassword(uuids []string) ([]entity.UserInfo, error)
	GetUserBriefByUUIDs(uuids []string) ([]entity.UserBrief, error)
	GetUserContactInfoByUUIDs(uuids []string) ([]contact.UserContactInfo, error)
	// SearchUsersByNickname 根据昵称模糊搜索用户（支持用户名降级）
	SearchUsersByNickname(keyword string, limit int) ([]entity.UserBrief, error)
	// FindUserByExactNickname 根据精确昵称查找用户（支持用户名降级）
	FindUserByExactNickname(nickname string) (*entity.UserBrief, error)
}
