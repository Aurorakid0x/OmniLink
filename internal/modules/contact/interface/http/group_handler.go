package handler

import (
	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	"OmniLink/internal/modules/contact/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	svc service.GroupService
}

func NewGroupHandler(svc service.GroupService) *GroupHandler {
	return &GroupHandler{svc: svc}
}

func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req contactRequest.CreateGroupRequest
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

	data, err := h.svc.CreateGroup(req)
	back.Result(c, data, err)
}

func (h *GroupHandler) GetGroupInfo(c *gin.Context) {
	var req contactRequest.GetGroupInfoRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}
	data, err := h.svc.GetGroupInfo(req)
	back.Result(c, data, err)
}

func (h *GroupHandler) GetGroupMemberList(c *gin.Context) {
	var req contactRequest.GetGroupMemberListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}
	data, err := h.svc.GetGroupMemberList(req)
	back.Result(c, data, err)
}

func (h *GroupHandler) InviteGroupMembers(c *gin.Context) {
	var req contactRequest.InviteGroupMembersRequest
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

	err := h.svc.InviteGroupMembers(req)
	back.Result(c, nil, err)
}