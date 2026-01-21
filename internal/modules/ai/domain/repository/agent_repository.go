package repository

import (
	"context"

	"OmniLink/internal/modules/ai/domain/agent"
)

// AgentRepository AI Agent仓储接口
type AgentRepository interface {
	// CreateAgent 创建新Agent
	CreateAgent(ctx context.Context, ag *agent.AIAgent) error

	// GetAgentByID 根据agent_id和owner_id获取Agent（权限隔离：仅返回owner拥有的或系统Agent）
	GetAgentByID(ctx context.Context, agentId, ownerId string) (*agent.AIAgent, error)

	// ListAgents 获取用户的Agent列表（包含用户自己的+系统的）
	ListAgents(ctx context.Context, ownerId string, limit, offset int) ([]*agent.AIAgent, error)

	// UpdateAgent 更新Agent（支持部分字段更新）
	UpdateAgent(ctx context.Context, agentId, ownerId string, updates map[string]interface{}) error

	// DisableAgent 禁用Agent
	DisableAgent(ctx context.Context, agentId, ownerId string) error
}
