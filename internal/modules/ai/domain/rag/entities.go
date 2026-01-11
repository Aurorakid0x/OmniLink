package rag

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

const (
	CommonStatusDisabled int8 = 0 // 通用状态：禁用/不可用
	CommonStatusEnabled  int8 = 1 // 通用状态：启用
)

const (
	VectorEmbedStatusPending   int8 = 0  // 向量化状态：待处理/未开始
	VectorEmbedStatusSucceeded int8 = 1  // 向量化状态：成功（向量已写入向量库）
	VectorEmbedStatusFailed    int8 = -1 // 向量化状态：失败（可重试）
)

const (
	IngestEventStatusPending    int8 = 0  // 入库事件状态：待处理
	IngestEventStatusProcessing int8 = 1  // 入库事件状态：处理中
	IngestEventStatusSucceeded  int8 = 2  // 入库事件状态：已完成
	IngestEventStatusFailed     int8 = -1 // 入库事件状态：失败（可重试/待排障）
)

type AIKnowledgeBase struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement"`                                   // 主键，自增
	OwnerType string    `gorm:"column:owner_type;type:varchar(20);not null;uniqueIndex:uniq_ai_kb_owner"` // 归属主体类型（例如 user/agent）
	OwnerId   string    `gorm:"column:owner_id;type:char(20);not null;uniqueIndex:uniq_ai_kb_owner"`      // 归属主体 ID（例如用户 uuid）
	KBType    string    `gorm:"column:kb_type;type:varchar(30);not null;uniqueIndex:uniq_ai_kb_owner"`    // 知识库类型（例如 global/agent_private）
	Name      string    `gorm:"column:name;type:varchar(64);not null"`                                  // 知识库名称（展示用）
	Status    int8      `gorm:"column:status;type:tinyint;not null;default:1"`                          // 状态：0=禁用，1=启用（见 CommonStatus*）
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null"`                               // 创建时间
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null"`                               // 更新时间
}

func (AIKnowledgeBase) TableName() string { return "ai_knowledge_base" }

type AIKnowledgeSource struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`                                           // 主键，自增
	KBId         int64     `gorm:"column:kb_id;index:idx_ai_source_kb;not null;uniqueIndex:uniq_ai_source"`      // 所属知识库 ID（用于 KB 级别隔离/过滤）
	SourceType   string    `gorm:"column:source_type;type:varchar(30);not null;uniqueIndex:uniq_ai_source"`     // 数据源类型（例如 chat_private/chat_group/file_upload）
	SourceKey    string    `gorm:"column:source_key;type:varchar(128);not null;uniqueIndex:uniq_ai_source"`     // 数据源唯一键（在 SourceType 语义下定位一份来源，如 group_id/contact_id/file_id）
	TenantUserId string    `gorm:"column:tenant_user_id;type:char(20);not null;index:idx_ai_source_tenant;uniqueIndex:uniq_ai_source"` // 租户用户 ID（强隔离维度，检索过滤必须包含）
	ACLJson      string    `gorm:"column:acl_json;type:json"`                                                   // 权限/可见性描述（JSON，可选；用于后续“权限内检索”扩展）
	Version      int       `gorm:"column:version;type:int;not null;default:1"`                                  // 数据源版本号（策略/权限/内容变更时可递增，便于增量与重建）
	Status       int8      `gorm:"column:status;type:tinyint;not null;default:1"`                               // 状态：0=禁用，1=启用（见 CommonStatus*）
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`                                    // 创建时间
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`                                    // 更新时间
}

func (AIKnowledgeSource) TableName() string { return "ai_knowledge_source" }

type AIKnowledgeChunk struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	KBId         int64     `gorm:"column:kb_id;index:idx_ai_chunk_kb;not null"`
	SourceId     int64     `gorm:"column:source_id;index:idx_ai_chunk_source;not null"`
	ChunkKey     string    `gorm:"column:chunk_key;type:varchar(160);not null;uniqueIndex:uniq_ai_chunk"`
	ChunkIndex   int       `gorm:"column:chunk_index;type:int;not null"`
	Content      string    `gorm:"column:content;type:mediumtext"`
	ContentHash  string    `gorm:"column:content_hash;type:char(64);not null"`
	MetadataJson string    `gorm:"column:metadata_json;type:json"`
	Status       int8      `gorm:"column:status;type:tinyint;not null;default:1"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIKnowledgeChunk) TableName() string { return "ai_knowledge_chunk" }

type AIVectorRecord struct {
	Id              int64        `gorm:"column:id;primaryKey;autoIncrement"`
	ChunkId         int64        `gorm:"column:chunk_id;not null;uniqueIndex:uniq_ai_vector_chunk"`
	VectorStore     string       `gorm:"column:vector_store;type:varchar(20);not null"`
	Collection      string       `gorm:"column:collection;type:varchar(64);not null"`
	VectorId        string       `gorm:"column:vector_id;type:varchar(128);not null;uniqueIndex:uniq_ai_vector"`
	EmbeddingProvider string     `gorm:"column:embedding_provider;type:varchar(30);not null"`
	EmbeddingModel  string       `gorm:"column:embedding_model;type:varchar(64);not null"`
	Dim             int          `gorm:"column:dim;type:int;not null"`
	EmbedStatus     int8         `gorm:"column:embed_status;type:tinyint;not null;default:0;index:idx_ai_vector_status"`
	ErrorMsg        string       `gorm:"column:error_msg;type:varchar(255)"`
	EmbeddedAt      sql.NullTime `gorm:"column:embedded_at;type:datetime"`
	CreatedAt       time.Time    `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt       time.Time    `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIVectorRecord) TableName() string { return "ai_vector_record" }

type AIIngestEvent struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	EventType    string    `gorm:"column:event_type;type:varchar(40);not null"`
	TenantUserId string    `gorm:"column:tenant_user_id;type:char(20);not null;index:idx_ai_event_tenant"`
	PayloadJson  string    `gorm:"column:payload_json;type:json"`
	DedupKey     string    `gorm:"column:dedup_key;type:varchar(160);not null;uniqueIndex:uniq_ai_event_dedup"`
	Status       int8      `gorm:"column:status;type:tinyint;not null;default:0;index:idx_ai_event_status"`
	RetryCount   int       `gorm:"column:retry_count;type:int;not null;default:0"`
	NextRetryAt  sql.NullTime `gorm:"column:next_retry_at;type:datetime;index:idx_ai_event_next_retry"`
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIIngestEvent) TableName() string { return "ai_ingest_event" }

type AIChatSession struct {
	Id          int64          `gorm:"column:id;primaryKey;autoIncrement"`
	UserId      string         `gorm:"column:user_id;type:char(20);not null;index:idx_ai_chat_session_user"`
	SessionType string         `gorm:"column:session_type;type:varchar(30);not null"`
	AgentId     sql.NullString `gorm:"column:agent_id;type:char(20)"`
	Title       string         `gorm:"column:title;type:varchar(128);not null"`
	CreatedAt   time.Time      `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt   time.Time      `gorm:"column:updated_at;type:datetime;not null"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;type:datetime;index"`
}

func (AIChatSession) TableName() string { return "ai_chat_session" }

type AIChatMessage struct {
	Id          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	AISessionId int64     `gorm:"column:ai_session_id;not null;index:idx_ai_chat_message_session"`
	Role        string    `gorm:"column:role;type:varchar(10);not null"`
	Content     string    `gorm:"column:content;type:mediumtext"`
	ToolCallJson string   `gorm:"column:tool_call_json;type:json"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null"`
}

func (AIChatMessage) TableName() string { return "ai_chat_message" }

type AIAgent struct {
	Id          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	AgentUuid   string    `gorm:"column:agent_uuid;type:char(20);not null;uniqueIndex:uniq_ai_agent_uuid"`
	OwnerUserId string    `gorm:"column:owner_user_id;type:char(20);not null;index:idx_ai_agent_owner"`
	Name        string    `gorm:"column:name;type:varchar(64);not null"`
	SystemPrompt string   `gorm:"column:system_prompt;type:text"`
	KBId        int64     `gorm:"column:kb_id;not null"`
	Status      int8      `gorm:"column:status;type:tinyint;not null;default:1"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIAgent) TableName() string { return "ai_agent" }

type AIToolRegistry struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	ToolName  string    `gorm:"column:tool_name;type:varchar(64);not null;uniqueIndex:uniq_ai_tool_name"`
	SchemaJson string   `gorm:"column:schema_json;type:json"`
	Status    int8      `gorm:"column:status;type:tinyint;not null;default:1"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIToolRegistry) TableName() string { return "ai_tool_registry" }

type AIAgentToolBinding struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	AgentId   int64     `gorm:"column:agent_id;not null;uniqueIndex:uniq_ai_agent_tool"`
	ToolName  string    `gorm:"column:tool_name;type:varchar(64);not null;uniqueIndex:uniq_ai_agent_tool"`
	PolicyJson string   `gorm:"column:policy_json;type:json"`
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIAgentToolBinding) TableName() string { return "ai_agent_tool_binding" }

type AIUploadedFile struct {
	Id          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	OwnerUserId string    `gorm:"column:owner_user_id;type:char(20);not null;index:idx_ai_uploaded_file_owner"`
	AgentId     sql.NullInt64 `gorm:"column:agent_id"`
	KBId        int64     `gorm:"column:kb_id;not null;index:idx_ai_uploaded_file_kb"`
	Filename    string    `gorm:"column:filename;type:varchar(255);not null"`
	FileType    string    `gorm:"column:file_type;type:varchar(20);not null"`
	StorageURL  string    `gorm:"column:storage_url;type:varchar(512);not null"`
	ContentHash string    `gorm:"column:content_hash;type:char(64);not null;index:idx_ai_uploaded_file_hash"`
	Status      int8      `gorm:"column:status;type:tinyint;not null;default:1"`
	CreatedAt   time.Time `gorm:"column:created_at;type:datetime;not null"`
	UpdatedAt   time.Time `gorm:"column:updated_at;type:datetime;not null"`
}

func (AIUploadedFile) TableName() string { return "ai_uploaded_file" }
