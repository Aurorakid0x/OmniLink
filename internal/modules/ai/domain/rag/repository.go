package rag

import "context"

// RAGRepository 负责 RAG 元数据（MySQL）的持久化
type RAGRepository interface {
	// EnsureKnowledgeBase 确保知识库存在：不存在则创建，存在则按需要更新（应具备幂等性）
	EnsureKnowledgeBase(ctx context.Context, kb *AIKnowledgeBase) error

	// EnsureKnowledgeSource 确保数据源存在：不存在则创建，存在则按需要更新（应具备幂等性）
	EnsureKnowledgeSource(ctx context.Context, source *AIKnowledgeSource) error

	// CreateChunkAndVectorRecord 原子写入：同时落库 chunk 与对应的向量记录（建议使用事务）
	CreateChunkAndVectorRecord(ctx context.Context, chunk *AIKnowledgeChunk, record *AIVectorRecord) error

	// UpdateVectorStatus 更新向量化状态与错误信息（用于失败重试/排障）
	UpdateVectorStatus(ctx context.Context, vectorID string, status int8, errorMsg string) error
}

