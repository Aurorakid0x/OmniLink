package handler

import (
	chatRequest "OmniLink/internal/modules/chat/application/dto/request"
	"OmniLink/internal/modules/chat/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
)

type SessionHandler struct {
	svc service.SessionService
}

func NewSessionHandler(svc service.SessionService) *SessionHandler {
	return &SessionHandler{svc: svc}
}

func (h *SessionHandler) CheckOpenSessionAllowed(c *gin.Context) {
	var req chatRequest.OpenSessionRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		if req.SendId != "" && req.SendId != uuid {
			back.Error(c, xerr.Forbidden, "send_id 不匹配")
			return
		}
		req.SendId = uuid
	}

	allowed, err := h.svc.CheckOpenSessionAllowed(req)
	back.Result(c, allowed, err)
}
func (h *SessionHandler) GetUserSessionList(c *gin.Context) {
	var req chatRequest.GetUserSessionListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		if req.OwnerId != "" && req.OwnerId != uuid {
			back.Error(c, xerr.Forbidden, "owner_id 不匹配")
			return
		}
		req.OwnerId = uuid
	}

	data, err := h.svc.GetUserSessionList(req.OwnerId)
	back.Result(c, data, err)
}

func (h *SessionHandler) OpenSession(c *gin.Context) {
	var req chatRequest.OpenSessionRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		if req.SendId != "" && req.SendId != uuid {
			back.Error(c, xerr.Forbidden, "send_id 不匹配")
			return
		}
		req.SendId = uuid
	}

	data, err := h.svc.OpenSession(req)
	back.Result(c, data, err)
}
