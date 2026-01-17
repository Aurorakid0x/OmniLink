package repository

import (
	"context"

	"OmniLink/internal/modules/ai/domain/rag"
)

type RAGRepository interface {
	EnsureKnowledgeBase(ctx context.Context, kb *rag.AIKnowledgeBase) (int64, error)
	EnsureKnowledgeSource(ctx context.Context, source *rag.AIKnowledgeSource) (int64, error)

	GetKnowledgeSource(ctx context.Context, kbID int64, tenantUserID, sourceType, sourceKey string) (*rag.AIKnowledgeSource, error)
	ListVectorIDsBySourceID(ctx context.Context, sourceID int64) ([]string, error)
	DeleteChunksAndVectorRecordsBySourceID(ctx context.Context, sourceID int64) error
	UpdateKnowledgeSourceStatus(ctx context.Context, sourceID int64, status int8) error

	GetChunkByChunkKey(ctx context.Context, chunkKey string) (*rag.AIKnowledgeChunk, error)
	GetVectorRecordByVectorID(ctx context.Context, vectorID string) (*rag.AIVectorRecord, error)
	GetVectorRecordByChunkID(ctx context.Context, chunkID int64) (*rag.AIVectorRecord, error)

	CreateChunkAndVectorRecord(ctx context.Context, chunk *rag.AIKnowledgeChunk, record *rag.AIVectorRecord) error
	CreateVectorRecord(ctx context.Context, record *rag.AIVectorRecord) error
	UpdateVectorStatus(ctx context.Context, vectorID string, status int8, errorMsg string) error
}
