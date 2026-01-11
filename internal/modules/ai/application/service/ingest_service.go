package service

import (
	"context"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/infrastructure/pipeline"
	"OmniLink/internal/modules/ai/infrastructure/reader"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type BackfillRequest struct {
	TenantUserID      string
	PageSize          int
	MaxSessions       int
	MaxPagesPerSession int
	Since             string
	Until             string
}

type BackfillResult struct {
	TenantUserID string `json:"tenant_user_id"`
	Sessions     int    `json:"sessions"`
	Pages        int    `json:"pages"`
	Messages     int    `json:"messages"`
	Chunks       int    `json:"chunks"`
	VectorsOK    int    `json:"vectors_ok"`
	VectorsSkip  int    `json:"vectors_skip"`
	VectorsFail  int    `json:"vectors_fail"`
	DurationMs   int64  `json:"duration_ms"`
}

type IngestService interface {
	Backfill(ctx context.Context, req BackfillRequest) (*BackfillResult, error)
}

type ingestService struct {
	reader   *reader.ChatSessionReader
	pipeline *pipeline.IngestPipeline
}

func NewIngestService(r *reader.ChatSessionReader, p *pipeline.IngestPipeline) IngestService {
	return &ingestService{reader: r, pipeline: p}
}

func (s *ingestService) Backfill(ctx context.Context, req BackfillRequest) (*BackfillResult, error) {
	start := time.Now()
	tenant := strings.TrimSpace(req.TenantUserID)
	if tenant == "" {
		return nil, xerr.New(xerr.BadRequest, "missing tenant_user_id")
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

	sessions, err := s.reader.ListAllSessions(ctx, tenant)
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

	out := &BackfillResult{TenantUserID: tenant}
	for i := 0; i < maxSessions; i++ {
		sess := sessions[i]
		sType := "chat_group"
		if sess.Type == reader.SessionTypePrivate {
			sType = "chat_private"
		}

		pages := 0
		for page := 1; ; page++ {
			if maxPages > 0 && pages >= maxPages {
				break
			}
			msgs, err := s.reader.ReadMessages(ctx, tenant, sess, page, pageSize, since)
			if err != nil {
				out.VectorsFail++
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

			pr, err := s.pipeline.Ingest(ctx, pipeline.IngestRequest{TenantUserID: tenant, SessionUUID: sess.SessionUUID, SessionType: int(sess.Type), SessionName: sess.Name, SourceType: sType, SourceKey: sess.TargetID, Messages: msgs})
			if err != nil {
				out.VectorsFail++
				continue
			}
			out.Chunks += pr.Chunks
			out.VectorsOK += pr.VectorsOK
			out.VectorsSkip += pr.VectorsSkip
			out.VectorsFail += pr.VectorsFail
		}

		out.Sessions++
		zlog.Info("ai backfill session done", zap.String("tenant_user_id", tenant), zap.String("session_uuid", sess.SessionUUID), zap.String("source_type", sType), zap.String("source_key", sess.TargetID), zap.Int("pages", pages))
	}

	out.DurationMs = time.Since(start).Milliseconds()
	zlog.Info("ai backfill done", zap.String("tenant_user_id", tenant), zap.Int("sessions", out.Sessions), zap.Int("pages", out.Pages), zap.Int("messages", out.Messages), zap.Int("chunks", out.Chunks), zap.Int("ok", out.VectorsOK), zap.Int("skip", out.VectorsSkip), zap.Int("fail", out.VectorsFail), zap.Int64("ms", out.DurationMs))
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
