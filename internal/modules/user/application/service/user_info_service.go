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
	"context"
	"errors"
	"strings"
	"time"

	aiService "OmniLink/internal/modules/ai/application/service"

	"gorm.io/gorm"
)

// UserInfoService 接口定义 (Application Service)
type UserInfoService interface {
	Register(registerReq request.RegisterRequest) (*respond.RegisterRespond, error)
	Login(loginReq request.LoginRequest) (*respond.LoginRespond, error)
	GetUserInfoInternal(ctx context.Context, uuid string) (*respond.InternalUserInfoRespond, error)
}

type userInfoServiceImpl struct {
	repo         repository.UserInfoRepository
	lifecycleSvc aiService.UserLifecycleService // AI模块：用户生命周期服务
}

// NewUserInfoService 构造函数
func NewUserInfoService(repo repository.UserInfoRepository, lifecycleSvc aiService.UserLifecycleService) UserInfoService {
	return &userInfoServiceImpl{
		repo:         repo,
		lifecycleSvc: lifecycleSvc,
	}
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

	// ==================== AI模块集成点 ====================
	// 用户注册成功后，初始化AI助手（创建全局Agent和系统会话）
	// 注意：此处失败不影响注册流程，仅记录日志
	if u.lifecycleSvc != nil {
		if err := u.lifecycleSvc.InitializeUserAIAssistant(context.Background(), newUser.Uuid); err != nil {
			zlog.Error("用户AI助手初始化失败，用户UUID: " + newUser.Uuid + ", 错误: " + err.Error())
			// 不返回错误，不阻断注册流程
		}
	}
	// =====================================================

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

	// ==================== AI模块兜底初始化 ====================
	// 登录时兜底初始化系统全局AI助手（避免注册时失败导致缺失）
	if u.lifecycleSvc != nil {
		if err := u.lifecycleSvc.InitializeUserAIAssistant(context.Background(), user.Uuid); err != nil {
			zlog.Error("用户AI助手初始化失败，用户UUID: " + user.Uuid + ", 错误: " + err.Error())
		}
	}
	// =====================================================

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

func (u *userInfoServiceImpl) GetUserInfoInternal(ctx context.Context, uuid string) (*respond.InternalUserInfoRespond, error) {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return nil, xerr.New(xerr.BadRequest, "uuid is required")
	}

	user, err := u.repo.GetUserInfoByUUIDWithoutPassword(uuid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.BadRequest, "用户不存在")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	lastOnlineAt := ""
	if user.LastOnlineAt.Valid {
		lastOnlineAt = user.LastOnlineAt.Time.Format("2006-01-02 15:04:05")
	}
	lastOfflineAt := ""
	if user.LastOfflineAt.Valid {
		lastOfflineAt = user.LastOfflineAt.Time.Format("2006-01-02 15:04:05")
	}

	return &respond.InternalUserInfoRespond{
		Id:            user.Id,
		Uuid:          user.Uuid,
		Username:      user.Username,
		Nickname:      user.Nickname,
		Avatar:        user.Avatar,
		Gender:        user.Gender,
		Signature:     user.Signature,
		Birthday:      user.Birthday,
		CreatedAt:     user.CreatedAt.Format("2006-01-02 15:04:05"),
		LastOnlineAt:  lastOnlineAt,
		LastOfflineAt: lastOfflineAt,
		IsAdmin:       user.IsAdmin,
		Status:        user.Status,
	}, nil
}
