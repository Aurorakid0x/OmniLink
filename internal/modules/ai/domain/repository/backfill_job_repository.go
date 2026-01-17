package repository

import (
	"context"

	"OmniLink/internal/modules/ai/domain/rag"
)

type BackfillJobRepository interface {
	Create(ctx context.Context, job *rag.AIBackfillJob) error
	GetByID(ctx context.Context, id int64) (*rag.AIBackfillJob, error)
	UpdateStatus(ctx context.Context, id int64, status int8) error
	AddCounters(ctx context.Context, id int64, totalDelta, publishedDelta, succeededDelta, failedDelta int) error
}
