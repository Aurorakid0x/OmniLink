package entity

import (
	"time"
)

// MessageMention 消息提及表
type MessageMention struct {
	Id              int64     `gorm:"column:id;primaryKey;comment:自增id"`
	MessageUuid     string    `gorm:"column:message_uuid;index;type:char(20);not null;comment:消息uuid"`
	SessionId       string    `gorm:"column:session_id;index;type:char(20);not null;comment:会话uuid"`
	MentionedUserId string    `gorm:"column:mentioned_user_id;index;type:char(20);not null;comment:被提及用户ID"`
	MentionType     int8      `gorm:"column:mention_type;not null;default:0;comment:提及类型：0.指定用户，1.全体成员"`
	CreatedAt       time.Time `gorm:"column:created_at;not null;comment:创建时间"`
}

func (MessageMention) TableName() string {
	return "message_mention"
}
