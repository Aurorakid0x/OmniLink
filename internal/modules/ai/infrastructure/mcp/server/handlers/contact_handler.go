package handlers

import (
	"context"
	"fmt"
	"strings"

	"OmniLink/internal/modules/ai/infrastructure/mcp/types"
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

// RegisterTools 注册所有好友相关工具
func (h *ContactToolHandler) RegisterTools() []types.ToolInfo {
	return []types.ToolInfo{
		{
			Descriptor: types.ToolDescriptor{
				Name:        "contact_list_friends",
				Description: "获取用户的好友列表，返回好友基本信息（用户名、头像、状态）",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"tenant_user_id": map[string]string{
							"type":        "string",
							"description": "租户用户ID（必填，从上下文获取）",
						},
					},
					"required": []string{"tenant_user_id"},
				},
			},
			Handler: h.handleListFriends,
		},
		// 可以在这里添加更多工具：contact_get_info, contact_apply 等
	}
}

// handleListFriends 处理获取好友列表
func (h *ContactToolHandler) handleListFriends(ctx context.Context, args map[string]interface{}) (*types.CallToolResult, error) {
	// 1. 参数校验
	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || strings.TrimSpace(tenantUserID) == "" {
		return nil, types.NewMCPError(types.ErrCodeInvalidParams, "tenant_user_id is required and must be a non-empty string")
	}

	// 2. 调用 ContactService 获取好友列表
	friends, err := h.contactSvc.GetUserList(contactRequest.GetUserListRequest{
		OwnerId: tenantUserID,
	})
	if err != nil {
		return &types.CallToolResult{
			Content: []types.Content{
				{
					Type: "text",
					Text: fmt.Sprintf("查询好友列表失败：%v", err),
				},
			},
			IsError: true,
		}, nil
	}

	// 3. 转换为人类可读文本
	textContent := h.formatFriendsAsText(friends, tenantUserID)

	// 4. 返回 MCP 结果
	return &types.CallToolResult{
		Content: []types.Content{
			{
				Type: "text",
				Text: textContent,
			},
		},
		Metadata: map[string]interface{}{
			"friends": friends,
			"total":   len(friends),
		},
		IsError: false,
	}, nil
}

// formatFriendsAsText 将好友列表格式化为文本
func (h *ContactToolHandler) formatFriendsAsText(friends interface{}, tenantUserID string) string {
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
