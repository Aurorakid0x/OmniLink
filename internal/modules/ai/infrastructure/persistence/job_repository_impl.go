package persistence

import (
	"context"
	"time"

	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"

	"gorm.io/gorm"
)

type aiJobRepoImpl struct {
	db *gorm.DB
}

func NewAIJobRepository(db *gorm.DB) repository.AIJobRepository {
	return &aiJobRepoImpl{db: db}
}

func (r *aiJobRepoImpl) CreateDef(ctx context.Context, def *job.AIJobDef) error {
	return r.db.WithContext(ctx).Create(def).Error
}

func (r *aiJobRepoImpl) GetActiveCronDefs(ctx context.Context) ([]*job.AIJobDef, error) {
	var defs []*job.AIJobDef
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND trigger_type = ?", true, job.TriggerTypeCron).
		Find(&defs).Error
	return defs, err
}

func (r *aiJobRepoImpl) GetDefsByEvent(ctx context.Context, eventKey string) ([]*job.AIJobDef, error) {
	var defs []*job.AIJobDef
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND trigger_type = ? AND event_key = ?", true, job.TriggerTypeEvent, eventKey).
		Find(&defs).Error
	return defs, err
}

func (r *aiJobRepoImpl) GetDefsByEventAndUser(ctx context.Context, eventKey string, userID string) ([]*job.AIJobDef, error) {
	var defs []*job.AIJobDef
	err := r.db.WithContext(ctx).
		Where("is_active = ? AND trigger_type = ? AND event_key = ? AND tenant_user_id = ?",
			true, job.TriggerTypeEvent, eventKey, userID).
		Find(&defs).Error
	return defs, err
}

func (r *aiJobRepoImpl) DeactivateDef(ctx context.Context, defID int64, userID string) error {
	if defID <= 0 || userID == "" {
		return nil
	}
	// 软删除规则：保留历史实例记录
	return r.db.WithContext(ctx).Model(&job.AIJobDef{}).
		Where("id = ? AND tenant_user_id = ?", defID, userID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"updated_at": time.Now(),
		}).Error
}

func (r *aiJobRepoImpl) CreateInst(ctx context.Context, inst *job.AIJobInst) error {
	return r.db.WithContext(ctx).Create(inst).Error
}

func (r *aiJobRepoImpl) GetPendingInsts(ctx context.Context, limit int) ([]*job.AIJobInst, error) {
	var insts []*job.AIJobInst
	// 查找待执行且时间已到的任务
	err := r.db.WithContext(ctx).
		Where("status = ? AND trigger_at <= ?", job.JobStatusPending, time.Now()).
		Order("trigger_at ASC").
		Limit(limit).
		Find(&insts).Error
	return insts, err
}

func (r *aiJobRepoImpl) UpdateInstStatus(ctx context.Context, id int64, status int, result string) error {
	updates := map[string]interface{}{
		"status":         status,
		"result_summary": result,
	}
	if status == job.JobStatusRunning {
		updates["started_at"] = time.Now()
	}
	if status == job.JobStatusCompleted || status == job.JobStatusFailed {
		updates["finished_at"] = time.Now()
	}
	return r.db.WithContext(ctx).Model(&job.AIJobInst{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *aiJobRepoImpl) IncrInstRetry(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Model(&job.AIJobInst{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + ?", 1)).Error
}

func (r *aiJobRepoImpl) UpdateInstForRetry(ctx context.Context, id int64, nextTriggerAt time.Time, result string) error {
	if id <= 0 {
		return nil
	}
	// 延后执行时间，避免失败任务频繁重试
	return r.db.WithContext(ctx).Model(&job.AIJobInst{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":         job.JobStatusPending,
			"trigger_at":     nextTriggerAt,
			"result_summary": result,
		}).Error
}
