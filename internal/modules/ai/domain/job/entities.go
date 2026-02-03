package job

import (
	"time"
)

// 任务类型
const (
	TriggerTypeOnce  = 0 // 一次性任务 (Time Delay)
	TriggerTypeCron  = 1 // 定时任务 (Recurring)
	TriggerTypeEvent = 2 // 事件驱动
)

// 任务执行状态
const (
	JobStatusPending   = 0 // 待执行
	JobStatusRunning   = 1 // 执行中
	JobStatusCompleted = 2 // 完成
	JobStatusFailed    = 3 // 失败
)

// AIJobDef (任务定义/规则)
type AIJobDef struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	TenantUserID string    `gorm:"column:tenant_user_id;index;type:varchar(64)"` // 规则归属人
	AgentID      string    `gorm:"column:agent_id;not null;type:varchar(64)"`    // 执行的 Agent
	Title        string    `gorm:"column:title;type:varchar(100)"`
	TriggerType  int       `gorm:"column:trigger_type;not null"`      // 0:Once, 1:Cron, 2:Event
	CronExpr     string    `gorm:"column:cron_expr;type:varchar(64)"` // e.g. "0 8 * * *"
	EventKey     string    `gorm:"column:event_key;type:varchar(64)"` // e.g. "user_login"
	Prompt       string    `gorm:"column:prompt;type:text"`           // 核心 Prompt
	IsActive     bool      `gorm:"column:is_active;default:true"`     // 是否启用
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (AIJobDef) TableName() string {
	return "ai_job_def"
}

// AIJobInst (任务实例/执行日志)
type AIJobInst struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	JobDefID      int64      `gorm:"column:job_def_id;index"` // 关联的规则ID
	TenantUserID  string     `gorm:"column:tenant_user_id;index;type:varchar(64)"`
	AgentID       string     `gorm:"column:agent_id;type:varchar(64)"`
	Prompt        string     `gorm:"column:prompt;type:text"`
	Status        int        `gorm:"column:status;default:0;index"`
	TriggerAt     time.Time  `gorm:"column:trigger_at;index"` // 计划执行时间
	StartedAt     *time.Time `gorm:"column:started_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at"`
	RetryCount    int        `gorm:"column:retry_count;default:0"`
	ResultSummary string     `gorm:"column:result_summary;type:text"` // 错误信息或执行简报
	CreatedAt     time.Time  `gorm:"column:created_at"`
}

func (AIJobInst) TableName() string {
	return "ai_job_inst"
}
