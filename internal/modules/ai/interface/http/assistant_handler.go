package http

import (
	"encoding/json"
	"fmt"
	"strings"

	aiRequest "OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AssistantHandler AI助手HTTP Handler
type AssistantHandler struct {
	svc service.AssistantService
}

// NewAssistantHandler 创建AssistantHandler
func NewAssistantHandler(svc service.AssistantService) *AssistantHandler {
	return &AssistantHandler{svc: svc}
}

// Chat 处理AI助手聊天请求（非流式）
//
// 路由: POST /ai/assistant/chat
// 鉴权: 需要JWT（从authed分组继承）
// 请求体: AssistantChatRequest
// 响应体: AssistantChatRespond
func (h *AssistantHandler) Chat(c *gin.Context) {
	var req aiRequest.AssistantChatRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error("assistant chat bind error", zap.Error(err))
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	// 从JWT中提取tenant_user_id
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	// 调用Service执行聊天
	data, err := h.svc.Chat(c.Request.Context(), req, uuid)
	if err != nil {
		zlog.Error("assistant chat failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}

// ChatStream 处理AI助手流式聊天请求（SSE）
//
// 路由: POST /ai/assistant/chat/stream
// 鉴权: 需要JWT
// 请求体: AssistantChatRequest
// 响应: SSE流
func (h *AssistantHandler) ChatStream(c *gin.Context) {
	var req aiRequest.AssistantChatRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error("assistant chat stream bind error", zap.Error(err))
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	// 从JWT中提取tenant_user_id
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// 调用Service获取流式channel
	eventChan, err := h.svc.ChatStream(c.Request.Context(), req, uuid)
	if err != nil {
		zlog.Error("assistant chat stream failed", zap.Error(err), zap.String("uuid", uuid))
		// 发送错误事件
		c.SSEvent("error", map[string]string{"error": err.Error()})
		c.Writer.Flush()
		return
	}

	// 读取事件并发送SSE
	for event := range eventChan {
		switch event.Event {
		case "delta":
			// 流式token输出
			c.SSEvent("delta", event.Data)
			c.Writer.Flush()
		case "done":
			// 完成事件（包含完整信息）
			c.SSEvent("done", event.Data)
			c.Writer.Flush()
		case "error":
			// 错误事件
			c.SSEvent("error", event.Data)
			c.Writer.Flush()
			return
		}
	}

	zlog.Info("assistant chat stream completed", zap.String("uuid", uuid))
}

// ListSessions 获取AI助手会话列表
//
// 路由: GET /ai/assistant/sessions
// 鉴权: 需要JWT
// 查询参数: limit, offset
// 响应体: AssistantSessionListRespond
func (h *AssistantHandler) ListSessions(c *gin.Context) {
	// 从JWT中提取tenant_user_id
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	// 解析查询参数
	limit := 20
	offset := 0
	if l, ok := c.GetQuery("limit"); ok {
		if n, err := parsePositiveInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o, ok := c.GetQuery("offset"); ok {
		if n, err := parsePositiveInt(o); err == nil && n >= 0 {
			offset = n
		}
	}

	// 调用Service
	data, err := h.svc.ListSessions(c.Request.Context(), uuid, limit, offset)
	if err != nil {
		zlog.Error("assistant list sessions failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}

// ListAgents 获取Agent列表
//
// 路由: GET /ai/assistant/agents
// 鉴权: 需要JWT
// 查询参数: limit, offset
// 响应体: AssistantAgentListRespond
func (h *AssistantHandler) ListAgents(c *gin.Context) {
	// 从JWT中提取tenant_user_id
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	// 解析查询参数
	limit := 20
	offset := 0
	if l, ok := c.GetQuery("limit"); ok {
		if n, err := parsePositiveInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o, ok := c.GetQuery("offset"); ok {
		if n, err := parsePositiveInt(o); err == nil && n >= 0 {
			offset = n
		}
	}

	// 调用Service
	data, err := h.svc.ListAgents(c.Request.Context(), uuid, limit, offset)
	if err != nil {
		zlog.Error("assistant list agents failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}

// GetSessionMessages 获取会话历史消息列表
//
// 路由: GET /ai/assistant/sessions/:session_id/messages
// 鉴权: 需要JWT
// 路径参数: session_id
// 查询参数: limit, offset
// 响应体: AssistantMessageListRespond
func (h *AssistantHandler) GetSessionMessages(c *gin.Context) {
	// 从JWT中提取tenant_user_id
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	// 从路径参数获取session_id
	sessionID := strings.TrimSpace(c.Param("session_id"))
	if sessionID == "" {
		back.Error(c, xerr.BadRequest, "session_id is required")
		return
	}

	// 解析查询参数
	limit := 20
	offset := 0
	if l, ok := c.GetQuery("limit"); ok {
		if n, err := parsePositiveInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o, ok := c.GetQuery("offset"); ok {
		if n, err := parsePositiveInt(o); err == nil && n >= 0 {
			offset = n
		}
	}

	// 调用Service
	data, err := h.svc.GetSessionMessages(c.Request.Context(), sessionID, uuid, limit, offset)
	if err != nil {
		zlog.Error("assistant get session messages failed", zap.Error(err), zap.String("uuid", uuid), zap.String("session_id", sessionID))
	}
	back.Result(c, data, err)
}

// CreateAgent 创建Agent
//
// 路由: POST /ai/assistant/agents
// 鉴权: 需要JWT
// 请求体: CreateAgentRequest
// 响应体: CreateAgentRespond
func (h *AssistantHandler) CreateAgent(c *gin.Context) {
	var req aiRequest.CreateAgentRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error("create agent bind error", zap.Error(err))
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	data, err := h.svc.CreateAgent(c.Request.Context(), req, uuid)
	if err != nil {
		zlog.Error("create agent failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}

// CreateSession 创建会话
//
// 路由: POST /ai/assistant/sessions
// 鉴权: 需要JWT
// 请求体: CreateSessionRequest
// 响应体: CreateSessionRespond
func (h *AssistantHandler) CreateSession(c *gin.Context) {
	var req aiRequest.CreateSessionRequest
	if err := c.BindJSON(&req); err != nil {
		zlog.Error("create session bind error", zap.Error(err))
		back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
		return
	}

	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	data, err := h.svc.CreateSession(c.Request.Context(), req, uuid)
	if err != nil {
		zlog.Error("create session failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}

// parsePositiveInt 解析正整数
func parsePositiveInt(s string) (int, error) {
	var n int
	if err := json.Unmarshal([]byte(s), &n); err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("negative number")
	}
	return n, nil
}
