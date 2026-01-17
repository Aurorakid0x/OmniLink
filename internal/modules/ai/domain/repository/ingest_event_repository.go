package repository

import (
	"context"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
)

type IngestEventRepository interface {
	Create(ctx context.Context, ev *rag.AIIngestEvent) error
	ClaimForPublish(ctx context.Context, now time.Time, limit int) ([]rag.AIIngestEvent, error)
	MarkPublished(ctx context.Context, id int64, topic string, partition int, offset int64, publishedAt time.Time) error
	MarkPublishFailed(ctx context.Context, id int64, nextRetryAt time.Time, errMsg string) error
	CreateBatch(ctx context.Context, events []rag.AIIngestEvent) error
	GetByID(ctx context.Context, id int64) (*rag.AIIngestEvent, error)
	TryMarkProcessing(ctx context.Context, id int64, now time.Time) (bool, error)
	MarkSucceeded(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}
