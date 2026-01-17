package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/rag"
	aiRepo "OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/xerr"
)

type ChatMessagesPageRequest struct {
	TenantUserID string

	SessionUUID string
	SessionType int
	SessionName string

	TargetID string

	Page     int
	PageSize int

	Since *time.Time
	Until *time.Time

	SourceType string
	SourceKey  string

	DedupExtra string
}

type AsyncIngestService interface {
	EnqueueSelfProfile(ctx context.Context, tenantUserID string) error
	EnqueueContactProfile(ctx context.Context, tenantUserID, contactID string) error
	EnqueueGroupProfile(ctx context.Context, tenantUserID, groupID string) error
	EnqueueChatMessagesPage(ctx context.Context, req ChatMessagesPageRequest) error
}

type asyncIngestService struct {
	eventRepo aiRepo.IngestEventRepository
}

func NewAsyncIngestService(eventRepo aiRepo.IngestEventRepository) AsyncIngestService {
	return &asyncIngestService{eventRepo: eventRepo}
}

func (s *asyncIngestService) EnqueueSelfProfile(ctx context.Context, tenantUserID string) error {
	tenant := strings.TrimSpace(tenantUserID)
	if tenant == "" {
		return xerr.New(xerr.BadRequest, "missing tenant_user_id")
	}
	payload := map[string]any{"tenant_user_id": tenant}
	return s.enqueue(ctx, "self_profile", tenant, "self_profile", tenant, payload, dedupByMinute())
}

func (s *asyncIngestService) EnqueueContactProfile(ctx context.Context, tenantUserID, contactID string) error {
	tenant := strings.TrimSpace(tenantUserID)
	cid := strings.TrimSpace(contactID)
	if tenant == "" || cid == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	payload := map[string]any{"contact_id": cid}
	return s.enqueue(ctx, "contact_profile", tenant, "contact_profile", cid, payload, dedupByMinute())
}

func (s *asyncIngestService) EnqueueGroupProfile(ctx context.Context, tenantUserID, groupID string) error {
	tenant := strings.TrimSpace(tenantUserID)
	gid := strings.TrimSpace(groupID)
	if tenant == "" || gid == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	payload := map[string]any{"group_id": gid}
	return s.enqueue(ctx, "group_profile", tenant, "group_profile", gid, payload, dedupByMinute())
}

func (s *asyncIngestService) EnqueueChatMessagesPage(ctx context.Context, req ChatMessagesPageRequest) error {
	if s == nil || s.eventRepo == nil {
		return nil
	}

	tenant := strings.TrimSpace(req.TenantUserID)
	if tenant == "" {
		return xerr.New(xerr.BadRequest, "missing tenant_user_id")
	}

	sessUUID := strings.TrimSpace(req.SessionUUID)
	if sessUUID == "" {
		return xerr.New(xerr.BadRequest, "missing session_uuid")
	}

	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		targetID = strings.TrimSpace(req.SourceKey)
	}
	if targetID == "" {
		return xerr.New(xerr.BadRequest, "missing target_id")
	}

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	sinceStr := ""
	if req.Since != nil && !req.Since.IsZero() {
		sinceStr = req.Since.Format(time.RFC3339)
	}
	untilStr := ""
	if req.Until != nil && !req.Until.IsZero() {
		untilStr = req.Until.Format(time.RFC3339)
	}

	sourceType := strings.TrimSpace(req.SourceType)
	sourceKey := strings.TrimSpace(req.SourceKey)
	if sourceType == "" {
		return xerr.New(xerr.BadRequest, "missing source_type")
	}
	if sourceKey == "" {
		sourceKey = targetID
	}

	payload := map[string]any{
		"session_uuid": sessUUID,
		"session_type": req.SessionType,
		"session_name": strings.TrimSpace(req.SessionName),
		"target_id":    targetID,
		"page":         page,
		"page_size":    pageSize,
		"since":        sinceStr,
		"until":        untilStr,
	}

	dedupExtra := strings.TrimSpace(req.DedupExtra)
	if dedupExtra == "" {
		dedupExtra = strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	return s.enqueue(ctx, "chat_messages_page", tenant, sourceType, sourceKey, payload, dedupExtra)
}

func (s *asyncIngestService) enqueue(ctx context.Context, eventType, tenantUserID, sourceType, sourceKey string, payload any, dedupExtra string) error {
	if s == nil || s.eventRepo == nil {
		return nil
	}
	eventType = strings.TrimSpace(eventType)
	tenantUserID = strings.TrimSpace(tenantUserID)
	sourceType = strings.TrimSpace(sourceType)
	sourceKey = strings.TrimSpace(sourceKey)
	dedupExtra = strings.TrimSpace(dedupExtra)

	if tenantUserID == "" || eventType == "" || sourceType == "" || sourceKey == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return xerr.ErrServerError
	}

	now := time.Now()
	ev := &rag.AIIngestEvent{
		EventType:     eventType,
		TenantUserId:  tenantUserID,
		SourceType:    sourceType,
		SourceKey:     sourceKey,
		PayloadJson:   string(b),
		DedupKey:      buildDedupKey(tenantUserID, eventType, sourceType, sourceKey, dedupExtra),
		PublishStatus: rag.IngestPublishStatusPending,
		Status:        rag.IngestEventStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.eventRepo.Create(ctx, ev); err != nil {
		if isDuplicateKeyErr(err) {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return err
	}

	return nil
}

func buildDedupKey(tenantUserID, eventType, sourceType, sourceKey, dedupExtra string) string {
	raw := strings.TrimSpace(tenantUserID) + "|" +
		strings.TrimSpace(eventType) + "|" +
		strings.TrimSpace(sourceType) + "|" +
		strings.TrimSpace(sourceKey) + "|" +
		strings.TrimSpace(dedupExtra)

	sum := sha256.Sum256([]byte(raw))
	return "inc_" + hex.EncodeToString(sum[:])
}

func dedupByMinute() string {
	return strconv.FormatInt(time.Now().Unix()/60, 10)
}

func isDuplicateKeyErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "duplicate entry") {
		return true
	}
	if strings.Contains(s, "uniq_ai_event_dedup") {
		return true
	}
	return false
}
