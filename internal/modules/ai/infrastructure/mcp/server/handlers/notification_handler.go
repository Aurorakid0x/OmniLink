package handlers

import (
	"context"
	"fmt"
	"time"

	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/ws"
	"OmniLink/pkg/zlog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"
)

type NotificationToolHandler struct {
	hub         *ws.Hub
	messageRepo repository.AssistantMessageRepository
	sessionRepo repository.AssistantSessionRepository
}

func NewNotificationToolHandler(hub *ws.Hub, messageRepo repository.AssistantMessageRepository, sessionRepo repository.AssistantSessionRepository) *NotificationToolHandler {
	return &NotificationToolHandler{
		hub:         hub,
		messageRepo: messageRepo,
		sessionRepo: sessionRepo,
	}
}

func (h *NotificationToolHandler) RegisterTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("push_notification",
		mcp.WithDescription("主动向用户推送通知消息。当需要提醒用户、发送日报或主动告知信息时使用。消息会自动保存并推送到用户前端。"),
		mcp.WithString("content", mcp.Required(), mcp.Description("推送的消息内容")),
	), h.handlePushNotification)
}

// handlePushNotification 处理消息推送请求
// 核心逻辑：
// 1. 验证参数和上下文（UserID, SessionID）
// 2. 将消息保存到 ai_assistant_message 表，确保历史记录完整
// 3. 构造包含完整元数据（AgentID, SessionID等）的 Payload
// 4. 通过 WebSocket 推送 ai_notification 类型的消息给前端
func (h *NotificationToolHandler) handlePushNotification(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content cannot be empty"), nil
	}
	var userID, agentID, sessionID string
	if v := ctx.Value("tenant_user_id"); v != nil {
		if value, ok := v.(string); ok {
			userID = value
		}
	}
	if v := ctx.Value("agent_id"); v != nil {
		if value, ok := v.(string); ok {
			agentID = value
		}
	}
	if v := ctx.Value("session_id"); v != nil {
		if value, ok := v.(string); ok {
			sessionID = value
		}
	}
	if userID == "" {
		zlog.Warn("push_notification missing user_id")
		return mcp.NewToolResultError("user_id not found in context"), nil
	}
	if h.hub == nil {
		zlog.Warn("push_notification hub is nil")
		return mcp.NewToolResultError("ws hub not configured"), nil
	}

	zlog.Info("push_notification start",
		zap.String("user_id", userID),
		zap.String("session_id", sessionID),
		zap.String("agent_id", agentID))

	// 1. 保存消息到数据库 (role=assistant)
	now := time.Now()
	msg := &assistant.AIAssistantMessage{
		SessionId:     sessionID,
		Role:          "assistant",
		Content:       content,
		CitationsJson: "{}",
		TokensJson:    "{}",
		CreatedAt:     now,
	}
	// 如果没有sessionID，尝试获取一个默认的或跳过保存（但通常应该有）
	if sessionID != "" && h.messageRepo != nil {
		if err := h.messageRepo.SaveMessage(ctx, msg); err != nil {
			zlog.Error("push_notification failed to save message", zap.Error(err))
			// 不阻断推送，但记录错误
		} else {
			zlog.Info("push_notification message saved", zap.Int64("msg_id", msg.Id))
			// 更新会话时间
			if h.sessionRepo != nil {
				_ = h.sessionRepo.UpdateSessionUpdatedAt(ctx, sessionID)
			}
		}
	} else {
		zlog.Warn("push_notification skip save", zap.String("session_id", sessionID), zap.Bool("repo_nil", h.messageRepo == nil))
	}

	// 2. WebSocket推送
	payload := map[string]interface{}{
		"uuid":       fmt.Sprintf("ai_%d", now.UnixNano()),
		"agent_id":   agentID,
		"session_id": sessionID,
		"send_id":    agentID,
		"receive_id": userID,
		"send_name":  "AI助手", // TODO: 从AgentRepo获取真实名称
		"type":       0,
		"content":    content,
		"role":       "assistant",
		"created_at": now.Format(time.RFC3339),
		"time":       now.Unix(),
	}

	// 推送 AI 专用消息类型，前端需要兼容处理
	// 或者，如果前端已经能处理标准IM消息，我们可以直接推送标准类型？
	// 这里为了区分，还是用 ai_notification，但在前端转换为标准消息处理
	if err := h.hub.SendJSON(userID, map[string]interface{}{
		"type":    "ai_notification",
		"payload": payload,
	}); err != nil {
		return mcp.NewToolResultError("failed to push notification: " + err.Error()), nil
	}

	return mcp.NewToolResultText("Notification pushed and saved successfully"), nil
}
