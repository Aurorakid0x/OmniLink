package request

import "time"

// RAGQueryRequest RAG 召回查询请求
type RAGQueryRequest struct {
	Question string `json:"question" binding:"required"` // 用户问题（必填）
	TopK     int    `json:"top_k"`                       // 返回 Top-K 个 chunks（默认 5，范围 1-50）
	KBType   string `json:"kb_type"`                     // 知识库类型（默认 global）

	// 可选过滤条件
	SourceTypes []string `json:"source_types,omitempty"` // 过滤数据源类型（如 chat_private, chat_group）
	SourceKeys  []string `json:"source_keys,omitempty"`  // 过滤数据源键（如特定的 session_uuid）

	// 召回质量控制参数
	ScoreThreshold  float32 `json:"score_threshold,omitempty"`   // 相似度得分阈值（0.0-1.0，低于此值的结果会被过滤）
	MaxChunks       int     `json:"max_chunks,omitempty"`        // 最大返回 chunks 数量（硬限制，优先级高于 TopK）
	MaxContentChars int     `json:"max_content_chars,omitempty"` // 最大返回内容字符数（避免超过 LLM 窗口）

	// 去重与合并策略
	DedupBySameSource bool `json:"dedup_by_same_source,omitempty"` // 是否对同一个 source 去重（只保留得分最高的）
}

type ChatMessagesPageRequest struct {
	TenantUserID string

	SessionUUID string
	SessionType int
	SessionName string

	TargetID string

	Page     int
	PageSize int

	Since *time.Time
	Until *time.Time

	SourceType string
	SourceKey  string

	DedupExtra string
}
type BackfillRequest struct {
	TenantUserID       string `json:"tenant_user_id"`
	PageSize           int    `json:"page_size"`
	MaxSessions        int    `json:"max_sessions"`
	MaxPagesPerSession int    `json:"max_pages_per_session"`
	Since              string `json:"since"`
	Until              string `json:"until"`
}
