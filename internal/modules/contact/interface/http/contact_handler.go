package handler

import (
	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	"OmniLink/internal/modules/contact/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/ws"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"time"

	"github.com/gin-gonic/gin"
)

type ContactHandler struct {
	svc service.ContactService
	hub *ws.Hub
}

func NewContactHandler(svc service.ContactService, hub *ws.Hub) *ContactHandler {
	return &ContactHandler{svc: svc, hub: hub}
}

func (h *ContactHandler) GetUserList(c *gin.Context) {
	var req contactRequest.GetUserListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		req.OwnerId = uuid
	}

	data, err := h.svc.GetUserList(req)
	back.Result(c, data, err)
}

func (h *ContactHandler) GetContactInfo(c *gin.Context) {
	var req contactRequest.GetContactInfoRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		req.OwnerId = uuid
	}

	data, err := h.svc.GetContactInfo(req)
	back.Result(c, data, err)
}

func (h *ContactHandler) ApplyContact(c *gin.Context) {
	var req contactRequest.ApplyContactRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		req.OwnerId = uuid
	}

	data, err := h.svc.ApplyContact(req)
	if err == nil && h.hub != nil && req.ContactId != "" {
		_ = h.hub.SendJSON(req.ContactId, map[string]interface{}{
			"type":        "contact.apply",
			"apply_id":    data.ApplyId,
			"from_user_id": req.OwnerId,
			"created_at":  time.Now().Format(time.RFC3339),
		})
	}
	back.Result(c, data, err)
}

func (h *ContactHandler) GetNewContactList(c *gin.Context) {
	var req contactRequest.GetNewContactListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		req.OwnerId = uuid
	}

	data, err := h.svc.GetNewContactList(req)
	back.Result(c, data, err)
}

func (h *ContactHandler) PassContactApply(c *gin.Context) {
	var req contactRequest.PassContactApplyRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		req.OwnerId = uuid
	}

	err := h.svc.PassContactApply(req)
	back.Result(c, nil, err)
}

func (h *ContactHandler) RefuseContactApply(c *gin.Context) {
	var req contactRequest.RefuseContactApplyRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		req.OwnerId = uuid
	}

	err := h.svc.RefuseContactApply(req)
	back.Result(c, nil, err)
}
