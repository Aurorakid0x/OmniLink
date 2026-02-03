package repository

import (
	"OmniLink/internal/modules/ai/domain/job"
	"context"
)

type AIJobRepository interface {
	// Def CRUD
	CreateDef(ctx context.Context, def *job.AIJobDef) error
	GetActiveCronDefs(ctx context.Context) ([]*job.AIJobDef, error)
	GetDefsByEvent(ctx context.Context, eventKey string) ([]*job.AIJobDef, error)
	GetDefsByEventAndUser(ctx context.Context, eventKey string, userID string) ([]*job.AIJobDef, error)

	// Inst CRUD
	CreateInst(ctx context.Context, inst *job.AIJobInst) error
	GetPendingInsts(ctx context.Context, limit int) ([]*job.AIJobInst, error)
	UpdateInstStatus(ctx context.Context, id int64, status int, result string) error
	IncrInstRetry(ctx context.Context, id int64) error
}
