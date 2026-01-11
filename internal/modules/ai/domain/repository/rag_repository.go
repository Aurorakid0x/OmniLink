package repository

import (
	"context"

	"OmniLink/internal/modules/ai/domain/rag"
)

// RAGRepository 负责 RAG 元数据（MySQL）的持久化
type RAGRepository interface {
	EnsureKnowledgeBase(ctx context.Context, kb *rag.AIKnowledgeBase) (int64, error)
	EnsureKnowledgeSource(ctx context.Context, source *rag.AIKnowledgeSource) (int64, error)
	CreateChunkAndVectorRecord(ctx context.Context, chunk *rag.AIKnowledgeChunk, record *rag.AIVectorRecord) error
	UpdateVectorStatus(ctx context.Context, vectorID string, status int8, errorMsg string) error
}
