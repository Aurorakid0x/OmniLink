package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
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
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || strings.TrimSpace(tenantUserID) == "" {
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}

	// 2. 调用 ContactService 获取好友列表
	friends, err := h.contactSvc.GetUserList(contactRequest.GetUserListRequest{
		OwnerId: tenantUserID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("查询好友列表失败：%v", err)), nil
	}

	// 3. 转换为人类可读文本
	textContent := h.formatFriendsAsText(friends, tenantUserID)

	// 4. 返回 MCP 结果
	return mcp.NewToolResultText(textContent), nil
}

// formatFriendsAsText 将好友列表格式化为文本
func (h *ContactToolHandler) formatFriendsAsText(friends interface{}, tenantUserID string) string {
	// ... (保持原有逻辑不变)
	// 类型断言
	friendList, ok := friends.([]interface{})
	if !ok {
		// 尝试其他类型
		return fmt.Sprintf("找到好友列表，共 %d 位好友", 0)
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

		userName, _ := friendMap["UserName"].(string)
		userId, _ := friendMap["UserId"].(string)
		status, _ := friendMap["Status"].(int8)

		statusText := "离线"
		if status == 0 {
			statusText = "在线"
		}

		sb.WriteString(fmt.Sprintf("%d. %s (ID: %s) - %s\n", i+1, userName, userId, statusText))
	}

	return sb.String()
}
