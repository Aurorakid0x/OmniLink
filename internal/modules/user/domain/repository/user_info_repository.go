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
}
