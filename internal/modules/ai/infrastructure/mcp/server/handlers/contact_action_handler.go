package handlers

import (
	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	contactService "OmniLink/internal/modules/contact/application/service"
	"OmniLink/pkg/zlog"
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ContactActionToolHandler 处理需要确认的操作类工具
type ContactActionToolHandler struct {
	contactSvc contactService.ContactService
	groupSvc   contactService.GroupService
}

func NewContactActionToolHandler(
	contactSvc contactService.ContactService,
	groupSvc contactService.GroupService,
) *ContactActionToolHandler {
	return &ContactActionToolHandler{
		contactSvc: contactSvc,
		groupSvc:   groupSvc,
	}
}
func (h *ContactActionToolHandler) RegisterTools(s *server.MCPServer) {
	// 同意好友申请工具
	passFriendApplyTool := mcp.NewTool("contact_pass_friend_apply",
		mcp.WithDescription("同意好友申请。**需要用户确认后才会执行**"),
		mcp.WithString("apply_id", mcp.Required(), mcp.Description("好友申请ID")),
		mcp.WithBoolean("confirmed", mcp.Description("用户是否已确认(第一次调用不传,返回确认信息后再次调用时传true)")),
	)
	s.AddTool(passFriendApplyTool, h.handlePassFriendApply)
	// 拒绝好友申请工具
	refuseFriendApplyTool := mcp.NewTool("contact_refuse_friend_apply",
		mcp.WithDescription("拒绝好友申请。**需要用户确认后才会执行**"),
		mcp.WithString("apply_id", mcp.Required(), mcp.Description("好友申请ID")),
		mcp.WithBoolean("confirmed", mcp.Description("用户是否已确认")),
	)
	s.AddTool(refuseFriendApplyTool, h.handleRefuseFriendApply)
	// 加入群聊工具(申请)
	joinGroupTool := mcp.NewTool("group_join",
		mcp.WithDescription("申请加入群聊。**需要用户确认后才会执行**"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID")),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("群组ID")),
		mcp.WithString("message", mcp.Description("申请消息")),
		mcp.WithBoolean("confirmed", mcp.Description("用户是否已确认")),
	)
	s.AddTool(joinGroupTool, h.handleJoinGroup)
	// 退出群聊工具
	leaveGroupTool := mcp.NewTool("group_leave",
		mcp.WithDescription("退出群聊。**需要用户确认后才会执行**"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID")),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("群组ID")),
		mcp.WithBoolean("confirmed", mcp.Description("用户是否已确认")),
	)
	s.AddTool(leaveGroupTool, h.handleLeaveGroup)
}

// handlePassFriendApply 处理同意好友申请
func (h *ContactActionToolHandler) handlePassFriendApply(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	applyID, ok := args["apply_id"].(string)
	if !ok || applyID == "" {
		return mcp.NewToolResultError("apply_id is required"), nil
	}
	// 检查是否已确认
	confirmed, _ := args["confirmed"].(bool)

	if !confirmed {
		// 第一次调用:返回需要确认的信息
		return mcp.NewToolResultJSON(map[string]interface{}{
			"requires_confirmation": true,
			"action":                "同意好友申请",
			"apply_id":              applyID,
			"message":               fmt.Sprintf("确认同意好友申请 (ID: %s) 吗?", applyID),
			"next_step":             "请用户明确回复'确认'、'是'或'yes'后,再次调用此工具并传入 confirmed=true",
		})
	}
	// 第二次调用(用户已确认):执行实际操作
	// TODO: 从上下文获取 tenant_user_id
	tenantUserID := "获取当前用户ID的逻辑"

	err := h.contactSvc.PassContactApply(contactRequest.PassContactApplyRequest{
		OwnerId: tenantUserID,
		ApplyId: applyID,
	})

	if err != nil {
		zlog.Error("PassContactApply failed: " + err.Error())
		return mcp.NewToolResultError(fmt.Sprintf("同意好友申请失败: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]interface{}{
		"success":  true,
		"message":  "已成功同意好友申请",
		"apply_id": applyID,
	})
}

// handleRefuseFriendApply 处理拒绝好友申请
func (h *ContactActionToolHandler) handleRefuseFriendApply(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	applyID, ok := args["apply_id"].(string)
	if !ok || applyID == "" {
		return mcp.NewToolResultError("apply_id is required"), nil
	}
	confirmed, _ := args["confirmed"].(bool)

	if !confirmed {
		return mcp.NewToolResultJSON(map[string]interface{}{
			"requires_confirmation": true,
			"action":                "拒绝好友申请",
			"apply_id":              applyID,
			"message":               fmt.Sprintf("确认拒绝好友申请 (ID: %s) 吗?", applyID),
			"next_step":             "请用户明确回复'确认'、'是'或'yes'后,再次调用此工具并传入 confirmed=true",
		})
	}
	tenantUserID := "获取当前用户ID的逻辑"

	err := h.contactSvc.RefuseContactApply(contactRequest.RefuseContactApplyRequest{
		OwnerId: tenantUserID,
		ApplyId: applyID,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("拒绝好友申请失败: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]interface{}{
		"success": true,
		"message": "已成功拒绝好友申请",
	})
}

// handleJoinGroup 处理加入群聊
func (h *ContactActionToolHandler) handleJoinGroup(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	groupID, ok := args["group_id"].(string)
	if !ok || groupID == "" {
		return mcp.NewToolResultError("group_id is required"), nil
	}
	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || tenantUserID == "" {
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}
	message, _ := args["message"].(string)
	confirmed, _ := args["confirmed"].(bool)

	if !confirmed {
		return mcp.NewToolResultJSON(map[string]interface{}{
			"requires_confirmation": true,
			"action":                "申请加入群聊",
			"group_id":              groupID,
			"message":               fmt.Sprintf("确认申请加入群聊 (ID: %s) 吗?", groupID),
			"next_step":             "请用户明确回复'确认'后,再次调用此工具并传入 confirmed=true",
		})
	}
	// 调用 ApplyContact 服务(群聊申请)
	_, err := h.contactSvc.ApplyContact(contactRequest.ApplyContactRequest{
		OwnerId:   tenantUserID,
		ContactId: groupID,
		Message:   message,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("申请加入群聊失败: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]interface{}{
		"success": true,
		"message": "已成功提交入群申请,等待群主审批",
	})
}

// handleLeaveGroup 处理退出群聊
func (h *ContactActionToolHandler) handleLeaveGroup(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	groupID, ok := args["group_id"].(string)
	if !ok || groupID == "" {
		return mcp.NewToolResultError("group_id is required"), nil
	}
	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || tenantUserID == "" {
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}
	confirmed, _ := args["confirmed"].(bool)

	if !confirmed {
		return mcp.NewToolResultJSON(map[string]interface{}{
			"requires_confirmation": true,
			"action":                "退出群聊",
			"group_id":              groupID,
			"message":               fmt.Sprintf("确认退出群聊 (ID: %s) 吗?退出后将无法查看群消息", groupID),
			"next_step":             "请用户明确回复'确认'后,再次调用此工具并传入 confirmed=true",
		})
	}
	err := h.groupSvc.LeaveGroup(contactRequest.LeaveGroupRequest{
		OwnerId: tenantUserID,
		GroupId: groupID,
	})

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("退出群聊失败: %v", err)), nil
	}
	return mcp.NewToolResultJSON(map[string]interface{}{
		"success": true,
		"message": "已成功退出群聊",
	})
}
