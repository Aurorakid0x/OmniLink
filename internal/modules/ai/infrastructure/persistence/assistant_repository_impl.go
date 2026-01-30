package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/repository"

	"gorm.io/gorm"
)

type assistantSessionRepositoryImpl struct {
	db *gorm.DB
}

func NewAssistantSessionRepository(db *gorm.DB) repository.AssistantSessionRepository {
	return &assistantSessionRepositoryImpl{db: db}
}

func (r *assistantSessionRepositoryImpl) CreateSession(ctx context.Context, session *assistant.AIAssistantSession) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *assistantSessionRepositoryImpl) GetSessionByID(ctx context.Context, sessionId, tenantUserId string) (*assistant.AIAssistantSession, error) {
	sessionId = strings.TrimSpace(sessionId)
	tenantUserId = strings.TrimSpace(tenantUserId)
	if sessionId == "" || tenantUserId == "" {
		return nil, nil
	}

	var session assistant.AIAssistantSession
	err := r.db.WithContext(ctx).
		Where("session_id = ? AND tenant_user_id = ?", sessionId, tenantUserId).
		Take(&session).Error
	if err == nil {
		return &session, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (r *assistantSessionRepositoryImpl) ListSessions(ctx context.Context, tenantUserId string, limit, offset int) ([]*assistant.AIAssistantSession, error) {
	tenantUserId = strings.TrimSpace(tenantUserId)
	if tenantUserId == "" {
		return []*assistant.AIAssistantSession{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var sessions []*assistant.AIAssistantSession
	err := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND status = ?", tenantUserId, assistant.SessionStatusActive).
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&sessions).Error
	return sessions, err
}

func (r *assistantSessionRepositoryImpl) UpdateSessionTitle(ctx context.Context, sessionId, tenantUserId, title string) error {
	sessionId = strings.TrimSpace(sessionId)
	tenantUserId = strings.TrimSpace(tenantUserId)
	title = strings.TrimSpace(title)
	if sessionId == "" || tenantUserId == "" {
		return nil
	}

	return r.db.WithContext(ctx).Model(&assistant.AIAssistantSession{}).
		Where("session_id = ? AND tenant_user_id = ?", sessionId, tenantUserId).
		Updates(map[string]interface{}{
			"title":      title,
			"updated_at": time.Now(),
		}).Error
}

func (r *assistantSessionRepositoryImpl) UpdateSessionAgent(ctx context.Context, sessionId, tenantUserId, agentId string) error {
	sessionId = strings.TrimSpace(sessionId)
	tenantUserId = strings.TrimSpace(tenantUserId)
	agentId = strings.TrimSpace(agentId)
	if sessionId == "" || tenantUserId == "" {
		return nil
	}

	return r.db.WithContext(ctx).Model(&assistant.AIAssistantSession{}).
		Where("session_id = ? AND tenant_user_id = ?", sessionId, tenantUserId).
		Updates(map[string]interface{}{
			"agent_id":   agentId,
			"updated_at": time.Now(),
		}).Error
}

func (r *assistantSessionRepositoryImpl) UpdateSessionUpdatedAt(ctx context.Context, sessionId string) error {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return nil
	}

	return r.db.WithContext(ctx).Model(&assistant.AIAssistantSession{}).
		Where("session_id = ?", sessionId).
		Update("updated_at", time.Now()).Error
}

type assistantMessageRepositoryImpl struct {
	db *gorm.DB
}

func NewAssistantMessageRepository(db *gorm.DB) repository.AssistantMessageRepository {
	return &assistantMessageRepositoryImpl{db: db}
}

func (r *assistantMessageRepositoryImpl) SaveMessage(ctx context.Context, message *assistant.AIAssistantMessage) error {
	if message == nil {
		return nil
	}
	if strings.TrimSpace(message.CitationsJson) == "" {
		message.CitationsJson = "[]"
	}
	if strings.TrimSpace(message.TokensJson) == "" {
		message.TokensJson = "{}"
	}
	if strings.TrimSpace(message.MetadataJson) == "" {
		message.MetadataJson = "{}"
	}
	if strings.TrimSpace(message.RenderDataJson) == "" {
		message.RenderDataJson = "{}"
	}
	return r.db.WithContext(ctx).Create(message).Error
}

func (r *assistantMessageRepositoryImpl) ListRecentMessages(ctx context.Context, sessionId string, limit int) ([]*assistant.AIAssistantMessage, error) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return []*assistant.AIAssistantMessage{}, nil
	}
	if limit <= 0 {
		limit = 12
	}

	var messages []*assistant.AIAssistantMessage
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionId).
		Order("created_at DESC").
		Limit(limit).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages, nil
}

func (r *assistantMessageRepositoryImpl) ListMessages(ctx context.Context, sessionId string, limit, offset int) ([]*assistant.AIAssistantMessage, error) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return []*assistant.AIAssistantMessage{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var messages []*assistant.AIAssistantMessage
	err := r.db.WithContext(ctx).
		Where("session_id = ?", sessionId).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&messages).Error
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *assistantMessageRepositoryImpl) CountSessionMessages(ctx context.Context, sessionId string) (int64, error) {
	sessionId = strings.TrimSpace(sessionId)
	if sessionId == "" {
		return 0, nil
	}

	var count int64
	err := r.db.WithContext(ctx).Model(&assistant.AIAssistantMessage{}).
		Where("session_id = ?", sessionId).
		Count(&count).Error
	return count, err
}

func (r *assistantSessionRepositoryImpl) GetSystemGlobalSession(ctx context.Context, tenantUserID string) (*assistant.AIAssistantSession, error) {
	var session assistant.AIAssistantSession
	err := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND session_type = ?", tenantUserID, assistant.SessionTypeSystemGlobal).
		First(&session).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *assistantSessionRepositoryImpl) CreateSystemGlobalSession(ctx context.Context, session *assistant.AIAssistantSession) error {
	// 检查该用户是否已有系统助手会话
	existing, err := r.GetSystemGlobalSession(ctx, session.TenantUserId)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("user already has a system global session")
	}

	// 强制设置关键字段
	session.SessionType = assistant.SessionTypeSystemGlobal
	session.IsPinned = assistant.IsPinnedTrue
	session.IsDeletable = assistant.IsDeletableFalse
	session.Status = assistant.SessionStatusActive

	return r.db.WithContext(ctx).Create(session).Error
}

func (r *assistantSessionRepositoryImpl) ListSessionsWithType(ctx context.Context, tenantUserID string, sessionType string, limit, offset int) ([]*assistant.AIAssistantSession, error) {
	var sessions []*assistant.AIAssistantSession

	query := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND status = ?", tenantUserID, assistant.SessionStatusActive)

	// 如果指定了类型，则过滤
	if sessionType != "" {
		query = query.Where("session_type = ?", sessionType)
	}

	// 按置顶和更新时间排序
	query = query.Order("is_pinned DESC, updated_at DESC")

	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}

	err := query.Find(&sessions).Error
	return sessions, err
}
