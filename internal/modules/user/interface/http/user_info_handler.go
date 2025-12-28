package handler

import (
	"OmniLink/internal/modules/user/application/dto/request"
	"OmniLink/internal/modules/user/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/constants"
	"OmniLink/pkg/zlog"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type UserInfoHandler struct {
	svc service.UserInfoService
}

func NewUserInfoHandler(svc service.UserInfoService) *UserInfoHandler {
	return &UserInfoHandler{svc: svc}
}

func (h *UserInfoHandler) Register(c *gin.Context) {
	var registerReq request.RegisterRequest
	if err := c.BindJSON(&registerReq); err != nil {
		zlog.Error(err.Error())
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": constants.SYSTEM_ERROR,
		})
		return
	}
	fmt.Println(registerReq)
	message, userInfo, ret := h.svc.Register(registerReq)
	back.JsonBack(c, message, ret, userInfo)
}
