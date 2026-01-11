package http

import (
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"strings"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	ingestSvc service.IngestService
}

func NewAdminHandler(ingestSvc service.IngestService) *AdminHandler {
	return &AdminHandler{ingestSvc: ingestSvc}
}

type backfillRequest struct {
	UserID            string `json:"user_id"`
	PageSize          int    `json:"page_size"`
	MaxSessions       int    `json:"max_sessions"`
	MaxPagesPerSession int   `json:"max_pages_per_session"`
	Since             string `json:"since"`
	Until             string `json:"until"`
}

func (h *AdminHandler) Backfill(c *gin.Context) {
	var req backfillRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	userID := strings.TrimSpace(req.UserID)
	if userID != "" && userID != uuid {
		back.Error(c, xerr.Forbidden, "user_id 不匹配")
		return
	}
	if userID == "" {
		userID = uuid
	}

	data, err := h.ingestSvc.Backfill(c.Request.Context(), service.BackfillRequest{
		TenantUserID:      userID,
		PageSize:          req.PageSize,
		MaxSessions:       req.MaxSessions,
		MaxPagesPerSession: req.MaxPagesPerSession,
		Since:             req.Since,
		Until:             req.Until,
	})
	back.Result(c, data, err)
}
