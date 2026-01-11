package repository

import (
	"context"

	"OmniLink/internal/modules/ai/domain/rag"
)

// RAGRepository 负责 RAG 元数据（MySQL）的持久化
type RAGRepository interface {
	EnsureKnowledgeBase(ctx context.Context, kb *rag.AIKnowledgeBase) error
	EnsureKnowledgeSource(ctx context.Context, source *rag.AIKnowledgeSource) error
	CreateChunkAndVectorRecord(ctx context.Context, chunk *rag.AIKnowledgeChunk, record *rag.AIVectorRecord) error
	UpdateVectorStatus(ctx context.Context, vectorID string, status int8, errorMsg string) error
}
