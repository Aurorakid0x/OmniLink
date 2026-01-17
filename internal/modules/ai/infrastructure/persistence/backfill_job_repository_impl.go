package persistence

import (
	"context"
	"errors"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"

	"gorm.io/gorm"
)

type backfillJobRepositoryImpl struct {
	db *gorm.DB
}

func NewBackfillJobRepository(db *gorm.DB) repository.BackfillJobRepository {
	return &backfillJobRepositoryImpl{db: db}
}

func (r *backfillJobRepositoryImpl) Create(ctx context.Context, job *rag.AIBackfillJob) error {
	if job == nil {
		return nil
	}
	now := time.Now()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	job.UpdatedAt = now
	return r.db.WithContext(ctx).Create(job).Error
}

func (r *backfillJobRepositoryImpl) GetByID(ctx context.Context, id int64) (*rag.AIBackfillJob, error) {
	var j rag.AIBackfillJob
	err := r.db.WithContext(ctx).Where("id = ?", id).Take(&j).Error
	if err == nil {
		return &j, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

func (r *backfillJobRepositoryImpl) UpdateStatus(ctx context.Context, id int64, status int8) error {
	return r.db.WithContext(ctx).Model(&rag.AIBackfillJob{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": status, "updated_at": time.Now()}).Error
}

func (r *backfillJobRepositoryImpl) AddCounters(ctx context.Context, id int64, totalDelta, publishedDelta, succeededDelta, failedDelta int) error {
	updates := map[string]any{"updated_at": time.Now()}
	if totalDelta != 0 {
		updates["total_events"] = gorm.Expr("total_events + ?", totalDelta)
	}
	if publishedDelta != 0 {
		updates["published_events"] = gorm.Expr("published_events + ?", publishedDelta)
	}
	if succeededDelta != 0 {
		updates["succeeded_events"] = gorm.Expr("succeeded_events + ?", succeededDelta)
	}
	if failedDelta != 0 {
		updates["failed_events"] = gorm.Expr("failed_events + ?", failedDelta)
	}

	return r.db.WithContext(ctx).Model(&rag.AIBackfillJob{}).
		Where("id = ?", id).
		Updates(updates).Error
}
