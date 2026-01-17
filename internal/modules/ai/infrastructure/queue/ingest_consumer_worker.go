package queue

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/mq"
	"OmniLink/internal/modules/ai/infrastructure/pipeline"
	"OmniLink/internal/modules/ai/infrastructure/reader"
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type IngestConsumerWorker struct {
	consumer mq.Consumer

	eventRepo repository.IngestEventRepository
	jobRepo   repository.BackfillJobRepository

	chatReader    *reader.ChatSessionReader
	selfReader    *reader.SelfProfileReader
	contactReader *reader.ContactProfileReader
	groupReader   *reader.GroupProfileReader

	pipeline *pipeline.IngestPipeline
}

func NewIngestConsumerWorker(consumer mq.Consumer, eventRepo repository.IngestEventRepository, jobRepo repository.BackfillJobRepository, chatReader *reader.ChatSessionReader, selfReader *reader.SelfProfileReader, contactReader *reader.ContactProfileReader, groupReader *reader.GroupProfileReader, p *pipeline.IngestPipeline) *IngestConsumerWorker {
	return &IngestConsumerWorker{
		consumer:      consumer,
		eventRepo:     eventRepo,
		jobRepo:       jobRepo,
		chatReader:    chatReader,
		selfReader:    selfReader,
		contactReader: contactReader,
		groupReader:   groupReader,
		pipeline:      p,
	}
}

func (w *IngestConsumerWorker) Run(ctx context.Context) error {
	if w == nil || w.consumer == nil {
		return errors.New("consumer is nil")
	}
	if w.eventRepo == nil {
		return errors.New("event repo is nil")
	}
	if w.pipeline == nil {
		return errors.New("pipeline is nil")
	}
	return w.consumer.Run(ctx, w)
}

func (w *IngestConsumerWorker) Handle(ctx context.Context, msg mq.Message) error {
	idStr := strings.TrimSpace(string(msg.Value))
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		zlog.Warn("ai ingest consumer invalid event_id", zap.String("topic", msg.Topic))
		return nil
	}

	ev, err := w.eventRepo.GetByID(ctx, id)
	if err != nil {
		zlog.Warn("ai ingest consumer get event failed", zap.Int64("event_id", id), zap.Error(err))
		return err
	}
	if ev == nil {
		return nil
	}
	if ev.Status == rag.IngestEventStatusSucceeded {
		return nil
	}

	now := time.Now()
	ok, err := w.eventRepo.TryMarkProcessing(ctx, ev.Id, now)
	if err != nil {
		zlog.Warn("ai ingest consumer mark processing failed", zap.Int64("event_id", ev.Id), zap.Error(err))
		return err
	}
	if !ok {
		return nil
	}

	procErr := w.processEvent(ctx, ev)
	if procErr != nil {
		_ = w.eventRepo.MarkFailed(ctx, ev.Id, scrubErrMsg(procErr.Error()))
		if w.jobRepo != nil && ev.BackfillJobId.Valid {
			_ = w.jobRepo.AddCounters(ctx, ev.BackfillJobId.Int64, 0, 0, 0, 1)
			_ = w.tryFinalizeJob(ctx, ev.BackfillJobId.Int64)
		}
		zlog.Warn("ai ingest consumer event failed",
			zap.Int64("event_id", ev.Id),
			zap.String("event_type", strings.TrimSpace(ev.EventType)),
			zap.String("tenant_user_id", strings.TrimSpace(ev.TenantUserId)),
			zap.String("source_type", strings.TrimSpace(ev.SourceType)),
			zap.String("source_key", strings.TrimSpace(ev.SourceKey)),
			zap.String("error", scrubErrMsg(procErr.Error())),
		)
		return nil
	}

	if err := w.eventRepo.MarkSucceeded(ctx, ev.Id); err != nil {
		zlog.Warn("ai ingest consumer mark succeeded failed", zap.Int64("event_id", ev.Id), zap.Error(err))
		return err
	}
	if w.jobRepo != nil && ev.BackfillJobId.Valid {
		_ = w.jobRepo.AddCounters(ctx, ev.BackfillJobId.Int64, 0, 0, 1, 0)
		_ = w.tryFinalizeJob(ctx, ev.BackfillJobId.Int64)
	}

	return nil
}

func (w *IngestConsumerWorker) processEvent(ctx context.Context, ev *rag.AIIngestEvent) error {
	if ev == nil {
		return errors.New("nil event")
	}

	switch strings.TrimSpace(ev.EventType) {
	case "self_profile":
		if w.selfReader == nil {
			return errors.New("self reader is nil")
		}
		doc, err := w.selfReader.ReadProfile(ctx, ev.TenantUserId)
		if err != nil {
			return err
		}
		doc = strings.TrimSpace(doc)
		if doc == "" {
			return nil
		}
		_, err = w.pipeline.Ingest(ctx, pipeline.IngestRequest{
			TenantUserID: ev.TenantUserId,
			SourceType:   "self_profile",
			SourceKey:    strings.TrimSpace(ev.SourceKey),
			Documents:    []string{doc},
		})
		return err

	case "contact_profile":
		if w.contactReader == nil {
			return errors.New("contact reader is nil")
		}
		var p struct {
			ContactID string `json:"contact_id"`
		}
		if err := json.Unmarshal([]byte(ev.PayloadJson), &p); err != nil {
			return err
		}
		cid := strings.TrimSpace(p.ContactID)
		if cid == "" {
			cid = strings.TrimSpace(ev.SourceKey)
		}
		if cid == "" {
			return errors.New("missing contact_id")
		}
		doc, err := w.contactReader.ReadContactProfile(ctx, ev.TenantUserId, cid)
		if err != nil {
			return err
		}
		doc = strings.TrimSpace(doc)
		if doc == "" {
			return nil
		}
		_, err = w.pipeline.Ingest(ctx, pipeline.IngestRequest{
			TenantUserID: ev.TenantUserId,
			SourceType:   "contact_profile",
			SourceKey:    cid,
			Documents:    []string{doc},
		})
		return err

	case "group_profile":
		if w.groupReader == nil {
			return errors.New("group reader is nil")
		}
		var p struct {
			GroupID string `json:"group_id"`
		}
		if err := json.Unmarshal([]byte(ev.PayloadJson), &p); err != nil {
			return err
		}
		gid := strings.TrimSpace(p.GroupID)
		if gid == "" {
			gid = strings.TrimSpace(ev.SourceKey)
		}
		if gid == "" {
			return errors.New("missing group_id")
		}

		doc, _, err := w.groupReader.ReadGroupProfile(ctx, ev.TenantUserId, gid)
		if err != nil {
			return err
		}
		doc = strings.TrimSpace(doc)

		if doc == "" {
			if err := w.pipeline.PurgeSource(ctx, ev.TenantUserId, "group_profile", gid, true); err != nil {
				return err
			}
			return nil
		}

		if err := w.pipeline.PurgeSource(ctx, ev.TenantUserId, "group_profile", gid, false); err != nil {
			return err
		}
		_, err = w.pipeline.Ingest(ctx, pipeline.IngestRequest{
			TenantUserID: ev.TenantUserId,
			SourceType:   "group_profile",
			SourceKey:    gid,
			Documents:    []string{doc},
		})
		return err

	case "chat_messages_page":
		if w.chatReader == nil {
			return errors.New("chat reader is nil")
		}

		var p struct {
			SessionUUID string `json:"session_uuid"`
			SessionType int    `json:"session_type"`
			SessionName string `json:"session_name"`
			TargetID    string `json:"target_id"`
			Page        int    `json:"page"`
			PageSize    int    `json:"page_size"`
			Since       string `json:"since"`
			Until       string `json:"until"`
		}
		if err := json.Unmarshal([]byte(ev.PayloadJson), &p); err != nil {
			return err
		}

		sessUUID := strings.TrimSpace(p.SessionUUID)
		if sessUUID == "" {
			return errors.New("missing session_uuid")
		}
		targetID := strings.TrimSpace(p.TargetID)
		if targetID == "" {
			targetID = strings.TrimSpace(ev.SourceKey)
		}
		if targetID == "" {
			return errors.New("missing target_id")
		}

		page := p.Page
		if page <= 0 {
			page = 1
		}
		pageSize := p.PageSize
		if pageSize <= 0 {
			pageSize = 200
		}
		if pageSize > 200 {
			pageSize = 200
		}

		var since *time.Time
		var until *time.Time
		if strings.TrimSpace(p.Since) != "" {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.Since)); err == nil {
				since = &t
			}
		}
		if strings.TrimSpace(p.Until) != "" {
			if t, err := time.Parse(time.RFC3339, strings.TrimSpace(p.Until)); err == nil {
				until = &t
			}
		}

		sess := reader.ChatSessionItem{
			SessionUUID: sessUUID,
			TargetID:    targetID,
			Type:        reader.SessionType(p.SessionType),
			Name:        strings.TrimSpace(p.SessionName),
		}

		msgs := make([]chatEntity.Message, 0, pageSize)

		if !ev.BackfillJobId.Valid {
			maxPages := 50
			for cur := 1; cur <= maxPages; cur++ {
				raw, err := w.chatReader.ReadMessagesPage(ctx, ev.TenantUserId, sess, cur, pageSize)
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					break
				}

				pageMsgs, err := w.chatReader.ReadMessages(ctx, ev.TenantUserId, sess, cur, pageSize, since)
				if err != nil {
					return err
				}
				if until != nil {
					filtered := pageMsgs[:0]
					for _, m := range pageMsgs {
						if !m.CreatedAt.After(*until) {
							filtered = append(filtered, m)
						}
					}
					pageMsgs = filtered
				}
				if len(pageMsgs) > 0 {
					msgs = append(msgs, pageMsgs...)
				}

				if since != nil {
					oldest := raw[len(raw)-1].CreatedAt
					if !oldest.After(*since) {
						break
					}
				}
			}
		} else {
			pageMsgs, err := w.chatReader.ReadMessages(ctx, ev.TenantUserId, sess, page, pageSize, since)
			if err != nil {
				return err
			}
			if until != nil {
				filtered := pageMsgs[:0]
				for _, m := range pageMsgs {
					if !m.CreatedAt.After(*until) {
						filtered = append(filtered, m)
					}
				}
				pageMsgs = filtered
			}
			msgs = append(msgs, pageMsgs...)
		}

		if len(msgs) == 0 {
			return nil
		}

		_, err := w.pipeline.Ingest(ctx, pipeline.IngestRequest{
			TenantUserID: ev.TenantUserId,
			SessionUUID:  sessUUID,
			SessionType:  p.SessionType,
			SessionName:  strings.TrimSpace(p.SessionName),
			SourceType:   strings.TrimSpace(ev.SourceType),
			SourceKey:    targetID,
			Messages:     msgs,
		})
		return err

	default:
		return errors.New("unknown event_type")
	}
}

func (w *IngestConsumerWorker) tryFinalizeJob(ctx context.Context, jobID int64) error {
	if w == nil || w.jobRepo == nil || jobID <= 0 {
		return nil
	}
	j, err := w.jobRepo.GetByID(ctx, jobID)
	if err != nil || j == nil {
		return err
	}
	if j.TotalEvents <= 0 {
		return nil
	}
	done := j.SucceededEvents + j.FailedEvents
	if done < j.TotalEvents {
		return nil
	}
	status := rag.BackfillJobStatusSucceeded
	if j.FailedEvents > 0 {
		status = rag.BackfillJobStatusFailed
	}
	return w.jobRepo.UpdateStatus(ctx, jobID, status)
}

func scrubErrMsg(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	low := strings.ToLower(s)
	if strings.Contains(low, "api_key") || strings.Contains(low, "apikey") || strings.Contains(low, "secret") || strings.Contains(s, "sk-") {
		return "redacted"
	}
	if len(s) > 255 {
		return s[:255]
	}
	return s
}
