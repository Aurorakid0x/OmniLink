package registry

import (
	"context"
	"fmt"
	"sync"

	"OmniLink/internal/modules/ai/infrastructure/mcp/server"
	"OmniLink/pkg/zlog"
)

// ServerRegistry MCP Server 注册表
type ServerRegistry struct {
	servers map[string]*RegisteredServer
	mu      sync.RWMutex
}

// RegisteredServer 已注册的 Server
type RegisteredServer struct {
	Name     string
	Type     string // builtin | external
	Priority int
	Status   string // running | stopped | error
	Server   server.MCPServer
}

// NewServerRegistry 创建 Server Registry
func NewServerRegistry() *ServerRegistry {
	return &ServerRegistry{
		servers: make(map[string]*RegisteredServer),
	}
}

// RegisterBuiltinServer 注册内置 Server
func (r *ServerRegistry) RegisterBuiltinServer(name string, srv server.MCPServer, priority int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.servers[name]; exists {
		return fmt.Errorf("server '%s' already registered", name)
	}

	r.servers[name] = &RegisteredServer{
		Name:     name,
		Type:     "builtin",
		Priority: priority,
		Status:   "running",
		Server:   srv,
	}

	zlog.Info(fmt.Sprintf("MCP: Registered builtin server '%s' with priority %d", name, priority))
	return nil
}

// GetServerByName 根据名称获取 Server
func (r *ServerRegistry) GetServerByName(name string) (*RegisteredServer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	srv, exists := r.servers[name]
	if !exists {
		return nil, fmt.Errorf("server '%s' not found", name)
	}

	return srv, nil
}

// ListServers 列出所有已注册的 Server
func (r *ServerRegistry) ListServers() []*RegisteredServer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	servers := make([]*RegisteredServer, 0, len(r.servers))
	for _, srv := range r.servers {
		servers = append(servers, srv)
	}

	return servers
}

// StartAll 启动所有 Server
func (r *ServerRegistry) StartAll(ctx context.Context) error {
	// 内置 Server 无需启动（直接调用）
	// 外部 Server 需要启动子进程或建立连接（暂不实现）
	zlog.Info("MCP: All servers started")
	return nil
}

// StopAll 停止所有 Server
func (r *ServerRegistry) StopAll(ctx context.Context) error {
	// 内置 Server 无需停止
	// 外部 Server 需要关闭连接或停止子进程（暂不实现）
	zlog.Info("MCP: All servers stopped")
	return nil
}
