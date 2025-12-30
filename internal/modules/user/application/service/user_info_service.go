package service

import (
	"OmniLink/internal/modules/user/application/dto/request"
	"OmniLink/internal/modules/user/application/dto/respond"
	"OmniLink/internal/modules/user/domain/entity"
	"OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/util"
	"OmniLink/pkg/util/myjwt"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"errors"
	"time"

	"gorm.io/gorm"
)

// UserInfoService 接口定义 (Application Service)
type UserInfoService interface {
	Register(registerReq request.RegisterRequest) (*respond.RegisterRespond, error)
	Login(loginReq request.LoginRequest) (*respond.LoginRespond, error)
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
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	// 2. Generate UUID
	uuid := util.GenerateUserID()
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

	token, err := myjwt.GenerateToken(newUser.Uuid, newUser.Username)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	return &respond.RegisterRespond{
		Uuid:      newUser.Uuid,
		Username:  newUser.Username,
		Nickname:  newUser.Nickname,
		Avatar:    newUser.Avatar,
		Gender:    newUser.Gender,
		Birthday:  newUser.Birthday,
		Signature: newUser.Signature,
		CreatedAt: newUser.CreatedAt.Format("2006-01-02 15:04:05"),
		IsAdmin:   newUser.IsAdmin,
		Status:    newUser.Status,
		Token:     token,
	}, nil
}

func (u *userInfoServiceImpl) Login(loginReq request.LoginRequest) (*respond.LoginRespond, error) {
	user, err := u.repo.GetUserInfoByUsername(loginReq.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.BadRequest, "用户不存在")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	if user.Status != 0 {
		return nil, xerr.New(xerr.Forbidden, "用户已被禁用")
	}

	if user.Password != loginReq.Password {
		return nil, xerr.New(xerr.BadRequest, "密码错误")
	}

	token, err := myjwt.GenerateToken(user.Uuid, user.Username)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	return &respond.LoginRespond{
		Uuid:      user.Uuid,
		Username:  user.Username,
		Nickname:  user.Nickname,
		Avatar:    user.Avatar,
		Gender:    user.Gender,
		Birthday:  user.Birthday,
		Signature: user.Signature,
		CreatedAt: user.CreatedAt.Format("2006-01-02 15:04:05"),
		IsAdmin:   user.IsAdmin,
		Status:    user.Status,
		Token:     token,
	}, nil
}
