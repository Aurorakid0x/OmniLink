package handler

import (
	"OmniLink/internal/modules/user/application/dto/request"
	"OmniLink/internal/modules/user/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"fmt"

	//"net/http"

	"github.com/gin-gonic/gin"
)

type UserInfoHandler struct {
	svc service.UserInfoService
}

func NewUserInfoHandler(svc service.UserInfoService) *UserInfoHandler {
	return &UserInfoHandler{svc: svc}
}

func (h *UserInfoHandler) Login(c *gin.Context) {
	var loginReq request.LoginRequest
	if err := c.BindJSON(&loginReq); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}
	data, err := h.svc.Login(loginReq)
	back.Result(c, data, err)
}

func (h *UserInfoHandler) Register(c *gin.Context) {
	var registerReq request.RegisterRequest
	if err := c.BindJSON(&registerReq); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}
	fmt.Println(registerReq)
	data, err := h.svc.Register(registerReq)
	back.Result(c, data, err)
}
