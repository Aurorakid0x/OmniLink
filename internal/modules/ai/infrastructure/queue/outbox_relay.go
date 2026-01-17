package queue

import (
	"context"
	"errors"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/mq"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type OutboxRelay struct {
	repo         repository.IngestEventRepository
	pub          mq.Publisher
	defaultTopic string
	batchSize    int
	pollInterval time.Duration
}

func NewOutboxRelay(repo repository.IngestEventRepository, pub mq.Publisher, defaultTopic string, batchSize int, pollInterval time.Duration) *OutboxRelay {
	if batchSize <= 0 {
		batchSize = 200
	}
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	return &OutboxRelay{
		repo:         repo,
		pub:          pub,
		defaultTopic: strings.TrimSpace(defaultTopic),
		batchSize:    batchSize,
		pollInterval: pollInterval,
	}
}

func (r *OutboxRelay) Run(ctx context.Context) error {
	if r.repo == nil {
		return errors.New("ingest event repo is nil")
	}
	if r.pub == nil {
		return errors.New("publisher is nil")
	}

	backoff := r.pollInterval
	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		n, err := r.RunOnce(ctx)
		if err != nil {
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		backoff = r.pollInterval

		if n == 0 {
			time.Sleep(r.pollInterval)
		}
	}
}

func (r *OutboxRelay) RunOnce(ctx context.Context) (int, error) {
	now := time.Now()
	events, err := r.repo.ClaimForPublish(ctx, now, r.batchSize)
	if err != nil {
		zlog.Warn("ai outbox relay claim failed", zap.Error(err))
		return 0, err
	}
	if len(events) == 0 {
		return 0, nil
	}

	published := 0
	for i := range events {
		ev := events[i]
		topic := r.defaultTopic
		if topic == "" {
			topic = strings.TrimSpace(ev.KafkaTopic)
		}
		if topic == "" {
			_ = r.repo.MarkPublishFailed(ctx, ev.Id, now.Add(5*time.Minute), "kafka topic is empty")
			continue
		}

		key := []byte(ev.DedupKey)
		if len(key) == 0 {
			key = []byte(strconvInt64(ev.Id))
		}

		res, pubErr := r.pub.Publish(ctx, mq.Message{
			Topic: topic,
			Key:   key,
			Value: []byte(ev.PayloadJson),
			Headers: map[string]string{
				"event_type":     ev.EventType,
				"tenant_user_id": ev.TenantUserId,
				"source_type":    ev.SourceType,
				"source_key":     ev.SourceKey,
				"trace_id":       ev.TraceId,
				"dedup_key":      ev.DedupKey,
			},
		})
		if pubErr != nil {
			next := computeNextRetry(now, ev.RetryCount)
			_ = r.repo.MarkPublishFailed(ctx, ev.Id, next, pubErr.Error())
			continue
		}

		if err := r.repo.MarkPublished(ctx, ev.Id, topic, int(res.Partition), res.Offset, time.Now()); err != nil {
			zlog.Warn("ai outbox relay mark published failed", zap.Int64("id", ev.Id), zap.Error(err))
			continue
		}
		published++
	}

	return published, nil
}

func computeNextRetry(now time.Time, retryCount int) time.Time {
	if retryCount < 0 {
		retryCount = 0
	}
	d := 500 * time.Millisecond
	for i := 0; i < retryCount && d < 5*time.Minute; i++ {
		d = d * 2
	}
	if d > 5*time.Minute {
		d = 5 * time.Minute
	}
	return now.Add(d)
}

func strconvInt64(v int64) string {
	if v == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	n := v
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
