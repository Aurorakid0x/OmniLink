package service

import (
	"context"
	"fmt"
	"time"

	"OmniLink/internal/modules/ai/infrastructure/mcp/registry"
	"OmniLink/internal/modules/ai/infrastructure/mcp/types"
	"OmniLink/pkg/zlog"
)

// MCPDispatcher MCP 调度器接口
type MCPDispatcher interface {
	// ListAvailableTools 列出所有可用工具
	ListAvailableTools(ctx context.Context, tenantUserID string) ([]types.ToolDescriptor, error)

	// CallTool 调用指定工具
	CallTool(ctx context.Context, req *ToolCallRequest) (*ToolCallResponse, error)

	// GetToolByName 根据名称查找工具
	GetToolByName(ctx context.Context, name string) (*types.ToolDescriptor, error)
}

// ToolCallRequest 工具调用请求
type ToolCallRequest struct {
	TenantUserID string                 // 租户用户 ID (必填)
	ToolName     string                 // 工具名称 (必填)
	Arguments    map[string]interface{} // 工具参数
	Timeout      int                    // 超时时间 (秒)，0 表示使用默认值
}

// ToolCallResponse 工具调用响应
type ToolCallResponse struct {
	Success  bool                   // 是否成功
	Content  string                 // 文本内容 (给 LLM 看的)
	Metadata map[string]interface{} // 结构化数据 (可选)
	Error    string                 // 错误信息 (如果失败)
}

// mcpDispatcherImpl MCP 调度器实现
type mcpDispatcherImpl struct {
	registry       *registry.ServerRegistry
	defaultTimeout time.Duration
}

// NewMCPDispatcher 创建 MCP Dispatcher
func NewMCPDispatcher(reg *registry.ServerRegistry, defaultTimeoutSeconds int) MCPDispatcher {
	timeout := 30 * time.Second
	if defaultTimeoutSeconds > 0 {
		timeout = time.Duration(defaultTimeoutSeconds) * time.Second
	}

	return &mcpDispatcherImpl{
		registry:       reg,
		defaultTimeout: timeout,
	}
}

// ListAvailableTools 列出所有可用工具
func (d *mcpDispatcherImpl) ListAvailableTools(ctx context.Context, tenantUserID string) ([]types.ToolDescriptor, error) {
	servers := d.registry.ListServers()
	if len(servers) == 0 {
		return []types.ToolDescriptor{}, nil
	}

	allTools := make([]types.ToolDescriptor, 0)

	// 遍历所有 Server，收集工具
	for _, srv := range servers {
		if srv.Status != "running" {
			continue
		}

		tools, err := srv.Server.ListTools(ctx)
		if err != nil {
			zlog.Error(fmt.Sprintf("MCP: Failed to list tools from server '%s': %v", srv.Name, err))
			continue
		}

		allTools = append(allTools, tools...)
	}

	return allTools, nil
}

// CallTool 调用指定工具
func (d *mcpDispatcherImpl) CallTool(ctx context.Context, req *ToolCallRequest) (*ToolCallResponse, error) {
	// 1. 参数校验
	if req.ToolName == "" {
		return nil, fmt.Errorf("tool name is required")
	}

	// 2. 设置超时
	timeout := d.defaultTimeout
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 3. 注入 tenant_user_id 到参数中（如果不存在）
	if req.Arguments == nil {
		req.Arguments = make(map[string]interface{})
	}
	if _, exists := req.Arguments["tenant_user_id"]; !exists {
		req.Arguments["tenant_user_id"] = req.TenantUserID
	}

	// 4. 路由到正确的 Server 并调用工具
	result, err := d.routeAndCallTool(ctxWithTimeout, req.ToolName, req.Arguments)
	if err != nil {
		return &ToolCallResponse{
			Success: false,
			Content: "",
			Error:   err.Error(),
		}, nil
	}

	// 5. 转换为 ToolCallResponse
	return d.convertToResponse(result), nil
}

// GetToolByName 根据名称查找工具
func (d *mcpDispatcherImpl) GetToolByName(ctx context.Context, name string) (*types.ToolDescriptor, error) {
	servers := d.registry.ListServers()

	for _, srv := range servers {
		if srv.Status != "running" {
			continue
		}

		tools, err := srv.Server.ListTools(ctx)
		if err != nil {
			continue
		}

		for _, tool := range tools {
			if tool.Name == name {
				return &tool, nil
			}
		}
	}

	return nil, types.ErrToolNotFound
}

// routeAndCallTool 路由并调用工具
func (d *mcpDispatcherImpl) routeAndCallTool(ctx context.Context, toolName string, args map[string]interface{}) (*types.CallToolResult, error) {
	servers := d.registry.ListServers()

	// 简单路由策略：遍历所有 Server，找到第一个包含该工具的 Server
	for _, srv := range servers {
		if srv.Status != "running" {
			continue
		}

		// 尝试调用工具
		result, err := srv.Server.CallTool(ctx, toolName, args)
		if err != nil {
			// 如果是工具不存在，继续尝试下一个 Server
			if mcpErr, ok := err.(*types.MCPError); ok && mcpErr.Code == types.ErrCodeToolNotFound {
				continue
			}
			// 其他错误直接返回
			return nil, err
		}

		// 调用成功
		return result, nil
	}

	// 所有 Server 都没有找到该工具
	return nil, types.NewMCPError(types.ErrCodeToolNotFound, fmt.Sprintf("tool '%s' not found in any server", toolName))
}

// convertToResponse 转换 MCP 结果为响应
func (d *mcpDispatcherImpl) convertToResponse(result *types.CallToolResult) *ToolCallResponse {
	if result.IsError {
		return &ToolCallResponse{
			Success:  false,
			Content:  "",
			Metadata: result.Metadata,
			Error:    extractTextContent(result.Content),
		}
	}

	return &ToolCallResponse{
		Success:  true,
		Content:  extractTextContent(result.Content),
		Metadata: result.Metadata,
		Error:    "",
	}
}

// extractTextContent 提取文本内容
func extractTextContent(contents []types.Content) string {
	if len(contents) == 0 {
		return ""
	}

	// 拼接所有 text 类型的内容
	var text string
	for _, content := range contents {
		if content.Type == "text" {
			text += content.Text
		}
	}

	return text
}
