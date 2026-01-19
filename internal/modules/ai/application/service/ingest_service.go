package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	aiRequest "OmniLink/internal/modules/ai/application/dto/request"
	aiRespond "OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/rag"
	aiRepo "OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/reader"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type IngestService interface {
	Backfill(ctx context.Context, req aiRequest.BackfillRequest) (*aiRespond.BackfillResult, error)
}

type ingestService struct {
	chatReader    *reader.ChatSessionReader
	selfReader    *reader.SelfProfileReader
	contactReader *reader.ContactProfileReader
	groupReader   *reader.GroupProfileReader
	jobRepo       aiRepo.BackfillJobRepository
	eventRepo     aiRepo.IngestEventRepository
}

func NewIngestService(chat *reader.ChatSessionReader, self *reader.SelfProfileReader, contact *reader.ContactProfileReader, group *reader.GroupProfileReader, jobRepo aiRepo.BackfillJobRepository, eventRepo aiRepo.IngestEventRepository) IngestService {
	return &ingestService{chatReader: chat, selfReader: self, contactReader: contact, groupReader: group, jobRepo: jobRepo, eventRepo: eventRepo}
}

func (s *ingestService) Backfill(ctx context.Context, req aiRequest.BackfillRequest) (*aiRespond.BackfillResult, error) {
	start := time.Now()
	tenant := strings.TrimSpace(req.TenantUserID)
	if tenant == "" {
		return nil, xerr.New(xerr.BadRequest, "missing tenant_user_id")
	}
	if s == nil || s.chatReader == nil || s.eventRepo == nil || s.jobRepo == nil {
		return nil, xerr.ErrServerError
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 200
	}
	if pageSize > 200 {
		pageSize = 200
	}

	since, err := parseTime(req.Since)
	if err != nil {
		return nil, xerr.New(xerr.BadRequest, "invalid since")
	}
	until, err := parseTime(req.Until)
	if err != nil {
		return nil, xerr.New(xerr.BadRequest, "invalid until")
	}

	now := time.Now()
	job := &rag.AIBackfillJob{
		TenantUserId:       tenant,
		Status:             rag.BackfillJobStatusRunning,
		PageSize:           pageSize,
		MaxSessions:        req.MaxSessions,
		MaxPagesPerSession: req.MaxPagesPerSession,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if since != nil {
		job.Since = sql.NullTime{Time: *since, Valid: true}
	}
	if until != nil {
		job.Until = sql.NullTime{Time: *until, Valid: true}
	}
	if err := s.jobRepo.Create(ctx, job); err != nil {
		return nil, err
	}

	out := &aiRespond.BackfillResult{TenantUserID: tenant, JobID: job.Id}

	sinceStr := ""
	untilStr := ""
	if since != nil {
		sinceStr = since.Format(time.RFC3339)
	}
	if until != nil {
		untilStr = until.Format(time.RFC3339)
	}

	events := make([]rag.AIIngestEvent, 0, 128)
	add := func(eventType, sourceType, sourceKey, dedupExtra string, payload any) {
		b, mErr := json.Marshal(payload)
		if mErr != nil {
			return
		}
		dk := fmt.Sprintf("bf_%d_%s_%s_%s_%s", job.Id, eventType, sourceType, sourceKey, strings.TrimSpace(dedupExtra))
		ev := rag.AIIngestEvent{
			EventType:     eventType,
			TenantUserId:  tenant,
			BackfillJobId: sql.NullInt64{Int64: job.Id, Valid: true},
			SourceType:    sourceType,
			SourceKey:     sourceKey,
			PayloadJson:   string(b),
			DedupKey:      dk,
			PublishStatus: rag.IngestPublishStatusPending,
			Status:        rag.IngestEventStatusPending,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		events = append(events, ev)
	}

	add("self_profile", "self_profile", tenant, "0", map[string]any{"tenant_user_id": tenant})

	if s.contactReader != nil {
		items, err := s.contactReader.ListContactProfiles(ctx, tenant)
		if err != nil {
			zlog.Warn("ai backfill contact list failed", zap.String("tenant_user_id", tenant), zap.Error(err))
		} else {
			for _, it := range items {
				cid := strings.TrimSpace(it.ContactID)
				if cid == "" {
					continue
				}
				add("contact_profile", "contact_profile", cid, "0", map[string]any{"contact_id": cid})
			}
		}
	}

	if s.groupReader != nil {
		items, err := s.groupReader.ListGroupProfiles(ctx, tenant)
		if err != nil {
			zlog.Warn("ai backfill group list failed", zap.String("tenant_user_id", tenant), zap.Error(err))
		} else {
			for _, it := range items {
				gid := strings.TrimSpace(it.GroupID)
				if gid == "" {
					continue
				}
				add("group_profile", "group_profile", gid, "0", map[string]any{"group_id": gid})
			}
		}
	}

	sessions, err := s.chatReader.ListAllSessions(ctx, tenant)
	if err != nil {
		return nil, err
	}
	maxSessions := req.MaxSessions
	if maxSessions <= 0 || maxSessions > len(sessions) {
		maxSessions = len(sessions)
	}
	maxPages := req.MaxPagesPerSession
	if maxPages < 0 {
		maxPages = 0
	}

	for i := 0; i < maxSessions; i++ {
		sess := sessions[i]
		out.Sessions++

		sType := "chat_group"
		if sess.Type == reader.SessionTypePrivate {
			sType = "chat_private"
		}

		pages := 0
		for page := 1; ; page++ {
			if maxPages > 0 && pages >= maxPages {
				break
			}
			msgs, err := s.chatReader.ReadMessages(ctx, tenant, sess, page, pageSize, since)
			if err != nil {
				break
			}
			if until != nil {
				filtered := msgs[:0]
				for _, m := range msgs {
					if !m.CreatedAt.After(*until) {
						filtered = append(filtered, m)
					}
				}
				msgs = filtered
			}
			if len(msgs) == 0 {
				break
			}

			pages++
			out.Pages++
			out.Messages += len(msgs)

			add("chat_messages_page", sType, sess.TargetID, fmt.Sprintf("%d", page), map[string]any{
				"session_uuid": sess.SessionUUID,
				"session_type": int(sess.Type),
				"session_name": sess.Name,
				"target_id":    sess.TargetID,
				"page":         page,
				"page_size":    pageSize,
				"since":        sinceStr,
				"until":        untilStr,
			})
		}

		zlog.Info("ai backfill session scheduled", zap.String("tenant_user_id", tenant), zap.String("session_uuid", sess.SessionUUID), zap.String("source_type", sType), zap.String("source_key", sess.TargetID), zap.Int("pages", pages))
	}

	if err := s.eventRepo.CreateBatch(ctx, events); err != nil {
		return nil, err
	}
	_ = s.jobRepo.AddCounters(ctx, job.Id, len(events), 0, 0, 0)

	out.TotalEvents = len(events)
	out.DurationMs = time.Since(start).Milliseconds()
	zlog.Info("ai backfill scheduled", zap.String("tenant_user_id", tenant), zap.Int64("job_id", job.Id), zap.Int("events", out.TotalEvents), zap.Int("sessions", out.Sessions), zap.Int("pages", out.Pages), zap.Int("messages", out.Messages), zap.Int64("ms", out.DurationMs))
	return out, nil
}

func parseTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, l := range layouts {
		t, err := time.ParseInLocation(l, s, time.Local)
		if err == nil {
			return &t, nil
		}
	}
	return nil, xerr.New(xerr.BadRequest, "invalid time")
}
