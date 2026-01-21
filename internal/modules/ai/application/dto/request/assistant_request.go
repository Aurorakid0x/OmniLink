package request

// AssistantChatRequest AI助手聊天请求（非流式）
type AssistantChatRequest struct {
	SessionID  string   `json:"session_id"`  // 会话ID（可空，不传则创建新会话）
	Question   string   `json:"question"`    // 用户问题（必填）
	TopK       int      `json:"top_k"`       // 召回Top-K个chunks（默认5）
	Scope      string   `json:"scope"`       // 检索范围：global/chat_private/chat_group
	SourceKeys []string `json:"source_keys"` // 限制检索的source_key列表（可选）
	AgentID    string   `json:"agent_id"`    // 绑定的Agent ID（可选）
}

// AssistantSessionListRequest 获取会话列表请求
type AssistantSessionListRequest struct {
	Limit  int `json:"limit"`  // 每页数量（默认20）
	Offset int `json:"offset"` // 偏移量（默认0）
}

// AssistantAgentListRequest 获取Agent列表请求
type AssistantAgentListRequest struct {
	Limit  int `json:"limit"`  // 每页数量（默认20）
	Offset int `json:"offset"` // 偏移量（默认0）
}
