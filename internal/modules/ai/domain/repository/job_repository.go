package repository

import (
	"context"
	"time"

	"OmniLink/internal/modules/ai/domain/job"
)

type AIJobRepository interface {
	// Def CRUD
	CreateDef(ctx context.Context, def *job.AIJobDef) error
	GetActiveCronDefs(ctx context.Context) ([]*job.AIJobDef, error)
	GetDefsByEvent(ctx context.Context, eventKey string) ([]*job.AIJobDef, error)
	GetDefsByEventAndUser(ctx context.Context, eventKey string, userID string) ([]*job.AIJobDef, error)
	// DeactivateDef 软删除任务定义（仅对所属用户）
	DeactivateDef(ctx context.Context, defID int64, userID string) error

	// Inst CRUD
	CreateInst(ctx context.Context, inst *job.AIJobInst) error
	GetPendingInsts(ctx context.Context, limit int) ([]*job.AIJobInst, error)
	UpdateInstStatus(ctx context.Context, id int64, status int, result string) error
	IncrInstRetry(ctx context.Context, id int64) error
	// UpdateInstForRetry 更新重试次数并延后执行时间
	UpdateInstForRetry(ctx context.Context, id int64, nextTriggerAt time.Time, result string) error
}
