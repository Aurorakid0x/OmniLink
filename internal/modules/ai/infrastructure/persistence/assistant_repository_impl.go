package persistence

import (
	"context"
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
