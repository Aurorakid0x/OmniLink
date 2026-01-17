package rag

import (
	"database/sql"
	"time"
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

const (
	IngestPublishStatusPending    int8 = 0  // 待投递
	IngestPublishStatusPublishing int8 = 1  // 投递中
	IngestPublishStatusPublished  int8 = 2  // 已投递
	IngestPublishStatusFailed     int8 = -1 // 投递失败（可重试）
)

const (
	BackfillJobStatusPending   int8 = 0
	BackfillJobStatusRunning   int8 = 1
	BackfillJobStatusSucceeded int8 = 2
	BackfillJobStatusFailed    int8 = -1
	BackfillJobStatusCanceled  int8 = -2
)

// AIKnowledgeBase 知识库主表（按 owner 维度区分不同知识库）
type AIKnowledgeBase struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement"`                                       // 主键，自增
	OwnerType string    `gorm:"column:owner_type;type:varchar(20);not null;uniqueIndex:uniq_ai_kb_owner"` // 归属主体类型（例如 user/agent）
	OwnerId   string    `gorm:"column:owner_id;type:char(20);not null;uniqueIndex:uniq_ai_kb_owner"`      // 归属主体 ID（例如用户 uuid）
	KBType    string    `gorm:"column:kb_type;type:varchar(30);not null;uniqueIndex:uniq_ai_kb_owner"`    // 知识库类型（例如 global/agent_private）
	Name      string    `gorm:"column:name;type:varchar(64);not null"`                                    // 知识库名称（展示用）
	Status    int8      `gorm:"column:status;type:tinyint;not null;default:1"`                            // 状态：0=禁用，1=启用（见 CommonStatus*）
	CreatedAt time.Time `gorm:"column:created_at;type:datetime;not null"`                                 // 创建时间
	UpdatedAt time.Time `gorm:"column:updated_at;type:datetime;not null"`                                 // 更新时间
}

func (AIKnowledgeBase) TableName() string { return "ai_knowledge_base" }

// AIKnowledgeSource 知识库数据源表（描述一份可回填/可索引的来源）
type AIKnowledgeSource struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`                                                                 // 主键，自增
	KBId         int64     `gorm:"column:kb_id;index:idx_ai_source_kb;not null;uniqueIndex:uniq_ai_source"`                            // 所属知识库 ID（用于 KB 级别隔离/过滤）
	SourceType   string    `gorm:"column:source_type;type:varchar(30);not null;uniqueIndex:uniq_ai_source"`                            // 数据源类型（例如 chat_private/chat_group/file_upload）
	SourceKey    string    `gorm:"column:source_key;type:varchar(128);not null;uniqueIndex:uniq_ai_source"`                            // 数据源唯一键（在 SourceType 语义下定位一份来源，如 group_id/contact_id/file_id）
	TenantUserId string    `gorm:"column:tenant_user_id;type:char(20);not null;index:idx_ai_source_tenant;uniqueIndex:uniq_ai_source"` // 租户用户 ID（强隔离维度，检索过滤必须包含）
	ACLJson      string    `gorm:"column:acl_json;type:json"`                                                                          // 权限/可见性描述（JSON，可选；用于后续“权限内检索”扩展）
	Version      int       `gorm:"column:version;type:int;not null;default:1"`                                                         // 数据源版本号（策略/权限/内容变更时可递增，便于增量与重建）
	Status       int8      `gorm:"column:status;type:tinyint;not null;default:1"`                                                      // 状态：0=禁用，1=启用（见 CommonStatus*）
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`                                                           // 创建时间
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`                                                           // 更新时间
}

func (AIKnowledgeSource) TableName() string { return "ai_knowledge_source" }

// AIKnowledgeChunk 知识库文本分片表（把一份来源切分成多个 chunk）
type AIKnowledgeChunk struct {
	Id           int64     `gorm:"column:id;primaryKey;autoIncrement"`                                    // 主键
	KBId         int64     `gorm:"column:kb_id;index:idx_ai_chunk_kb;not null"`                           // 所属知识库 ID
	SourceId     int64     `gorm:"column:source_id;index:idx_ai_chunk_source;not null"`                   // 所属数据源 ID
	ChunkKey     string    `gorm:"column:chunk_key;type:varchar(160);not null;uniqueIndex:uniq_ai_chunk"` // 分片唯一键
	ChunkIndex   int       `gorm:"column:chunk_index;type:int;not null"`                                  // 分片序号
	Content      string    `gorm:"column:content;type:mediumtext"`                                        // 分片内容
	ContentHash  string    `gorm:"column:content_hash;type:char(64);not null"`                            // 内容哈希
	MetadataJson string    `gorm:"column:metadata_json;type:json"`                                        // 元数据（JSON）
	Status       int8      `gorm:"column:status;type:tinyint;not null;default:1"`                         // 状态（见 CommonStatus*）
	CreatedAt    time.Time `gorm:"column:created_at;type:datetime;not null"`                              // 创建时间
	UpdatedAt    time.Time `gorm:"column:updated_at;type:datetime;not null"`                              // 更新时间
}

func (AIKnowledgeChunk) TableName() string { return "ai_knowledge_chunk" }

// AIVectorRecord 向量写入记录表（chunk 与向量库记录的映射与状态）
type AIVectorRecord struct {
	Id                int64        `gorm:"column:id;primaryKey;autoIncrement"`                                             // 主键
	ChunkId           int64        `gorm:"column:chunk_id;not null;uniqueIndex:uniq_ai_vector_chunk"`                      // 关联 chunk_id
	VectorStore       string       `gorm:"column:vector_store;type:varchar(20);not null"`                                  // 向量库类型（milvus 等）
	Collection        string       `gorm:"column:collection;type:varchar(64);not null"`                                    // Collection 名称
	VectorId          string       `gorm:"column:vector_id;type:varchar(128);not null;uniqueIndex:uniq_ai_vector"`         // 向量记录 ID
	EmbeddingProvider string       `gorm:"column:embedding_provider;type:varchar(30);not null"`                            // 向量化供应商
	EmbeddingModel    string       `gorm:"column:embedding_model;type:varchar(64);not null"`                               // 向量化模型
	Dim               int          `gorm:"column:dim;type:int;not null"`                                                   // 向量维度
	EmbedStatus       int8         `gorm:"column:embed_status;type:tinyint;not null;default:0;index:idx_ai_vector_status"` // 向量化状态（见 VectorEmbedStatus*）
	ErrorMsg          string       `gorm:"column:error_msg;type:varchar(255)"`                                             // 错误信息
	EmbeddedAt        sql.NullTime `gorm:"column:embedded_at;type:datetime"`                                               // 完成时间
	CreatedAt         time.Time    `gorm:"column:created_at;type:datetime;not null"`                                       // 创建时间
	UpdatedAt         time.Time    `gorm:"column:updated_at;type:datetime;not null"`                                       // 更新时间
}

func (AIVectorRecord) TableName() string { return "ai_vector_record" }

// AIIngestEvent 入库事件表（Outbox：由 DB 可靠投递到 Kafka 并被下游消费）
type AIIngestEvent struct {
	Id             int64         `gorm:"column:id;primaryKey;autoIncrement"`                                               // 主键
	EventType      string        `gorm:"column:event_type;type:varchar(40);not null;index:idx_ai_event_type"`              // 事件类型
	TenantUserId   string        `gorm:"column:tenant_user_id;type:char(20);not null;index:idx_ai_event_tenant"`           // 租户用户 ID
	BackfillJobId  sql.NullInt64 `gorm:"column:backfill_job_id;type:bigint;index:idx_ai_event_job"`                        // 回填任务 ID（可空）
	SourceType     string        `gorm:"column:source_type;type:varchar(30);not null;index:idx_ai_event_source"`           // 来源类型
	SourceKey      string        `gorm:"column:source_key;type:varchar(128);not null;index:idx_ai_event_source"`           // 来源唯一键
	PayloadJson    string        `gorm:"column:payload_json;type:json"`                                                    // 事件负载（JSON）
	DedupKey       string        `gorm:"column:dedup_key;type:varchar(160);not null;uniqueIndex:uniq_ai_event_dedup"`      // 去重键（幂等）
	PublishStatus  int8          `gorm:"column:publish_status;type:tinyint;not null;default:0;index:idx_ai_event_publish"` // 投递状态（见 IngestPublishStatus*）
	Status         int8          `gorm:"column:status;type:tinyint;not null;default:0;index:idx_ai_event_status"`          // 处理状态（见 IngestEventStatus*）
	RetryCount     int           `gorm:"column:retry_count;type:int;not null;default:0"`                                   // 重试次数
	NextRetryAt    sql.NullTime  `gorm:"column:next_retry_at;type:datetime;index:idx_ai_event_next_retry"`                 // 下次重试时间
	LastError      string        `gorm:"column:last_error;type:varchar(255)"`                                              // 最近错误
	KafkaTopic     string        `gorm:"column:kafka_topic;type:varchar(128)"`                                             // Kafka Topic
	KafkaPartition int           `gorm:"column:kafka_partition;type:int"`                                                  // Kafka 分区
	KafkaOffset    int64         `gorm:"column:kafka_offset;type:bigint"`                                                  // Kafka Offset
	PublishedAt    sql.NullTime  `gorm:"column:published_at;type:datetime"`                                                // 投递时间
	TraceId        string        `gorm:"column:trace_id;type:varchar(64)"`                                                 // 链路追踪 ID
	CreatedAt      time.Time     `gorm:"column:created_at;type:datetime;not null"`                                         // 创建时间
	UpdatedAt      time.Time     `gorm:"column:updated_at;type:datetime;not null"`                                         // 更新时间
}

// 代表“要被异步投递/处理的一条最小工作单元”（outbox 消息）
func (AIIngestEvent) TableName() string { return "ai_ingest_event" }

// AIBackfillJob 回填任务表（一次请求/一次全量或增量回填的聚合记录）
type AIBackfillJob struct {
	Id                 int64        `gorm:"column:id;primaryKey;autoIncrement"`                                             // 主键
	TenantUserId       string       `gorm:"column:tenant_user_id;type:char(20);not null;index:idx_ai_backfill_job_tenant"`  // 租户用户 ID
	Status             int8         `gorm:"column:status;type:tinyint;not null;default:0;index:idx_ai_backfill_job_status"` // 任务状态（见 BackfillJobStatus*）
	Since              sql.NullTime `gorm:"column:since;type:datetime"`                                                     // 起始时间（可空）
	Until              sql.NullTime `gorm:"column:until;type:datetime"`                                                     // 结束时间（可空）
	PageSize           int          `gorm:"column:page_size;type:int;not null;default:200"`                                 // 每页大小
	MaxSessions        int          `gorm:"column:max_sessions;type:int;not null;default:0"`                                // 最大会话数（0=不限制）
	MaxPagesPerSession int          `gorm:"column:max_pages_per_session;type:int;not null;default:0"`                       // 单会话最大页数（0=不限制）
	TotalEvents        int          `gorm:"column:total_events;type:int;not null;default:0"`                                // 事件总数
	PublishedEvents    int          `gorm:"column:published_events;type:int;not null;default:0"`                            // 已投递事件数
	SucceededEvents    int          `gorm:"column:succeeded_events;type:int;not null;default:0"`                            // 成功处理事件数
	FailedEvents       int          `gorm:"column:failed_events;type:int;not null;default:0"`                               // 失败事件数
	CreatedAt          time.Time    `gorm:"column:created_at;type:datetime;not null"`                                       // 创建时间
	UpdatedAt          time.Time    `gorm:"column:updated_at;type:datetime;not null"`                                       // 更新时间
}

// 代表“一次回填任务”的元数据/进度聚合（tenant、since/until、page_size、总事件数、成功失败统计、状态）。
func (AIBackfillJob) TableName() string { return "ai_backfill_job" }
