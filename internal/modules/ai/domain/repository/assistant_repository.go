package repository

import (
	"context"

	"OmniLink/internal/modules/ai/domain/assistant"
)

// AssistantSessionRepository AI助手会话仓储接口
type AssistantSessionRepository interface {
	// CreateSession 创建新会话
	CreateSession(ctx context.Context, session *assistant.AIAssistantSession) error

	// GetSessionByID 根据session_id和tenant_user_id获取会话（权限隔离）
	GetSessionByID(ctx context.Context, sessionId, tenantUserId string) (*assistant.AIAssistantSession, error)

	// ListSessions 获取用户的会话列表（按更新时间倒序）
	ListSessions(ctx context.Context, tenantUserId string, limit, offset int) ([]*assistant.AIAssistantSession, error)

	// UpdateSessionTitle 更新会话标题
	UpdateSessionTitle(ctx context.Context, sessionId, tenantUserId, title string) error

	// UpdateSessionAgent 更新会话绑定的Agent
	UpdateSessionAgent(ctx context.Context, sessionId, tenantUserId, agentId string) error

	// UpdateSessionUpdatedAt 更新会话的updated_at时间（每次消息后调用）
	UpdateSessionUpdatedAt(ctx context.Context, sessionId string) error
}

// AssistantMessageRepository AI助手消息仓储接口
type AssistantMessageRepository interface {
	// SaveMessage 保存单条消息
	SaveMessage(ctx context.Context, message *assistant.AIAssistantMessage) error

	// ListRecentMessages 获取会话最近N条消息（按时间正序，用于构建上下文）
	ListRecentMessages(ctx context.Context, sessionId string, limit int) ([]*assistant.AIAssistantMessage, error)

	ListMessages(ctx context.Context, sessionId string, limit, offset int) ([]*assistant.AIAssistantMessage, error)
	// - ListRecentMessages：取“最近 N 条”，按时间倒序取出，再反转为正序。适合用于构建 Prompt 的“最近上下文”，不适合分页。
	// - ListMessages：按时间正序，支持 limit/offset 的稳定分页。用于历史消息列表展示更合适。 <br/> 结论： 有必要新开函数 ，一个用于“上下文窗口”，一个用于“分页列表”。
	// CountSessionMessages 统计会话消息数量
	CountSessionMessages(ctx context.Context, sessionId string) (int64, error)
}
