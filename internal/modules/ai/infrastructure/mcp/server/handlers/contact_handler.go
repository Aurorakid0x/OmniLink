package handlers

import (
	"context"
	"fmt"
	"strings"

	"OmniLink/pkg/zlog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"go.uber.org/zap"

	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	contactRespond "OmniLink/internal/modules/contact/application/dto/respond"
	contactService "OmniLink/internal/modules/contact/application/service"
)

// ContactToolHandler 好友/联系人工具处理器
type ContactToolHandler struct {
	contactSvc contactService.ContactService
}

// NewContactToolHandler 创建 ContactToolHandler
func NewContactToolHandler(svc contactService.ContactService) *ContactToolHandler {
	return &ContactToolHandler{
		contactSvc: svc,
	}
}

// RegisterTools 注册所有好友相关工具到 Server
func (h *ContactToolHandler) RegisterTools(s *server.MCPServer) {
	// 注册 contact_list_friends 工具
	tool := mcp.NewTool("contact_list_friends",
		mcp.WithDescription("获取用户的好友列表，返回好友基本信息（用户名、头像、状态）"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID（必填，从上下文获取）")),
	)

	s.AddTool(tool, h.handleListFriends)
}

// handleListFriends 处理获取好友列表
func (h *ContactToolHandler) handleListFriends(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// 1. 参数校验
	var args map[string]interface{}
	var ok bool

	// mcp-go server 接收到的 Arguments 通常是 map[string]interface{}
	if args, ok = request.Params.Arguments.(map[string]interface{}); !ok {
		zlog.Error("contact_list_friends invalid arguments type")
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || strings.TrimSpace(tenantUserID) == "" {
		zlog.Error("contact_list_friends missing tenant_user_id")
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}

	zlog.Info("contact_list_friends start", zap.String("tenant_user_id", tenantUserID))

	// 2. 调用 ContactService 获取好友列表
	friends, err := h.contactSvc.GetUserList(contactRequest.GetUserListRequest{
		OwnerId: tenantUserID,
	})
	if err != nil {
		zlog.Error("contact_list_friends query failed", zap.Error(err), zap.String("tenant_user_id", tenantUserID))
		return mcp.NewToolResultError(fmt.Sprintf("查询好友列表失败：%v", err)), nil
	}

	zlog.Info("contact_list_friends result", zap.String("tenant_user_id", tenantUserID), zap.Any("friends", friends))

	// 3. 转换为人类可读文本
	textContent := h.formatFriendsAsText(friends, tenantUserID)

	// 4. 返回 MCP 结果
	return mcp.NewToolResultText(textContent), nil
}

// formatFriendsAsText 将好友列表格式化为文本
func (h *ContactToolHandler) formatFriendsAsText(friends interface{}, tenantUserID string) string {
	switch list := friends.(type) {
	case []contactRespond.UserListItem:
		if len(list) == 0 {
			return "您当前还没有添加任何好友。"
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("找到 %d 位好友：\n", len(list)))
		for i := range list {
			statusText := "离线"
			if list[i].Status == 0 {
				statusText = "在线"
			}
			sb.WriteString(fmt.Sprintf("%d. %s (ID: %s) - %s\n", i+1, list[i].UserName, list[i].UserId, statusText))
		}
		return sb.String()
	}

	friendList, ok := friends.([]interface{})
	if !ok {
		if list, ok := friends.([]map[string]interface{}); ok {
			friendList = make([]interface{}, 0, len(list))
			for i := range list {
				friendList = append(friendList, list[i])
			}
		} else {
			return "找到好友列表，但无法解析返回格式。"
		}
	}

	if len(friendList) == 0 {
		return "您当前还没有添加任何好友。"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("找到 %d 位好友：\n", len(friendList)))

	for i, friend := range friendList {
		friendMap, ok := friend.(map[string]interface{})
		if !ok {
			continue
		}

		userName := ""
		if v, ok := friendMap["user_name"].(string); ok {
			userName = v
		}
		userId := ""
		if v, ok := friendMap["user_id"].(string); ok {
			userId = v
		}
		statusVal := int64(1)
		if v, ok := friendMap["status"]; ok {
			switch s := v.(type) {
			case int:
				statusVal = int64(s)
			case int8:
				statusVal = int64(s)
			case int32:
				statusVal = int64(s)
			case int64:
				statusVal = s
			case float64:
				statusVal = int64(s)
			}
		}

		statusText := "离线"
		if statusVal == 0 {
			statusText = "在线"
		}

		sb.WriteString(fmt.Sprintf("%d. %s (ID: %s) - %s\n", i+1, userName, userId, statusText))
	}

	return sb.String()
}
