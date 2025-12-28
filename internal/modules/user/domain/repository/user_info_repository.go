package repository

import "OmniLink/internal/modules/user/domain/entity"

// UserInfoRepository 接口定义
type UserInfoRepository interface {
	CreateUserInfo(user *entity.UserInfo) error
	GetUserInfoById(id int64) (*entity.UserInfo, error)
	GetUserInfoByUsername(username string) (*entity.UserInfo, error)
}
