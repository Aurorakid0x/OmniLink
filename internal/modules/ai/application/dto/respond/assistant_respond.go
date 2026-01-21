package respond

import (
	"time"
)

// AssistantChatRespond AI助手聊天响应（非流式）
type AssistantChatRespond struct {
	SessionID string          `json:"session_id"` // 会话ID
	Answer    string          `json:"answer"`     // AI回答
	Citations []CitationEntry `json:"citations"`  // 引用列表
	QueryID   string          `json:"query_id"`   // 本次查询ID
	Timing    TimingInfo      `json:"timing"`     // 耗时统计
}

// AssistantStreamDoneEvent 流式输出完成事件（SSE final event）
type AssistantStreamDoneEvent struct {
	SessionID string          `json:"session_id"` // 会话ID
	Answer    string          `json:"answer"`     // 完整回答
	Citations []CitationEntry `json:"citations"`  // 引用列表
	QueryID   string          `json:"query_id"`   // 本次查询ID
	Timing    TimingInfo      `json:"timing"`     // 耗时统计
}

// CitationEntry 单条引用信息
type CitationEntry struct {
	ChunkID    string  `json:"chunk_id"`    // Chunk ID
	SourceType string  `json:"source_type"` // 来源类型（chat_private/chat_group/...）
	SourceKey  string  `json:"source_key"`  // 来源键（session_uuid/group_id）
	Score      float32 `json:"score"`       // 相似度分数
	Content    string  `json:"content"`     // 内容摘要
}

// TimingInfo 耗时统计信息
type TimingInfo struct {
	EmbeddingMs   int64 `json:"embedding_ms"`   // 向量化耗时（毫秒）
	SearchMs      int64 `json:"search_ms"`      // 检索耗时（毫秒）
	PostProcessMs int64 `json:"postprocess_ms"` // 后处理耗时（毫秒）
	LLMMs         int64 `json:"llm_ms"`         // LLM调用耗时（毫秒）
	TotalMs       int64 `json:"total_ms"`       // 总耗时（毫秒）
}

// AssistantSessionItem 会话列表项
type AssistantSessionItem struct {
	SessionID   string    `json:"session_id"`   // 会话ID
	Title       string    `json:"title"`        // 会话标题
	AgentID     string    `json:"agent_id"`     // 绑定的Agent ID
	UpdatedAt   time.Time `json:"updated_at"`   // 最后更新时间
	LastMessage string    `json:"last_message"` // 最新消息内容
	Summary     string    `json:"summary"`      // 列表摘要
}

// AssistantSessionListRespond 会话列表响应
type AssistantSessionListRespond struct {
	Sessions []*AssistantSessionItem `json:"sessions"` // 会话列表
	Total    int                     `json:"total"`    // 总数（当前仅返回查询到的数量）
}

// AssistantAgentItem Agent列表项
type AssistantAgentItem struct {
	AgentID     string `json:"agent_id"`    // Agent ID
	Name        string `json:"name"`        // Agent名称
	Description string `json:"description"` // Agent描述
	Status      int8   `json:"status"`      // 状态：1=enabled, 0=disabled
	OwnerType   string `json:"owner_type"`  // 归属类型：user/system
}

// AssistantAgentListRespond Agent列表响应
type AssistantAgentListRespond struct {
	Agents []*AssistantAgentItem `json:"agents"` // Agent列表
	Total  int                   `json:"total"`  // 总数
}
