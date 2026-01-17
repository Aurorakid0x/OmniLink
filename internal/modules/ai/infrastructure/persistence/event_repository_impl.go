package persistence

import (
	"context"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ingestEventRepositoryImpl struct {
	db *gorm.DB
}

func NewIngestEventRepository(db *gorm.DB) repository.IngestEventRepository {
	return &ingestEventRepositoryImpl{db: db}
}

func (r *ingestEventRepositoryImpl) Create(ctx context.Context, ev *rag.AIIngestEvent) error {
	if ev == nil {
		return nil
	}
	return r.db.WithContext(ctx).Create(ev).Error
}

func (r *ingestEventRepositoryImpl) ClaimForPublish(ctx context.Context, now time.Time, limit int) ([]rag.AIIngestEvent, error) {
	if limit <= 0 {
		limit = 100
	}

	var out []rag.AIIngestEvent
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var events []rag.AIIngestEvent
		q := tx.Model(&rag.AIIngestEvent{}).
			Where("(publish_status = ? OR publish_status = ?)", rag.IngestPublishStatusPending, rag.IngestPublishStatusFailed).
			Where("(next_retry_at IS NULL OR next_retry_at <= ?)", now).
			Order("id ASC").
			Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if err := q.Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			out = []rag.AIIngestEvent{}
			return nil
		}

		ids := make([]int64, 0, len(events))
		for i := range events {
			ids = append(ids, events[i].Id)
		}
		if err := tx.Model(&rag.AIIngestEvent{}).
			Where("id IN ?", ids).
			Updates(map[string]any{"publish_status": rag.IngestPublishStatusPublishing, "updated_at": now}).Error; err != nil {
			return err
		}

		out = events
		return nil
	})
	return out, err
}

func (r *ingestEventRepositoryImpl) MarkPublished(ctx context.Context, id int64, topic string, partition int, offset int64, publishedAt time.Time) error {
	topic = strings.TrimSpace(topic)
	updates := map[string]any{
		"publish_status":  rag.IngestPublishStatusPublished,
		"kafka_topic":     topic,
		"kafka_partition": partition,
		"kafka_offset":    offset,
		"published_at":    publishedAt,
		"last_error":      "",
		"updated_at":      time.Now(),
	}
	return r.db.WithContext(ctx).Model(&rag.AIIngestEvent{}).Where("id = ?", id).Updates(updates).Error
}

func (r *ingestEventRepositoryImpl) MarkPublishFailed(ctx context.Context, id int64, nextRetryAt time.Time, errMsg string) error {
	errMsg = strings.TrimSpace(errMsg)
	if len(errMsg) > 255 {
		errMsg = errMsg[:255]
	}
	updates := map[string]any{
		"publish_status": rag.IngestPublishStatusFailed,
		"retry_count":    gorm.Expr("retry_count + 1"),
		"next_retry_at":  nextRetryAt,
		"last_error":     errMsg,
		"updated_at":     time.Now(),
	}
	return r.db.WithContext(ctx).Model(&rag.AIIngestEvent{}).Where("id = ?", id).Updates(updates).Error
}