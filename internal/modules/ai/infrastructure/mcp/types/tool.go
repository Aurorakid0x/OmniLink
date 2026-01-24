package types

import "context"

// ToolDescriptor 工具描述符
type ToolDescriptor struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolHandler 工具处理函数
type ToolHandler func(ctx context.Context, args map[string]interface{}) (*CallToolResult, error)

// ToolInfo 内部工具信息（包含 handler）
type ToolInfo struct {
	Descriptor ToolDescriptor
	Handler    ToolHandler
}
