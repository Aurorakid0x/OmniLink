package agent

import (
	"time"
)

const (
	AgentStatusEnabled  int8 = 1 // Agent启用
	AgentStatusDisabled int8 = 0 // Agent禁用
)

const (
	OwnerTypeUser   = "user"   // 用户创建的Agent
	OwnerTypeSystem = "system" // 系统预置的Agent
)

const (
	KBTypeGlobal       = "global"        // 全局知识库
	KBTypeAgentPrivate = "agent_private" // Agent私有知识库
)

// AIAgent Agent管理表（统一管理用户自定义Agent和系统Agent）
type AIAgent struct {
	Id            int64     `gorm:"column:id;primaryKey;autoIncrement"`                 // 主键，自增
	AgentId       string    `gorm:"column:agent_id;type:char(20);uniqueIndex;not null"` // Agent唯一ID
	OwnerType     string    `gorm:"column:owner_type;type:varchar(20);not null"`        // 归属类型：user/system
	OwnerId       string    `gorm:"column:owner_id;type:char(20);index"`                // 归属ID（若为用户Agent，则为tenant_user_id）
	Name          string    `gorm:"column:name;type:varchar(64);not null"`              // Agent名称
	Description   string    `gorm:"column:description;type:varchar(255)"`               // Agent描述
	PersonaPrompt string    `gorm:"column:persona_prompt;type:mediumtext"`              // 人格化Prompt（系统人设）
	Status        int8      `gorm:"column:status;type:tinyint;not null;default:1"`      // 状态：1=enabled, 0=disabled
	KBType        string    `gorm:"column:kb_type;type:varchar(30)"`                    // 知识库类型：global/agent_private
	KBId          int64     `gorm:"column:kb_id;type:bigint"`                           // 关联的知识库ID
	ToolsJson     string    `gorm:"column:tools_json;type:json"`                        // MCP工具授权列表（JSON数组，后续扩展）
	CreatedAt     time.Time `gorm:"column:created_at;type:datetime;not null"`           // 创建时间
	UpdatedAt     time.Time `gorm:"column:updated_at;type:datetime;not null"`           // 更新时间
}

func (AIAgent) TableName() string {
	return "ai_agent"
}
