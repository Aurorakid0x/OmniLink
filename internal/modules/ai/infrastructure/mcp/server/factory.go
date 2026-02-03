package server

import (
	aiService "OmniLink/internal/modules/ai/application/service"
	aiRepository "OmniLink/internal/modules/ai/domain/repository"
	mcpHandlers "OmniLink/internal/modules/ai/infrastructure/mcp/server/handlers"
	chatService "OmniLink/internal/modules/chat/application/service"
	contactService "OmniLink/internal/modules/contact/application/service"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/ws"

	"github.com/mark3labs/mcp-go/server"
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
	UserRepo   userRepository.UserInfoRepository
	GroupRepo  contactRepository.GroupInfoRepository
	JobSvc     aiService.AIJobService
	AgentRepo  aiRepository.AgentRepository
	WsHub      *ws.Hub
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
	if conf.EnableContactTools && deps.ContactSvc != nil && deps.GroupSvc != nil && deps.UserRepo != nil && deps.GroupRepo != nil {
		contactHandler := mcpHandlers.NewContactToolHandler(deps.ContactSvc, deps.GroupSvc, deps.UserRepo, deps.GroupRepo)
		contactHandler.RegisterTools(s)
	}

	// TODO: 注册其他工具
	// if conf.EnableGroupTools && deps.GroupSvc != nil { ... }
	// ✨ 注册操作类工具(新增)
	if conf.EnableContactTools && deps.ContactSvc != nil && deps.GroupSvc != nil {
		actionHandler := mcpHandlers.NewContactActionToolHandler(deps.ContactSvc, deps.GroupSvc)
		actionHandler.RegisterTools(s)
	}

	if deps.JobSvc != nil && deps.AgentRepo != nil {
		jobHandler := mcpHandlers.NewJobManagementHandler(deps.JobSvc, deps.AgentRepo)
		jobHandler.RegisterTools(s)
	}

	if deps.WsHub != nil {
		notificationHandler := mcpHandlers.NewNotificationToolHandler(deps.WsHub)
		notificationHandler.RegisterTools(s)
	}

	return s
}
