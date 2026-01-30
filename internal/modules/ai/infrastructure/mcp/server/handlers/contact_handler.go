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
	contactService "OmniLink/internal/modules/contact/application/service"
)

// ContactToolHandler 好友/联系人工具处理器
type ContactToolHandler struct {
	contactSvc contactService.ContactService
	groupSvc   contactService.GroupService
}

// NewContactToolHandler 创建 ContactToolHandler
func NewContactToolHandler(contactSvc contactService.ContactService, groupSvc contactService.GroupService) *ContactToolHandler {
	return &ContactToolHandler{
		contactSvc: contactSvc,
		groupSvc:   groupSvc,
	}
}

// RegisterTools 注册所有好友/联系人/群组相关工具到 Server
func (h *ContactToolHandler) RegisterTools(s *server.MCPServer) {
	// 注册 contact_list_friends 工具
	listFriendsTool := mcp.NewTool("contact_list_friends",
		mcp.WithDescription("获取用户的好友列表，返回好友基本信息（用户名、头像、状态）JSON数据"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID（必填，从上下文获取）")),
	)
	s.AddTool(listFriendsTool, h.handleListFriends)

	// 注册 contact_get_info 工具
	getContactInfoTool := mcp.NewTool("contact_get_info",
		mcp.WithDescription("获取指定联系人的详细信息，包括头像、签名、性别、生日等JSON数据"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID（必填，从上下文获取）")),
		mcp.WithString("contact_id", mcp.Required(), mcp.Description("联系人ID（必填）")),
	)
	s.AddTool(getContactInfoTool, h.handleGetContactInfo)

	// 注册 contact_get_new_list 工具
	getNewContactListTool := mcp.NewTool("contact_get_new_list",
		mcp.WithDescription("获取待处理的好友申请列表，包括申请人信息、申请消息等JSON数据"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID（必填，从上下文获取）")),
	)
	s.AddTool(getNewContactListTool, h.handleGetNewContactList)

	// 注册 contact_my_groups 工具
	myGroupsTool := mcp.NewTool("contact_my_groups",
		mcp.WithDescription("获取用户已加入的群组列表，返回群组基本信息JSON数据"),
		mcp.WithString("tenant_user_id", mcp.Required(), mcp.Description("租户用户ID（必填，从上下文获取）")),
	)
	s.AddTool(myGroupsTool, h.handleLoadMyJoinedGroups)

	// 注册 group_get_info 工具
	getGroupInfoTool := mcp.NewTool("group_get_info",
		mcp.WithDescription("获取指定群组的详细信息，包括群名、公告、成员数、群主等JSON数据"),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("群组ID（必填）")),
	)
	s.AddTool(getGroupInfoTool, h.handleGetGroupInfo)

	// 注册 group_get_members 工具
	getGroupMembersTool := mcp.NewTool("group_get_members",
		mcp.WithDescription("获取指定群组的成员列表，包括成员的用户名、昵称、头像、角色等JSON数据"),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("群组ID（必填）")),
	)
	s.AddTool(getGroupMembersTool, h.handleGetGroupMembers)
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

	zlog.Info("contact_list_friends result", zap.String("tenant_user_id", tenantUserID), zap.Int("count", len(friends)))

	// 3. 返回 JSON 结果
	return mcp.NewToolResultJSON(friends)
}

// handleGetContactInfo 处理获取联系人详情
func (h *ContactToolHandler) handleGetContactInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]interface{}
	var ok bool

	if args, ok = request.Params.Arguments.(map[string]interface{}); !ok {
		zlog.Error("contact_get_info invalid arguments type")
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || strings.TrimSpace(tenantUserID) == "" {
		zlog.Error("contact_get_info missing tenant_user_id")
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}

	contactID, ok := args["contact_id"].(string)
	if !ok || strings.TrimSpace(contactID) == "" {
		zlog.Error("contact_get_info missing contact_id")
		return mcp.NewToolResultError("contact_id is required"), nil
	}

	zlog.Info("contact_get_info start", zap.String("tenant_user_id", tenantUserID), zap.String("contact_id", contactID))

	contactInfo, err := h.contactSvc.GetContactInfo(contactRequest.GetContactInfoRequest{
		OwnerId:   tenantUserID,
		ContactId: contactID,
	})
	if err != nil {
		zlog.Error("contact_get_info query failed", zap.Error(err))
		return mcp.NewToolResultError(fmt.Sprintf("查询联系人详情失败：%v", err)), nil
	}

	zlog.Info("contact_get_info result", zap.String("contact_id", contactID))
	return mcp.NewToolResultJSON(contactInfo)
}

// handleGetNewContactList 处理获取好友申请列表
func (h *ContactToolHandler) handleGetNewContactList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]interface{}
	var ok bool

	if args, ok = request.Params.Arguments.(map[string]interface{}); !ok {
		zlog.Error("contact_get_new_list invalid arguments type")
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || strings.TrimSpace(tenantUserID) == "" {
		zlog.Error("contact_get_new_list missing tenant_user_id")
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}

	zlog.Info("contact_get_new_list start", zap.String("tenant_user_id", tenantUserID))

	newContactList, err := h.contactSvc.GetNewContactList(contactRequest.GetNewContactListRequest{
		OwnerId: tenantUserID,
	})
	if err != nil {
		zlog.Error("contact_get_new_list query failed", zap.Error(err))
		return mcp.NewToolResultError(fmt.Sprintf("查询好友申请列表失败：%v", err)), nil
	}

	zlog.Info("contact_get_new_list result", zap.Int("count", len(newContactList)))
	return mcp.NewToolResultJSON(newContactList)
}

// handleLoadMyJoinedGroups 处理获取已加入群组列表
func (h *ContactToolHandler) handleLoadMyJoinedGroups(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]interface{}
	var ok bool

	if args, ok = request.Params.Arguments.(map[string]interface{}); !ok {
		zlog.Error("contact_my_groups invalid arguments type")
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	tenantUserID, ok := args["tenant_user_id"].(string)
	if !ok || strings.TrimSpace(tenantUserID) == "" {
		zlog.Error("contact_my_groups missing tenant_user_id")
		return mcp.NewToolResultError("tenant_user_id is required"), nil
	}

	zlog.Info("contact_my_groups start", zap.String("tenant_user_id", tenantUserID))

	groups, err := h.contactSvc.LoadMyJoinedGroup(contactRequest.LoadMyJoinedGroupRequest{
		OwnerId: tenantUserID,
	})
	if err != nil {
		zlog.Error("contact_my_groups query failed", zap.Error(err))
		return mcp.NewToolResultError(fmt.Sprintf("查询已加入群组列表失败：%v", err)), nil
	}

	zlog.Info("contact_my_groups result", zap.Int("count", len(groups)))
	return mcp.NewToolResultJSON(groups)
}

// handleGetGroupInfo 处理获取群组信息
func (h *ContactToolHandler) handleGetGroupInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]interface{}
	var ok bool

	if args, ok = request.Params.Arguments.(map[string]interface{}); !ok {
		zlog.Error("group_get_info invalid arguments type")
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	groupID, ok := args["group_id"].(string)
	if !ok || strings.TrimSpace(groupID) == "" {
		zlog.Error("group_get_info missing group_id")
		return mcp.NewToolResultError("group_id is required"), nil
	}

	zlog.Info("group_get_info start", zap.String("group_id", groupID))

	groupInfo, err := h.groupSvc.GetGroupInfo(contactRequest.GetGroupInfoRequest{
		GroupId: groupID,
	})
	if err != nil {
		zlog.Error("group_get_info query failed", zap.Error(err))
		return mcp.NewToolResultError(fmt.Sprintf("查询群组信息失败：%v", err)), nil
	}

	zlog.Info("group_get_info result", zap.String("group_id", groupID))
	return mcp.NewToolResultJSON(groupInfo)
}

// handleGetGroupMembers 处理获取群成员列表
func (h *ContactToolHandler) handleGetGroupMembers(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]interface{}
	var ok bool

	if args, ok = request.Params.Arguments.(map[string]interface{}); !ok {
		zlog.Error("group_get_members invalid arguments type")
		return mcp.NewToolResultError("invalid arguments format, expected map"), nil
	}

	groupID, ok := args["group_id"].(string)
	if !ok || strings.TrimSpace(groupID) == "" {
		zlog.Error("group_get_members missing group_id")
		return mcp.NewToolResultError("group_id is required"), nil
	}

	zlog.Info("group_get_members start", zap.String("group_id", groupID))

	members, err := h.groupSvc.GetGroupMemberList(contactRequest.GetGroupMemberListRequest{
		GroupId: groupID,
	})
	if err != nil {
		zlog.Error("group_get_members query failed", zap.Error(err))
		return mcp.NewToolResultError(fmt.Sprintf("查询群成员列表失败：%v", err)), nil
	}

	zlog.Info("group_get_members result", zap.String("group_id", groupID), zap.Int("count", len(members)))
	return mcp.NewToolResultJSON(members)
}
