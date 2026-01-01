package handler

import (
	chatRequest "OmniLink/internal/modules/chat/application/dto/request"
	"OmniLink/internal/modules/chat/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
)

type MessageHandler struct {
	svc service.MessageService
}

func NewMessageHandler(svc service.MessageService) *MessageHandler {
	return &MessageHandler{svc: svc}
}

func (h *MessageHandler) GetMessageList(c *gin.Context) {
	var req chatRequest.GetMessageListRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	if uuid := c.GetString("uuid"); uuid != "" {
		if req.UserOneId != "" && req.UserOneId != uuid {
			back.Error(c, xerr.Forbidden, "user_one_id 不匹配")
			return
		}
		req.UserOneId = uuid
	}

	data, err := h.svc.GetMessageList(req)
	back.Result(c, data, err)
}
