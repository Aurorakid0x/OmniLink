package server

import (
	"context"
	"fmt"
	"sync"

	"OmniLink/internal/modules/ai/infrastructure/mcp/types"
	"OmniLink/pkg/zlog"
)

// MCPServer MCP Server 接口
type MCPServer interface {
	// GetServerInfo 获取 Server 基本信息
	GetServerInfo() types.ServerInfo

	// ListTools 列出所有工具
	ListTools(ctx context.Context) ([]types.ToolDescriptor, error)

	// CallTool 执行工具调用
	CallTool(ctx context.Context, name string, args map[string]interface{}) (*types.CallToolResult, error)
}

// BuiltinMCPServer 内置 MCP Server 实现
type BuiltinMCPServer struct {
	info  types.ServerInfo
	tools map[string]*types.ToolInfo // toolName -> ToolInfo
	mu    sync.RWMutex
}

// NewBuiltinMCPServer 创建内置 MCP Server
func NewBuiltinMCPServer(name, version, description string) *BuiltinMCPServer {
	return &BuiltinMCPServer{
		info: types.ServerInfo{
			Name:        name,
			Version:     version,
			Description: description,
		},
		tools: make(map[string]*types.ToolInfo),
	}
}

// RegisterTool 注册单个工具
func (s *BuiltinMCPServer) RegisterTool(tool types.ToolInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tools[tool.Descriptor.Name] = &tool
	zlog.Info(fmt.Sprintf("MCP: Registered tool '%s'", tool.Descriptor.Name))
}

// RegisterTools 批量注册工具
func (s *BuiltinMCPServer) RegisterTools(tools []types.ToolInfo) {
	for _, tool := range tools {
		s.RegisterTool(tool)
	}
}

// GetServerInfo 获取 Server 基本信息
func (s *BuiltinMCPServer) GetServerInfo() types.ServerInfo {
	return s.info
}

// ListTools 列出所有工具
func (s *BuiltinMCPServer) ListTools(ctx context.Context) ([]types.ToolDescriptor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	descriptors := make([]types.ToolDescriptor, 0, len(s.tools))
	for _, toolInfo := range s.tools {
		descriptors = append(descriptors, toolInfo.Descriptor)
	}

	return descriptors, nil
}

// CallTool 执行工具调用
func (s *BuiltinMCPServer) CallTool(ctx context.Context, name string, args map[string]interface{}) (*types.CallToolResult, error) {
	s.mu.RLock()
	toolInfo, exists := s.tools[name]
	s.mu.RUnlock()

	if !exists {
		return nil, types.NewMCPError(types.ErrCodeToolNotFound, fmt.Sprintf("tool '%s' not found", name))
	}

	// 调用工具处理函数
	zlog.Info(fmt.Sprintf("MCP: Calling tool '%s' with args: %v", name, args))
	result, err := toolInfo.Handler(ctx, args)
	if err != nil {
		zlog.Error(fmt.Sprintf("MCP: Tool '%s' execution failed: %v", name, err))
		return nil, err
	}

	return result, nil
}

// GetToolByName 根据名称获取工具描述符
func (s *BuiltinMCPServer) GetToolByName(name string) (*types.ToolDescriptor, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	toolInfo, exists := s.tools[name]
	if !exists {
		return nil, types.ErrToolNotFound
	}

	return &toolInfo.Descriptor, nil
}
