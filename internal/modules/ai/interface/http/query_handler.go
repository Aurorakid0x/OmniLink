package http

import (
	aiRequest "OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// QueryHandler RAG 召回查询 HTTP Handler
type QueryHandler struct {
	retrieveSvc service.RetrieveService
}

// NewQueryHandler 创建 RAG 召回查询 Handler
func NewQueryHandler(retrieveSvc service.RetrieveService) *QueryHandler {
	return &QueryHandler{retrieveSvc: retrieveSvc}
}

// Query 处理 RAG 召回查询请求
//
// 路由: POST /ai/rag/query
// 鉴权: 需要 JWT（从 authed 分组继承）
// 请求体: RAGQueryRequest
// 响应体: RAGQueryRespond
func (h *QueryHandler) Query(c *gin.Context) {
	var req aiRequest.RAGQueryRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error(err.Error())
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}
	// 从 JWT 中提取 tenant_user_id
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}
	// 调用 Service 执行 RAG 召回
	data, err := h.retrieveSvc.Query(c.Request.Context(), req, uuid)
	zlog.Info("rag query result", zap.Any("data", data), zap.Error(err))
	back.Result(c, data, err)
}
