package service

import (
	"OmniLink/internal/modules/user/application/dto/request"
	"OmniLink/internal/modules/user/application/dto/respond"
	"OmniLink/internal/modules/user/domain/entity"
	"OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"fmt"
	"time"
)

// UserInfoService 接口定义 (Application Service)
type UserInfoService interface {
	Register(registerReq request.RegisterRequest) (*respond.RegisterRespond, error)
}

type userInfoServiceImpl struct {
	repo repository.UserInfoRepository
}

// NewUserInfoService 构造函数
func NewUserInfoService(repo repository.UserInfoRepository) UserInfoService {
	return &userInfoServiceImpl{repo: repo}
}

func (u *userInfoServiceImpl) Register(registerReq request.RegisterRequest) (*respond.RegisterRespond, error) {
	// 1. Check if user exists (only username)
	_, err := u.repo.GetUserInfoByUsername(registerReq.Username)
	if err == nil {
		return nil, xerr.New(xerr.BadRequest, "用户已存在")
	}

	// 2. Generate UUID
	uuid := fmt.Sprintf("%d", time.Now().UnixNano())

	// 3. Create UserInfo
	newUser := entity.UserInfo{
		Uuid:      uuid,
		Username:  registerReq.Username,
		Nickname:  registerReq.Nickname,
		Password:  registerReq.Password,
		Avatar:    "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png",
		Status:    0,
		IsAdmin:   0,
		CreatedAt: time.Now(),
	}

	err = u.repo.CreateUserInfo(&newUser)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	return &respond.RegisterRespond{
		Uuid:     newUser.Uuid,
		Username: newUser.Username,
		Nickname: newUser.Nickname,
		Avatar:   newUser.Avatar,
	}, nil
}
