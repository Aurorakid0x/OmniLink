package microservice

import "time"

// AIMicroserviceConfig 微服务配置表
type AIMicroserviceConfig struct {
	Id              int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ServiceType     string    `gorm:"column:service_type;type:varchar(50);uniqueIndex;not null"` // 服务类型：input_prediction/polish/digest
	IsEnabled       int8      `gorm:"column:is_enabled;type:tinyint;not null;default:1"`         // 是否启用
	ConfigJson      string    `gorm:"column:config_json;type:json;not null"`                     // 服务配置
	ModelConfigJson string    `gorm:"column:model_config_json;type:json;not null"`               // 模型配置
	PromptTemplate  string    `gorm:"column:prompt_template;type:mediumtext"`                    // Prompt模板
	CreatedAt       time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt       time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIMicroserviceConfig) TableName() string {
	return "ai_microservice_config"
}

// AIMicroserviceCallLog 微服务调用日志表
type AIMicroserviceCallLog struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	RequestId    string    `gorm:"column:request_id;type:char(20);uniqueIndex;not null"`
	TenantUserId string    `gorm:"column:tenant_user_id;type:char(20);index;not null"`
	ServiceType  string    `gorm:"column:service_type;type:varchar(50);index;not null"`
	InputText    string    `gorm:"column:input_text;type:mediumtext"`
	OutputText   string    `gorm:"column:output_text;type:mediumtext"`
	ContextJson  string    `gorm:"column:context_json;type:json"`
	LatencyMs    int       `gorm:"column:latency_ms;type:int"`
	TokensUsed   int       `gorm:"column:tokens_used;type:int"`
	IsCached     int8      `gorm:"column:is_cached;type:tinyint;default:0"`
	ErrorMsg     string    `gorm:"column:error_msg;type:text"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null;index"`
}

func (AIMicroserviceCallLog) TableName() string {
	return "ai_microservice_call_log"
}
