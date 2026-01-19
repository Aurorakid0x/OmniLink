package respond

// RAGChunkHit 单个召回的 chunk 结果
type RAGChunkHit struct {
	ChunkID    int64   `json:"chunk_id"`    // Chunk ID
	SourceType string  `json:"source_type"` // 数据源类型（chat_private, chat_group 等）
	SourceKey  string  `json:"source_key"`  // 数据源唯一键（如 session_uuid）
	Score      float32 `json:"score"`       // 相似度得分（越高越相关）
	Content    string  `json:"content"`     // Chunk 文本内容
	Metadata   string  `json:"metadata"`    // 元数据 JSON（包含 session_uuid, message_uuid 等）
}

// RAGQueryRespond RAG 召回查询响应
type RAGQueryRespond struct {
	QueryID       string        `json:"query_id"`        // 本次查询唯一 ID（便于追踪回放）
	Question      string        `json:"question"`        // 原始用户问题
	Chunks        []RAGChunkHit `json:"chunks"`          // 召回的 chunks 列表（按 score 降序）
	TotalHits     int           `json:"total_hits"`      // 向量库实际返回的结果数（过滤前）
	ReturnedCount int           `json:"returned_count"`  // 最终返回的 chunk 数量（过滤后）
	DurationMs    int64         `json:"duration_ms"`     // 召回耗时（毫秒）
	EmbeddingMs   int64         `json:"embedding_ms"`    // 向量化耗时（毫秒）
	SearchMs      int64         `json:"search_ms"`       // 向量检索耗时（毫秒）
	PostProcessMs int64         `json:"post_process_ms"` // 后处理耗时（毫秒）
	IsEmpty       bool          `json:"is_empty"`        // 是否未命中任何结果（兜底标识）
	Message       string        `json:"message"`         // 提示信息（如"未命中知识库，建议回填"）
}

type BackfillResult struct {
	TenantUserID string `json:"tenant_user_id"`
	JobID        int64  `json:"job_id"`
	TotalEvents  int    `json:"total_events"`
	Sessions     int    `json:"sessions"`
	Pages        int    `json:"pages"`
	Messages     int    `json:"messages"`
	Chunks       int    `json:"chunks"`
	VectorsOK    int    `json:"vectors_ok"`
	VectorsSkip  int    `json:"vectors_skip"`
	VectorsFail  int    `json:"vectors_fail"`
	DurationMs   int64  `json:"duration_ms"`
}
