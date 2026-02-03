package handlers

import (
	"context"
	"time"

	"OmniLink/pkg/ws"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type NotificationToolHandler struct {
	hub *ws.Hub
}

func NewNotificationToolHandler(hub *ws.Hub) *NotificationToolHandler {
	return &NotificationToolHandler{hub: hub}
}

func (h *NotificationToolHandler) RegisterTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("push_notification",
		mcp.WithDescription("主动向用户推送通知消息。当需要提醒用户、发送日报或主动告知信息时使用。"),
		mcp.WithString("content", mcp.Required(), mcp.Description("推送的消息内容")),
	), h.handlePushNotification)
}

func (h *NotificationToolHandler) handlePushNotification(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content cannot be empty"), nil
	}
	var userID, agentID string
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
	if userID == "" {
		return mcp.NewToolResultError("user_id not found in context"), nil
	}
	if h.hub == nil {
		return mcp.NewToolResultError("ws hub not configured"), nil
	}
	payload := map[string]interface{}{
		"agent_id": agentID,
		"content":  content,
		"time":     time.Now().Unix(),
	}
	if err := h.hub.SendJSON(userID, map[string]interface{}{
		"type":    "ai_notification",
		"payload": payload,
	}); err != nil {
		return mcp.NewToolResultError("failed to push notification: " + err.Error()), nil
	}
	return mcp.NewToolResultText("Notification pushed successfully"), nil
}
