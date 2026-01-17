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
}
