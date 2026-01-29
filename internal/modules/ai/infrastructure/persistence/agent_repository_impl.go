package persistence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/agent"
	"OmniLink/internal/modules/ai/domain/repository"

	"gorm.io/gorm"
)

type agentRepositoryImpl struct {
	db *gorm.DB
}

func NewAgentRepository(db *gorm.DB) repository.AgentRepository {
	return &agentRepositoryImpl{db: db}
}

func (r *agentRepositoryImpl) CreateAgent(ctx context.Context, ag *agent.AIAgent) error {
	return r.db.WithContext(ctx).Create(ag).Error
}

func (r *agentRepositoryImpl) GetAgentByID(ctx context.Context, agentId, ownerId string) (*agent.AIAgent, error) {
	agentId = strings.TrimSpace(agentId)
	ownerId = strings.TrimSpace(ownerId)
	if agentId == "" {
		return nil, nil
	}

	var ag agent.AIAgent
	query := r.db.WithContext(ctx).Where("agent_id = ?", agentId)

	// 权限控制：只能访问自己的Agent或系统Agent
	if ownerId != "" {
		query = query.Where("(owner_id = ? OR owner_type = ?)", ownerId, agent.OwnerTypeSystem)
	} else {
		query = query.Where("owner_type = ?", agent.OwnerTypeSystem)
	}

	err := query.Take(&ag).Error
	if err == nil {
		return &ag, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func (r *agentRepositoryImpl) ListAgents(ctx context.Context, ownerId string, limit, offset int) ([]*agent.AIAgent, error) {
	ownerId = strings.TrimSpace(ownerId)
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	var agents []*agent.AIAgent
	query := r.db.WithContext(ctx).
		Where("status = ?", agent.AgentStatusEnabled)

	// 返回用户自己的Agent + 系统Agent
	if ownerId != "" {
		query = query.Where("(owner_id = ? OR owner_type = ?)", ownerId, agent.OwnerTypeSystem)
	} else {
		query = query.Where("owner_type = ?", agent.OwnerTypeSystem)
	}

	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&agents).Error
	return agents, err
}

func (r *agentRepositoryImpl) UpdateAgent(ctx context.Context, agentId, ownerId string, updates map[string]interface{}) error {
	agentId = strings.TrimSpace(agentId)
	ownerId = strings.TrimSpace(ownerId)
	if agentId == "" || ownerId == "" {
		return nil
	}

	updates["updated_at"] = time.Now()
	return r.db.WithContext(ctx).Model(&agent.AIAgent{}).
		Where("agent_id = ? AND owner_id = ?", agentId, ownerId).
		Updates(updates).Error
}

func (r *agentRepositoryImpl) DisableAgent(ctx context.Context, agentId, ownerId string) error {
	agentId = strings.TrimSpace(agentId)
	ownerId = strings.TrimSpace(ownerId)
	if agentId == "" || ownerId == "" {
		return nil
	}

	return r.db.WithContext(ctx).Model(&agent.AIAgent{}).
		Where("agent_id = ? AND owner_id = ?", agentId, ownerId).
		Updates(map[string]interface{}{
			"status":     agent.AgentStatusDisabled,
			"updated_at": time.Now(),
		}).Error
}

func (r *agentRepositoryImpl) GetSystemGlobalAgent(ctx context.Context, tenantUserID string) (*agent.AIAgent, error) {
	var ag agent.AIAgent
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND is_system_global = ?", tenantUserID, agent.IsSystemGlobalTrue).
		First(&ag).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 未找到返回nil，不报错
		}
		return nil, err
	}
	return &ag, nil
}

func (r *agentRepositoryImpl) CreateSystemGlobalAgent(ctx context.Context, ag *agent.AIAgent) error {
	// 检查该用户是否已有系统全局助手
	existing, err := r.GetSystemGlobalAgent(ctx, ag.OwnerId)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("user already has a system global agent")
	}

	// 强制设置关键字段
	ag.IsSystemGlobal = agent.IsSystemGlobalTrue
	ag.OwnerType = agent.OwnerTypeUser // 注意：虽然是系统级，但归属仍为用户
	ag.Status = agent.AgentStatusEnabled

	return r.db.WithContext(ctx).Create(ag).Error
}
