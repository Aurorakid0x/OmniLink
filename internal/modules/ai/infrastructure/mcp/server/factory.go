package server

import (
	"github.com/mark3labs/mcp-go/server"

	mcpHandlers "OmniLink/internal/modules/ai/infrastructure/mcp/server/handlers"
	chatService "OmniLink/internal/modules/chat/application/service"
	contactService "OmniLink/internal/modules/contact/application/service"
)

// BuiltinServerConfig 内置服务器配置
type BuiltinServerConfig struct {
	Name               string
	Version            string
	EnableContactTools bool
	EnableGroupTools   bool
	EnableMessageTools bool
	EnableSessionTools bool
}

// BuiltinServerDependencies 内置服务器依赖
type BuiltinServerDependencies struct {
	ContactSvc contactService.ContactService
	GroupSvc   contactService.GroupService
	MessageSvc chatService.MessageService
	SessionSvc chatService.SessionService
}

// NewBuiltinMCPServer 创建并配置内置 MCP Server
func NewBuiltinMCPServer(conf BuiltinServerConfig, deps BuiltinServerDependencies) *server.MCPServer {
	// 创建 Server 实例
	s := server.NewMCPServer(
		conf.Name,
		conf.Version,
		server.WithToolCapabilities(true),
	)

	// 注册工具
	if conf.EnableContactTools && deps.ContactSvc != nil && deps.GroupSvc != nil {
		contactHandler := mcpHandlers.NewContactToolHandler(deps.ContactSvc, deps.GroupSvc)
		contactHandler.RegisterTools(s)
	}

	// TODO: 注册其他工具
	// if conf.EnableGroupTools && deps.GroupSvc != nil { ... }

	return s
}
