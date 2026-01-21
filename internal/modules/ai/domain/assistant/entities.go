package assistant

import (
	"time"
)

const (
	SessionStatusActive   int8 = 1 // 活跃会话
	SessionStatusArchived int8 = 0 // 已归档会话
)

// AIAssistantSession 全局AI助手会话表（独立于IM会话）
type AIAssistantSession struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`                   // 主键，自增
	SessionId    string    `gorm:"column:session_id;type:char(20);uniqueIndex;not null"` // 会话唯一ID（对外使用）
	TenantUserId string    `gorm:"column:tenant_user_id;type:char(20);index;not null"`   // 租户用户ID
	Title        string    `gorm:"column:title;type:varchar(64);not null"`               // 会话标题
	Status       int8      `gorm:"column:status;type:tinyint;not null;default:1"`        // 状态：1=active, 0=archived
	AgentId      string    `gorm:"column:agent_id;type:char(20)"`                        // 关联的Agent ID（可空，后续扩展）
	PersonaId    string    `gorm:"column:persona_id;type:char(20)"`                      // 关联的Persona ID（可空，后续扩展）
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`             // 创建时间
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`             // 更新时间
}

func (AIAssistantSession) TableName() string {
	return "ai_assistant_session"
}

// AIAssistantMessage 全局AI助手消息表
type AIAssistantMessage struct {
	Id            int64     `gorm:"column:id;primaryKey;autoIncrement"`                              // 主键，自增
	SessionId     string    `gorm:"column:session_id;type:char(20);index;not null"`                  // 所属会话ID
	Role          string    `gorm:"column:role;type:varchar(16);not null"`                           // 角色：system/user/assistant
	Content       string    `gorm:"column:content;type:mediumtext"`                                  // 消息内容
	CitationsJson string    `gorm:"column:citations_json;type:json"`                                 // 本轮检索引用（JSON数组）
	TokensJson    string    `gorm:"column:tokens_json;type:json"`                                    // Token统计（prompt_tokens/answer_tokens/total_tokens）
	CreatedAt     time.Time `gorm:"column:created_at;type:datetime;not null;index:idx_session_time"` // 创建时间（索引，用于历史消息查询）
}

func (AIAssistantMessage) TableName() string {
	return "ai_assistant_message"
}
