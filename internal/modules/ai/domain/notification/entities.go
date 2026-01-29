package notification

//系统通知领域（预留，用于模块一的离线总结和主动通知功能）
// 本阶段仅定义实体，业务逻辑在后续实现
import "time"

const (
	// 通知类型
	TypeOfflineSummary = "offline_summary" // 离线总结
	TypeReminder       = "reminder"        // 提醒
	TypeInsight        = "insight"         // 洞察

	// 通知状态
	StatusPending = 0 // 待推送
	StatusPushed  = 1 // 已推送
	StatusRead    = 2 // 已读
)

// AISystemNotification 系统通知实体（预留，用于离线总结/主动通知）
type AISystemNotification struct {
	Id             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	NotificationId string     `gorm:"column:notification_id;type:char(20);uniqueIndex;not null"`
	TenantUserId   string     `gorm:"column:tenant_user_id;type:char(20);index;not null"`
	SessionId      string     `gorm:"column:session_id;type:char(20);index;not null"`
	Type           string     `gorm:"column:type;type:varchar(30);not null"`
	Title          string     `gorm:"column:title;type:varchar(100)"`
	Content        string     `gorm:"column:content;type:mediumtext"`
	TriggerSource  string     `gorm:"column:trigger_source;type:varchar(50)"`
	Status         int8       `gorm:"column:status;type:tinyint;not null;default:0"`
	PushedAt       *time.Time `gorm:"column:pushed_at;type:datetime"`
	ReadAt         *time.Time `gorm:"column:read_at;type:datetime"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:datetime;not null"`
}

func (AISystemNotification) TableName() string {
	return "ai_system_notification"
}
