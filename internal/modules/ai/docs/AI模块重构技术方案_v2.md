# AI模块重构技术方案 v2.0

## 一、需求背景与目标

### 1.1 现状分析

**当前实现：**
- 用户可创建/选择Agent（global或private）
- Agent绑定知识库（全局RAG或私有知识库）
- 用户基于Agent创建会话进行聊天
- AI入口和IM入口分离（独立的路由和页面）

**存在问题：**
1. 与ai prd_new.md中"模块一：全局AI个人助手"的定位偏离
2. 缺少唯一的、系统级的助手会话（用于离线总结、主动通知等）
3. AI和IM模块分离，用户体验割裂
4. 未预留后续模块的扩展接口

### 1.2 改造目标

**核心功能：**
1. **系统级全局AI助手**：
   - 用户注册后自动创建全局Agent（系统级，owner_type=system）
   - 自动创建唯一的助手会话（不可删除、置顶、固定）
   - 用于：离线总结推送、主动通知、用户咨询等

2. **用户自定义Agent**：
   - 保留现有的用户创建Agent能力
   - 支持基于Agent创建多个会话（隔离上下文）

3. **前后端融合**：
   - 前端：取消独立AI页面，整合到IM主界面
   - Agent列表融入会话列表
   - 会话窗口统一展示（IM会话 + AI会话）

4. **扩展性设计**：
   - 为后续模块预留字段和接口（命令系统、MCP工具调用、动态上下文画布等）
   - 数据库设计支持未来的权限裁剪、多模态消息等

---

## 二、数据库设计改造

### 2.1 核心表结构调整

#### 2.1.1 `ai_agent` 表新增字段

**现有字段保持不变**，新增以下字段以支持扩展：

```sql
ALTER TABLE `ai_agent` 
ADD COLUMN `is_system_global` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否为系统全局助手（每用户唯一）' AFTER `owner_type`,
ADD COLUMN `capabilities_json` JSON NULL COMMENT '能力配置（MCP工具、命令权限等，预留）' AFTER `tools_json`,
ADD COLUMN `config_json` JSON NULL COMMENT '扩展配置（推理参数、安全策略等，预留）' AFTER `capabilities_json`,
ADD INDEX `idx_owner_system_global` (`owner_id`, `is_system_global`);
```

**字段说明：**
- `is_system_global`: 标识该Agent是否为系统级全局助手（每个用户只有一个，注册时自动创建）
- `capabilities_json`: 预留字段，用于配置Agent能力（如MCP工具列表、命令权限）
- `config_json`: 预留字段，存储扩展配置（如推理温度、安全过滤规则等）

**数据约束：**
- 每个用户（tenant_user_id）只能有一个 `is_system_global=1` 的Agent
- 后端在创建全局助手时需检查唯一性

#### 2.1.2 `ai_assistant_session` 表新增字段

```sql
ALTER TABLE `ai_assistant_session`
ADD COLUMN `session_type` VARCHAR(20) NOT NULL DEFAULT 'normal' COMMENT '会话类型：system_global=系统助手会话, normal=普通会话' AFTER `status`,
ADD COLUMN `is_pinned` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否置顶' AFTER `session_type`,
ADD COLUMN `is_deletable` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否可删除（系统助手会话不可删除）' AFTER `is_pinned`,
ADD COLUMN `context_config_json` JSON NULL COMMENT '上下文配置（检索范围、token限制等，预留）' AFTER `persona_id`,
ADD COLUMN `metadata_json` JSON NULL COMMENT '元数据（标签、分类、统计信息等，预留）' AFTER `context_config_json`,
ADD INDEX `idx_user_type_pinned` (`tenant_user_id`, `session_type`, `is_pinned`);
```

**字段说明：**
- `session_type`: 
  - `system_global`: 系统级助手会话（每用户唯一）
  - `normal`: 普通会话（用户基于Agent创建的会话）
- `is_pinned`: 是否置顶显示
- `is_deletable`: 是否可删除（系统助手会话强制为0）
- `context_config_json`: 预留字段，用于配置RAG检索范围、token限制等
- `metadata_json`: 预留字段，存储会话元数据（如标签、统计信息）

**数据约束：**
- 每个用户只能有一个 `session_type='system_global'` 的会话
- `session_type='system_global'` 的会话必须 `is_deletable=0` 且 `is_pinned=1`

#### 2.1.3 `ai_assistant_message` 表新增字段

```sql
ALTER TABLE `ai_assistant_message`
ADD COLUMN `metadata_json` JSON NULL COMMENT '消息元数据（推理耗时、模型信息、MCP调用记录等，预留）' AFTER `tokens_json`,
ADD COLUMN `render_type` VARCHAR(20) NULL COMMENT '渲染类型（text/card/widget，用于模块五动态UI，预留）' AFTER `metadata_json`,
ADD COLUMN `render_data_json` JSON NULL COMMENT '渲染数据（动态组件配置，预留）' AFTER `render_type`;
```

**字段说明：**
- `metadata_json`: 预留字段，存储推理元数据（模型版本、MCP工具调用记录等）
- `render_type`: 预留字段，用于模块五"动态上下文画布"（如投票卡片、地图标记）
- `render_data_json`: 预留字段，存储动态组件的数据

### 2.2 新增表：系统通知记录表（预留）

为支持"离线总结推送"、"主动通知"等功能，新增表：

```sql
CREATE TABLE `ai_system_notification` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键',
  `notification_id` CHAR(20) NOT NULL COMMENT '通知唯一ID',
  `tenant_user_id` CHAR(20) NOT NULL COMMENT '目标用户ID',
  `session_id` CHAR(20) NOT NULL COMMENT '关联的助手会话ID',
  `type` VARCHAR(30) NOT NULL COMMENT '通知类型：offline_summary/reminder/insight',
  `title` VARCHAR(100) NULL COMMENT '通知标题',
  `content` MEDIUMTEXT NULL COMMENT '通知内容',
  `trigger_source` VARCHAR(50) NULL COMMENT '触发来源（如cron_job/event_trigger）',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态：0=待推送, 1=已推送, 2=已读',
  `pushed_at` DATETIME NULL COMMENT '推送时间',
  `read_at` DATETIME NULL COMMENT '已读时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notification_id` (`notification_id`),
  KEY `idx_user_status` (`tenant_user_id`, `status`),
  KEY `idx_session` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI系统通知记录表（预留，用于离线总结/主动通知）';
```

**说明：**
- 本阶段不实现通知功能，仅创建表结构预留接口
- 未来实现时，离线总结/提醒等功能会写入此表，然后通过助手会话推送给用户

---

## 三、后端代码改造方案

### 3.1 改造目录结构概览

```
internal/modules/ai/
├── domain/
│   ├── agent/
│   │   └── entities.go                    # [修改] 新增字段和常量
│   ├── assistant/
│   │   └── entities.go                    # [修改] 新增字段和常量
│   └── notification/                      # [新建] 通知领域（预留）
│       └── entities.go                    # [新建] 系统通知实体
├── domain/repository/
│   ├── agent_repository.go                # [修改] 新增方法
│   ├── assistant_repository.go            # [修改] 新增方法
│   └── notification_repository.go         # [新建] 通知仓储接口（预留）
├── infrastructure/persistence/
│   ├── agent_repository_impl.go           # [修改] 实现新方法
│   ├── assistant_repository_impl.go       # [修改] 实现新方法
│   └── notification_repository_impl.go    # [新建] 通知仓储实现（预留）
├── application/service/
│   ├── assistant_service.go               # [修改] 新增全局助手创建逻辑
│   ├── user_lifecycle_service.go          # [新建] 用户生命周期服务
│   └── notification_service.go            # [新建] 通知服务（预留）
├── application/dto/
│   ├── request/
│   │   └── assistant_request.go           # [修改] 新增请求参数
│   └── respond/
│   │   └── assistant_respond.go           # [修改] 新增响应字段
├── interface/http/
│   └── assistant_handler.go               # [修改] 新增接口
└── interface/events/                      # [新建] 事件监听器（预留）
    └── user_registered_listener.go        # [新建] 用户注册事件监听器
```

### 3.2 详细改造步骤

---

#### **阶段一：数据库表结构升级**

**步骤 1.1：执行数据库迁移脚本**

在 `internal/modules/ai/migrations/` 目录下创建迁移脚本：

**文件：** `001_add_system_global_fields.sql`

```sql
-- ai_agent 表新增字段
ALTER TABLE `ai_agent` 
ADD COLUMN `is_system_global` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否为系统全局助手（每用户唯一）' AFTER `owner_type`,
ADD COLUMN `capabilities_json` JSON NULL COMMENT '能力配置（MCP工具、命令权限等，预留）' AFTER `tools_json`,
ADD COLUMN `config_json` JSON NULL COMMENT '扩展配置（推理参数、安全策略等，预留）' AFTER `capabilities_json`,
ADD INDEX `idx_owner_system_global` (`owner_id`, `is_system_global`);

-- ai_assistant_session 表新增字段
ALTER TABLE `ai_assistant_session`
ADD COLUMN `session_type` VARCHAR(20) NOT NULL DEFAULT 'normal' COMMENT '会话类型：system_global=系统助手会话, normal=普通会话' AFTER `status`,
ADD COLUMN `is_pinned` TINYINT(1) NOT NULL DEFAULT 0 COMMENT '是否置顶' AFTER `session_type`,
ADD COLUMN `is_deletable` TINYINT(1) NOT NULL DEFAULT 1 COMMENT '是否可删除（系统助手会话不可删除）' AFTER `is_pinned`,
ADD COLUMN `context_config_json` JSON NULL COMMENT '上下文配置（检索范围、token限制等，预留）' AFTER `persona_id`,
ADD COLUMN `metadata_json` JSON NULL COMMENT '元数据（标签、分类、统计信息等，预留）' AFTER `context_config_json`,
ADD INDEX `idx_user_type_pinned` (`tenant_user_id`, `session_type`, `is_pinned`);

-- ai_assistant_message 表新增字段
ALTER TABLE `ai_assistant_message`
ADD COLUMN `metadata_json` JSON NULL COMMENT '消息元数据（推理耗时、模型信息、MCP调用记录等，预留）' AFTER `tokens_json`,
ADD COLUMN `render_type` VARCHAR(20) NULL COMMENT '渲染类型（text/card/widget，用于模块五动态UI，预留）' AFTER `metadata_json`,
ADD COLUMN `render_data_json` JSON NULL COMMENT '渲染数据（动态组件配置，预留）' AFTER `render_type`;

-- 创建系统通知表（预留）
CREATE TABLE IF NOT EXISTS `ai_system_notification` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '主键',
  `notification_id` CHAR(20) NOT NULL COMMENT '通知唯一ID',
  `tenant_user_id` CHAR(20) NOT NULL COMMENT '目标用户ID',
  `session_id` CHAR(20) NOT NULL COMMENT '关联的助手会话ID',
  `type` VARCHAR(30) NOT NULL COMMENT '通知类型：offline_summary/reminder/insight',
  `title` VARCHAR(100) NULL COMMENT '通知标题',
  `content` MEDIUMTEXT NULL COMMENT '通知内容',
  `trigger_source` VARCHAR(50) NULL COMMENT '触发来源（如cron_job/event_trigger）',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态：0=待推送, 1=已推送, 2=已读',
  `pushed_at` DATETIME NULL COMMENT '推送时间',
  `read_at` DATETIME NULL COMMENT '已读时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_notification_id` (`notification_id`),
  KEY `idx_user_status` (`tenant_user_id`, `status`),
  KEY `idx_session` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI系统通知记录表（预留）';
```

**执行方式：**
- 手动执行SQL脚本，或集成到项目的迁移工具中
- 执行后验证表结构是否正确

---

#### **阶段二：领域实体层改造**

**步骤 2.1：修改 `domain/agent/entities.go`**

**改动内容：**

1. 新增常量定义：

```go
const (
	// 系统全局助手标识
	IsSystemGlobalTrue  int8 = 1
	IsSystemGlobalFalse int8 = 0
)
```

2. 在 `AIAgent` 结构体中新增字段：

```go
type AIAgent struct {
	// ... 现有字段保持不变 ...
	
	IsSystemGlobal    int8      `gorm:"column:is_system_global;type:tinyint;not null;default:0"` // 是否为系统全局助手
	CapabilitiesJson  string    `gorm:"column:capabilities_json;type:json"`                       // 能力配置（预留）
	ConfigJson        string    `gorm:"column:config_json;type:json"`                             // 扩展配置（预留）
	
	// ... 现有字段保持不变 ...
}
```

**AI开发Prompt：**

```
任务：修改 domain/agent/entities.go

1. 在常量定义区域新增：
   - IsSystemGlobalTrue  int8 = 1（表示系统全局助手）
   - IsSystemGlobalFalse int8 = 0（表示非系统全局助手）

2. 在 AIAgent 结构体中新增三个字段：
   - IsSystemGlobal    int8   `gorm:"column:is_system_global;type:tinyint;not null;default:0"`
   - CapabilitiesJson  string `gorm:"column:capabilities_json;type:json"`
   - ConfigJson        string `gorm:"column:config_json;type:json"`
   
3. 添加注释说明字段用途（CapabilitiesJson和ConfigJson为预留字段，用于未来扩展）

4. 不要修改现有字段和方法
```

---

**步骤 2.2：修改 `domain/assistant/entities.go`**

**改动内容：**

1. 新增常量定义：

```go
const (
	// 会话类型
	SessionTypeSystemGlobal = "system_global" // 系统级助手会话
	SessionTypeNormal       = "normal"         // 普通会话
	
	// 置顶状态
	IsPinnedTrue  int8 = 1
	IsPinnedFalse int8 = 0
	
	// 是否可删除
	IsDeletableTrue  int8 = 1
	IsDeletableFalse int8 = 0
)
```

2. 在 `AIAssistantSession` 结构体中新增字段：

```go
type AIAssistantSession struct {
	// ... 现有字段保持不变 ...
	
	SessionType       string    `gorm:"column:session_type;type:varchar(20);not null;default:'normal'"`  // 会话类型
	IsPinned          int8      `gorm:"column:is_pinned;type:tinyint;not null;default:0"`                 // 是否置顶
	IsDeletable       int8      `gorm:"column:is_deletable;type:tinyint;not null;default:1"`              // 是否可删除
	ContextConfigJson string    `gorm:"column:context_config_json;type:json"`                             // 上下文配置（预留）
	MetadataJson      string    `gorm:"column:metadata_json;type:json"`                                   // 元数据（预留）
	
	// ... 现有字段保持不变 ...
}
```

3. 在 `AIAssistantMessage` 结构体中新增字段：

```go
type AIAssistantMessage struct {
	// ... 现有字段保持不变 ...
	
	MetadataJson     string    `gorm:"column:metadata_json;type:json"`      // 消息元数据（预留）
	RenderType       string    `gorm:"column:render_type;type:varchar(20)"` // 渲染类型（预留，用于动态UI）
	RenderDataJson   string    `gorm:"column:render_data_json;type:json"`   // 渲染数据（预留）
	
	// ... 现有字段保持不变 ...
}
```

**AI开发Prompt：**

```
任务：修改 domain/assistant/entities.go

1. 在常量定义区域新增：
   - SessionTypeSystemGlobal = "system_global"（系统级助手会话）
   - SessionTypeNormal = "normal"（普通会话）
   - IsPinnedTrue/IsPinnedFalse（置顶状态）
   - IsDeletableTrue/IsDeletableFalse（是否可删除）

2. 在 AIAssistantSession 结构体中新增5个字段：
   - SessionType       string `gorm:"column:session_type;type:varchar(20);not null;default:'normal'"`
   - IsPinned          int8   `gorm:"column:is_pinned;type:tinyint;not null;default:0"`
   - IsDeletable       int8   `gorm:"column:is_deletable;type:tinyint;not null;default:1"`
   - ContextConfigJson string `gorm:"column:context_config_json;type:json"`
   - MetadataJson      string `gorm:"column:metadata_json;type:json"`

3. 在 AIAssistantMessage 结构体中新增3个字段：
   - MetadataJson     string `gorm:"column:metadata_json;type:json"`
   - RenderType       string `gorm:"column:render_type;type:varchar(20)"`
   - RenderDataJson   string `gorm:"column:render_data_json;type:json"`

4. 为预留字段添加注释，说明用途（如"预留，用于模块五动态UI"）

5. 不要修改现有字段和方法
```

---

**步骤 2.3：新建 `domain/notification/entities.go`（预留）**

**文件路径：** `internal/modules/ai/domain/notification/entities.go`

**内容：**

```go
package notification

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
	Id             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	NotificationId string    `gorm:"column:notification_id;type:char(20);uniqueIndex;not null"`
	TenantUserId   string    `gorm:"column:tenant_user_id;type:char(20);index;not null"`
	SessionId      string    `gorm:"column:session_id;type:char(20);index;not null"`
	Type           string    `gorm:"column:type;type:varchar(30);not null"`
	Title          string    `gorm:"column:title;type:varchar(100)"`
	Content        string    `gorm:"column:content;type:mediumtext"`
	TriggerSource  string    `gorm:"column:trigger_source;type:varchar(50)"`
	Status         int8      `gorm:"column:status;type:tinyint;not null;default:0"`
	PushedAt       *time.Time `gorm:"column:pushed_at;type:datetime"`
	ReadAt         *time.Time `gorm:"column:read_at;type:datetime"`
	CreatedAt      time.Time  `gorm:"column:created_at;type:datetime;not null"`
}

func (AISystemNotification) TableName() string {
	return "ai_system_notification"
}
```

**AI开发Prompt：**

```
任务：新建文件 internal/modules/ai/domain/notification/entities.go

1. 创建 notification 包，定义 AISystemNotification 实体结构体

2. 定义常量：
   - 通知类型：TypeOfflineSummary, TypeReminder, TypeInsight
   - 通知状态：StatusPending, StatusPushed, StatusRead

3. 定义 AISystemNotification 结构体，包含以下字段：
   - Id, NotificationId, TenantUserId, SessionId, Type, Title, Content
   - TriggerSource, Status, PushedAt, ReadAt, CreatedAt
   - 使用 gorm 标签定义字段映射

4. 实现 TableName() 方法返回 "ai_system_notification"

5. 在文件顶部添加注释：
   // Package notification 系统通知领域（预留，用于模块一的离线总结和主动通知功能）
   // 本阶段仅定义实体，业务逻辑在后续实现
```

---

#### **阶段三：仓储层改造**

**步骤 3.1：修改 `domain/repository/agent_repository.go`**

**改动内容：**

在 `AgentRepository` 接口中新增方法：

```go
type AgentRepository interface {
	// ... 现有方法保持不变 ...
	
	// GetSystemGlobalAgent 获取用户的系统全局助手Agent
	GetSystemGlobalAgent(ctx context.Context, tenantUserID string) (*agent.AIAgent, error)
	
	// CreateSystemGlobalAgent 创建系统全局助手Agent（仅内部调用，带唯一性检查）
	CreateSystemGlobalAgent(ctx context.Context, ag *agent.AIAgent) error
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/domain/repository/agent_repository.go

1. 在 AgentRepository 接口中新增两个方法：
   
   // GetSystemGlobalAgent 获取用户的系统全局助手Agent
   GetSystemGlobalAgent(ctx context.Context, tenantUserID string) (*agent.AIAgent, error)
   
   // CreateSystemGlobalAgent 创建系统全局助手Agent（带唯一性检查，防止重复创建）
   CreateSystemGlobalAgent(ctx context.Context, ag *agent.AIAgent) error

2. 不要修改现有方法签名

3. 添加注释说明方法用途
```

---

**步骤 3.2：修改 `infrastructure/persistence/agent_repository_impl.go`**

**改动内容：**

实现新增的仓储方法：

```go
func (r *agentRepositoryImpl) GetSystemGlobalAgent(ctx context.Context, tenantUserID string) (*agent.AIAgent, error) {
	var ag agent.AIAgent
	err := r.db.WithContext(ctx).
		Where("owner_id = ? AND is_system_global = ?", tenantUserID, agent.IsSystemGlobalTrue).
		First(&ag).Error
	
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // 未找到返回nil，不报错
		}
		return nil, err
	}
	return &ag, nil
}

func (r *agentRepositoryImpl) CreateSystemGlobalAgent(ctx context.Context, ag *agent.AIAgent) error {
	// 检查该用户是否已有系统全局助手
	existing, err := r.GetSystemGlobalAgent(ctx, ag.OwnerId)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("user already has a system global agent")
	}
	
	// 强制设置关键字段
	ag.IsSystemGlobal = agent.IsSystemGlobalTrue
	ag.OwnerType = agent.OwnerTypeUser // 注意：虽然是系统级，但归属仍为用户
	ag.Status = agent.AgentStatusEnabled
	
	return r.db.WithContext(ctx).Create(ag).Error
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/infrastructure/persistence/agent_repository_impl.go

1. 实现 GetSystemGlobalAgent 方法：
   - 查询条件：owner_id = 参数tenantUserID 且 is_system_global = 1
   - 使用 First() 查询
   - 如果未找到（gorm.ErrRecordNotFound），返回 (nil, nil) 而非错误
   - 其他错误正常返回

2. 实现 CreateSystemGlobalAgent 方法：
   - 先调用 GetSystemGlobalAgent 检查是否已存在
   - 如果已存在，返回错误 "user already has a system global agent"
   - 强制设置 ag.IsSystemGlobal = 1, ag.OwnerType = "user", ag.Status = 1
   - 调用 db.Create(ag) 插入数据库

3. 导入必要的包：fmt, errors, gorm.io/gorm

4. 不要修改现有方法
```

---

**步骤 3.3：修改 `domain/repository/assistant_repository.go`**

**改动内容：**

在 `AssistantSessionRepository` 接口中新增方法：

```go
type AssistantSessionRepository interface {
	// ... 现有方法保持不变 ...
	
	// GetSystemGlobalSession 获取用户的系统级助手会话
	GetSystemGlobalSession(ctx context.Context, tenantUserID string) (*assistant.AIAssistantSession, error)
	
	// CreateSystemGlobalSession 创建系统级助手会话（带唯一性检查）
	CreateSystemGlobalSession(ctx context.Context, session *assistant.AIAssistantSession) error
	
	// ListSessionsWithType 获取会话列表（支持按类型过滤、置顶排序）
	ListSessionsWithType(ctx context.Context, tenantUserID string, sessionType string, limit, offset int) ([]*assistant.AIAssistantSession, error)
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/domain/repository/assistant_repository.go

1. 在 AssistantSessionRepository 接口中新增三个方法：
   
   // GetSystemGlobalSession 获取用户的系统级助手会话
   GetSystemGlobalSession(ctx context.Context, tenantUserID string) (*assistant.AIAssistantSession, error)
   
   // CreateSystemGlobalSession 创建系统级助手会话（带唯一性检查，防止重复创建）
   CreateSystemGlobalSession(ctx context.Context, session *assistant.AIAssistantSession) error
   
   // ListSessionsWithType 获取会话列表（支持按类型过滤、置顶排序）
   // sessionType为空字符串表示不过滤类型，结果按is_pinned DESC, updated_at DESC排序
   ListSessionsWithType(ctx context.Context, tenantUserID string, sessionType string, limit, offset int) ([]*assistant.AIAssistantSession, error)

2. 不要修改现有方法签名

3. 添加清晰的注释说明方法用途和参数含义
```

---

**步骤 3.4：修改 `infrastructure/persistence/assistant_repository_impl.go`**

**改动内容：**

实现新增的仓储方法：

```go
func (r *assistantSessionRepositoryImpl) GetSystemGlobalSession(ctx context.Context, tenantUserID string) (*assistant.AIAssistantSession, error) {
	var session assistant.AIAssistantSession
	err := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND session_type = ?", tenantUserID, assistant.SessionTypeSystemGlobal).
		First(&session).Error
	
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &session, nil
}

func (r *assistantSessionRepositoryImpl) CreateSystemGlobalSession(ctx context.Context, session *assistant.AIAssistantSession) error {
	// 检查该用户是否已有系统助手会话
	existing, err := r.GetSystemGlobalSession(ctx, session.TenantUserId)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("user already has a system global session")
	}
	
	// 强制设置关键字段
	session.SessionType = assistant.SessionTypeSystemGlobal
	session.IsPinned = assistant.IsPinnedTrue
	session.IsDeletable = assistant.IsDeletableFalse
	session.Status = assistant.SessionStatusActive
	
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *assistantSessionRepositoryImpl) ListSessionsWithType(ctx context.Context, tenantUserID string, sessionType string, limit, offset int) ([]*assistant.AIAssistantSession, error) {
	var sessions []*assistant.AIAssistantSession
	
	query := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND status = ?", tenantUserID, assistant.SessionStatusActive)
	
	// 如果指定了类型，则过滤
	if sessionType != "" {
		query = query.Where("session_type = ?", sessionType)
	}
	
	// 按置顶和更新时间排序
	query = query.Order("is_pinned DESC, updated_at DESC")
	
	if limit > 0 {
		query = query.Limit(limit).Offset(offset)
	}
	
	err := query.Find(&sessions).Error
	return sessions, err
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/infrastructure/persistence/assistant_repository_impl.go

1. 实现 GetSystemGlobalSession 方法：
   - 查询条件：tenant_user_id = 参数 且 session_type = 'system_global'
   - 如果未找到返回 (nil, nil)，其他错误正常返回

2. 实现 CreateSystemGlobalSession 方法：
   - 先调用 GetSystemGlobalSession 检查是否已存在
   - 如果已存在，返回错误 "user already has a system global session"
   - 强制设置：session_type='system_global', is_pinned=1, is_deletable=0, status=1
   - 调用 db.Create(session) 插入

3. 实现 ListSessionsWithType 方法：
   - 查询条件：tenant_user_id = 参数 且 status = 1（活跃）
   - 如果 sessionType 非空，添加条件 session_type = sessionType
   - 排序：ORDER BY is_pinned DESC, updated_at DESC（置顶在前，最新在前）
   - 支持分页：Limit(limit).Offset(offset)

4. 导入必要的包：fmt, errors, gorm.io/gorm

5. 不要修改现有方法
```

---

**步骤 3.5：新建 `domain/repository/notification_repository.go`（预留）**

**文件路径：** `internal/modules/ai/domain/repository/notification_repository.go`

**内容：**

```go
package repository

import (
	"context"
	"OmniLink/internal/modules/ai/domain/notification"
)

// NotificationRepository 系统通知仓储接口（预留，暂不实现业务逻辑）
type NotificationRepository interface {
	// CreateNotification 创建通知
	CreateNotification(ctx context.Context, notif *notification.AISystemNotification) error
	
	// GetPendingNotifications 获取待推送的通知列表
	GetPendingNotifications(ctx context.Context, tenantUserID string, limit int) ([]*notification.AISystemNotification, error)
	
	// UpdateNotificationStatus 更新通知状态
	UpdateNotificationStatus(ctx context.Context, notificationID string, status int8) error
}
```

**AI开发Prompt：**

```
任务：新建文件 internal/modules/ai/domain/repository/notification_repository.go

1. 定义 NotificationRepository 接口，包含三个方法：
   - CreateNotification：创建通知记录
   - GetPendingNotifications：获取指定用户的待推送通知（status=0）
   - UpdateNotificationStatus：更新通知状态（如标记为已推送、已读）

2. 在文件顶部添加注释：
   // 系统通知仓储接口（预留，用于模块一的离线总结和主动通知功能）
   // 本阶段仅定义接口，具体业务逻辑在后续阶段实现

3. 不需要实现具体方法，仅定义接口
```

---

**步骤 3.6：新建 `infrastructure/persistence/notification_repository_impl.go`（预留）**

**文件路径：** `internal/modules/ai/infrastructure/persistence/notification_repository_impl.go`

**内容：**

```go
package persistence

import (
	"context"
	"OmniLink/internal/modules/ai/domain/notification"
	"OmniLink/internal/modules/ai/domain/repository"
	"gorm.io/gorm"
)

type notificationRepositoryImpl struct {
	db *gorm.DB
}

// NewNotificationRepository 创建通知仓储实现（预留）
func NewNotificationRepository(db *gorm.DB) repository.NotificationRepository {
	return &notificationRepositoryImpl{db: db}
}

func (r *notificationRepositoryImpl) CreateNotification(ctx context.Context, notif *notification.AISystemNotification) error {
	return r.db.WithContext(ctx).Create(notif).Error
}

func (r *notificationRepositoryImpl) GetPendingNotifications(ctx context.Context, tenantUserID string, limit int) ([]*notification.AISystemNotification, error) {
	var notifs []*notification.AISystemNotification
	err := r.db.WithContext(ctx).
		Where("tenant_user_id = ? AND status = ?", tenantUserID, notification.StatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&notifs).Error
	return notifs, err
}

func (r *notificationRepositoryImpl) UpdateNotificationStatus(ctx context.Context, notificationID string, status int8) error {
	return r.db.WithContext(ctx).
		Model(&notification.AISystemNotification{}).
		Where("notification_id = ?", notificationID).
		Update("status", status).Error
}
```

**AI开发Prompt：**

```
任务：新建文件 internal/modules/ai/infrastructure/persistence/notification_repository_impl.go

1. 实现 NotificationRepository 接口的所有方法：
   - CreateNotification：使用 db.Create() 插入记录
   - GetPendingNotifications：查询 status=0 的通知，按创建时间升序，支持limit
   - UpdateNotificationStatus：更新指定notification_id的status字段

2. 定义 notificationRepositoryImpl 结构体，包含 *gorm.DB 字段

3. 实现构造函数 NewNotificationRepository(db *gorm.DB) repository.NotificationRepository

4. 在文件顶部添加注释：
   // 系统通知仓储实现（预留，本阶段仅提供基础CRUD，业务逻辑在后续实现）

5. 导入必要的包
```

---

#### **阶段四：应用服务层改造**

**步骤 4.1：新建 `application/service/user_lifecycle_service.go`**

**文件路径：** `internal/modules/ai/application/service/user_lifecycle_service.go`

**功能说明：**
- 封装用户生命周期相关的AI初始化逻辑
- 提供"用户注册后自动创建全局助手+会话"的方法
- 供用户模块或事件监听器调用

**内容：**

```go
package service

import (
	"context"
	"fmt"
	"time"

	"OmniLink/internal/modules/ai/domain/agent"
	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/util"
)

// UserLifecycleService 用户生命周期服务（处理用户注册、注销等AI相关初始化）
type UserLifecycleService interface {
	// InitializeUserAIAssistant 用户注册后初始化AI助手（创建全局Agent和系统会话）
	InitializeUserAIAssistant(ctx context.Context, tenantUserID string) error
}

type userLifecycleServiceImpl struct {
	agentRepo   repository.AgentRepository
	sessionRepo repository.AssistantSessionRepository
	ragRepo     repository.RAGRepository
}

// NewUserLifecycleService 创建用户生命周期服务
func NewUserLifecycleService(
	agentRepo repository.AgentRepository,
	sessionRepo repository.AssistantSessionRepository,
	ragRepo repository.RAGRepository,
) UserLifecycleService {
	return &userLifecycleServiceImpl{
		agentRepo:   agentRepo,
		sessionRepo: sessionRepo,
		ragRepo:     ragRepo,
	}
}

func (s *userLifecycleServiceImpl) InitializeUserAIAssistant(ctx context.Context, tenantUserID string) error {
	// 1. 检查是否已初始化（幂等性保证）
	existingAgent, err := s.agentRepo.GetSystemGlobalAgent(ctx, tenantUserID)
	if err != nil {
		return fmt.Errorf("failed to check existing agent: %w", err)
	}
	if existingAgent != nil {
		// 已存在，直接返回（幂等）
		return nil
	}

	// 2. 创建全局知识库（如果不存在）
	kb := &rag.AIKnowledgeBase{
		OwnerType: "user", // 归属用户
		OwnerId:   tenantUserID,
		KBType:    agent.KBTypeGlobal,
		Name:      "Global Knowledge Base",
		Status:    rag.CommonStatusEnabled,
	}
	kbID, err := s.ragRepo.EnsureKnowledgeBase(ctx, kb)
	if err != nil {
		return fmt.Errorf("failed to ensure knowledge base: %w", err)
	}

	// 3. 创建系统全局AI助手Agent
	systemPrompt := `### 基础身份
你是由 OmniLink 构建的全局 AI 个人助手。你的核心目标是辅助用户管理社交关系、处理消息并提供智能问答。

### 核心能力与约束
1. **数据严谨性**：
   - 对于用户的私有数据（好友列表、群组信息、聊天记录），**必须** 通过工具调用（Tools）或检索增强生成（RAG）获取，**严禁** 臆造。
   - 若工具或检索未返回结果，请明确告知用户"未找到相关信息"，不要编造假数据。

2. **工具使用策略**：
   - 当用户询问"我有没有好友X"、"发消息给Y"、"最近群里聊了什么"等实时操作类问题时，**优先** 尝试调用对应的 MCP 工具。
   - 若无可用工具，请向用户解释当前能力受限。

3. **回答风格**：
   - 简洁、专业、友好。
   - 涉及敏感隐私（如手机号、详细地址）时，请进行脱敏处理或再次确认。

### 知识库范围
你拥有全局知识库的访问权限，可以回答关于 OmniLink 平台功能、通用百科等问题。

### 扩展能力（预留）
未来你将支持：
- 离线总结：用户登录时自动推送离线期间的重点消息摘要
- 主动通知：定时提醒、日报推送等
- 智能指令：通过 /todo、/remind 等快捷命令快速执行任务`

	newAgent := &agent.AIAgent{
		AgentId:         util.GenerateID("AG"),
		OwnerType:       agent.OwnerTypeUser,
		OwnerId:         tenantUserID,
		Name:            "全局AI助手",
		Description:     "您的专属智能助理，负责消息管理、智能问答和主动通知",
		PersonaPrompt:   "", // 系统助手无需用户自定义人设
		SystemPrompt:    systemPrompt,
		Status:          agent.AgentStatusEnabled,
		KBType:          agent.KBTypeGlobal,
		KBId:            kbID,
		ToolsJson:       "[]", // 预留，后续配置MCP工具
		IsSystemGlobal:  agent.IsSystemGlobalTrue,
		CapabilitiesJson: "", // 预留
		ConfigJson:      "", // 预留
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.agentRepo.CreateSystemGlobalAgent(ctx, newAgent); err != nil {
		return fmt.Errorf("failed to create system global agent: %w", err)
	}

	// 4. 创建系统级助手会话
	newSession := &assistant.AIAssistantSession{
		SessionId:         util.GenerateID("AS"),
		TenantUserId:      tenantUserID,
		Title:             "🤖 AI助手",
		Status:            assistant.SessionStatusActive,
		AgentId:           newAgent.AgentId,
		SessionType:       assistant.SessionTypeSystemGlobal,
		IsPinned:          assistant.IsPinnedTrue,
		IsDeletable:       assistant.IsDeletableFalse,
		ContextConfigJson: "", // 预留
		MetadataJson:      "", // 预留
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := s.sessionRepo.CreateSystemGlobalSession(ctx, newSession); err != nil {
		return fmt.Errorf("failed to create system global session: %w", err)
	}

	return nil
}
```

**AI开发Prompt：**

```
任务：新建文件 internal/modules/ai/application/service/user_lifecycle_service.go

1. 定义 UserLifecycleService 接口，包含方法：
   - InitializeUserAIAssistant(ctx context.Context, tenantUserID string) error
   
2. 实现 userLifecycleServiceImpl 结构体，依赖三个仓储：
   - agentRepo, sessionRepo, ragRepo

3. 实现 InitializeUserAIAssistant 方法，逻辑如下：
   a. 调用 agentRepo.GetSystemGlobalAgent() 检查是否已初始化，如已存在则直接返回（幂等）
   b. 调用 ragRepo.EnsureKnowledgeBase() 创建全局知识库
   c. 创建系统全局助手Agent，设置：
      - Name="全局AI助手"
      - Description="您的专属智能助理，负责消息管理、智能问答和主动通知"
      - IsSystemGlobal=1
      - SystemPrompt 使用我提供的多行字符串（包含能力说明和扩展预留）
   d. 调用 agentRepo.CreateSystemGlobalAgent() 创建Agent
   e. 创建系统级助手会话，设置：
      - Title="🤖 AI助手"
      - SessionType="system_global"
      - IsPinned=1, IsDeletable=0
   f. 调用 sessionRepo.CreateSystemGlobalSession() 创建会话

4. 实现构造函数 NewUserLifecycleService

5. 添加详细的注释和错误处理（每步失败返回带上下文的错误）

6. 导入必要的包：context, fmt, time, util
```

---

**步骤 4.2：修改 `application/service/assistant_service.go`**

**改动内容：**

1. 在 `AssistantService` 接口中新增方法：

```go
type AssistantService interface {
	// ... 现有方法保持不变 ...
	
	// GetOrCreateSystemSession 获取或创建系统助手会话（幂等）
	GetOrCreateSystemSession(ctx context.Context, tenantUserID string) (*respond.SystemSessionRespond, error)
}
```

2. 在 `assistantServiceImpl` 中实现该方法：

```go
func (s *assistantServiceImpl) GetOrCreateSystemSession(ctx context.Context, tenantUserID string) (*respond.SystemSessionRespond, error) {
	// 1. 尝试获取已有的系统会话
	session, err := s.sessionRepo.GetSystemGlobalSession(ctx, tenantUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get system session: %w", err)
	}
	
	if session != nil {
		// 已存在，直接返回
		return &respond.SystemSessionRespond{
			SessionID: session.SessionId,
			AgentID:   session.AgentId,
			Title:     session.Title,
		}, nil
	}
	
	// 2. 不存在，触发初始化（调用 UserLifecycleService）
	// 注意：这里需要注入 UserLifecycleService 依赖
	// 为避免循环依赖，可以在构造时传入，或者直接在这里重复逻辑（简化方案）
	
	// 简化方案：直接返回错误，提示需要先初始化
	return nil, fmt.Errorf("system session not found, please initialize user AI assistant first")
}
```

**注意：** 为避免循环依赖，这里采用"延迟初始化"策略：
- 用户注册时由用户模块主动调用 `UserLifecycleService.InitializeUserAIAssistant()`
- 本方法仅作为兜底检查，如果未初始化则返回错误

3. 修改 `ListSessionsWithType` 方法（新增）：

在 `AssistantService` 接口中新增：

```go
// ListSessionsWithFilter 获取会话列表（支持类型过滤）
ListSessionsWithFilter(ctx context.Context, tenantUserID string, sessionType string, limit, offset int) (*respond.AssistantSessionListRespond, error)
```

实现：

```go
func (s *assistantServiceImpl) ListSessionsWithFilter(ctx context.Context, tenantUserID string, sessionType string, limit, offset int) (*respond.AssistantSessionListRespond, error) {
	sessions, err := s.sessionRepo.ListSessionsWithType(ctx, tenantUserID, sessionType, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]*respond.AssistantSessionItem, 0, len(sessions))
	for _, sess := range sessions {
		lastMessage := ""
		summary := ""
		if s.messageRepo != nil {
			msgs, err := s.messageRepo.ListRecentMessages(ctx, sess.SessionId, 1)
			if err == nil && len(msgs) > 0 {
				lastMessage = msgs[0].Content
				summary = truncateSummary(lastMessage, 80)
			}
		}
		items = append(items, &respond.AssistantSessionItem{
			SessionID:   sess.SessionId,
			Title:       sess.Title,
			AgentID:     sess.AgentId,
			SessionType: sess.SessionType,    // 新增字段
			IsPinned:    sess.IsPinned == 1,  // 新增字段
			IsDeletable: sess.IsDeletable == 1, // 新增字段
			UpdatedAt:   sess.UpdatedAt,
			LastMessage: lastMessage,
			Summary:     summary,
		})
	}

	return &respond.AssistantSessionListRespond{
		Sessions: items,
		Total:    len(items),
	}, nil
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/application/service/assistant_service.go

1. 在 AssistantService 接口中新增两个方法：
   - GetOrCreateSystemSession(ctx, tenantUserID) (*respond.SystemSessionRespond, error)
   - ListSessionsWithFilter(ctx, tenantUserID, sessionType, limit, offset) (*respond.AssistantSessionListRespond, error)

2. 实现 GetOrCreateSystemSession：
   - 调用 sessionRepo.GetSystemGlobalSession() 查询系统会话
   - 如果存在，返回 SystemSessionRespond{SessionID, AgentID, Title}
   - 如果不存在，返回错误 "system session not found, please initialize user AI assistant first"

3. 实现 ListSessionsWithFilter：
   - 调用 sessionRepo.ListSessionsWithType() 获取会话列表
   - 返回结果中新增字段：SessionType, IsPinned, IsDeletable
   - 复用现有的 truncateSummary 和消息查询逻辑

4. 不要修改现有方法

5. 导入必要的包
```

---

**步骤 4.3：修改 DTO 响应结构**

**文件：** `internal/modules/ai/application/dto/respond/assistant_respond.go`

**改动内容：**

1. 新增响应结构体：

```go
// SystemSessionRespond 系统助手会话响应
type SystemSessionRespond struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
	Title     string `json:"title"`
}
```

2. 修改 `AssistantSessionItem` 结构体，新增字段：

```go
type AssistantSessionItem struct {
	// ... 现有字段保持不变 ...
	
	SessionType string `json:"session_type"` // 会话类型
	IsPinned    bool   `json:"is_pinned"`    // 是否置顶
	IsDeletable bool   `json:"is_deletable"` // 是否可删除
	
	// ... 现有字段保持不变 ...
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/application/dto/respond/assistant_respond.go

1. 新增结构体 SystemSessionRespond：
   - SessionID string `json:"session_id"`
   - AgentID   string `json:"agent_id"`
   - Title     string `json:"title"`

2. 在 AssistantSessionItem 结构体中新增三个字段：
   - SessionType string `json:"session_type"` // 会话类型
   - IsPinned    bool   `json:"is_pinned"`    // 是否置顶
   - IsDeletable bool   `json:"is_deletable"` // 是否可删除

3. 不要修改现有字段

4. 添加注释说明新字段用途
```

---

#### **阶段五：HTTP接口层改造**

**步骤 5.1：修改 `interface/http/assistant_handler.go`**

**改动内容：**

1. 新增接口：获取系统助手会话

```go
// GetSystemSession 获取系统助手会话
//
// 路由: GET /ai/assistant/system-session
// 鉴权: 需要JWT
// 响应体: SystemSessionRespond
func (h *AssistantHandler) GetSystemSession(c *gin.Context) {
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	data, err := h.svc.GetOrCreateSystemSession(c.Request.Context(), uuid)
	if err != nil {
		zlog.Error("get system session failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}
```

2. 修改 `ListSessions` 接口，支持类型过滤：

```go
// ListSessions 获取AI助手会话列表（支持类型过滤）
//
// 路由: GET /ai/assistant/sessions
// 鉴权: 需要JWT
// 查询参数: limit, offset, type (可选，值为 system_global 或 normal)
// 响应体: AssistantSessionListRespond
func (h *AssistantHandler) ListSessions(c *gin.Context) {
	uuid := strings.TrimSpace(c.GetString("uuid"))
	if uuid == "" {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}

	// 解析查询参数
	limit := 20
	offset := 0
	sessionType := strings.TrimSpace(c.Query("type")) // 新增参数
	
	if l, ok := c.GetQuery("limit"); ok {
		if n, err := parsePositiveInt(l); err == nil && n > 0 {
			limit = n
		}
	}
	if o, ok := c.GetQuery("offset"); ok {
		if n, err := parsePositiveInt(o); err == nil && n >= 0 {
			offset = n
		}
	}

	// 调用Service（改为调用新方法）
	data, err := h.svc.ListSessionsWithFilter(c.Request.Context(), uuid, sessionType, limit, offset)
	if err != nil {
		zlog.Error("assistant list sessions failed", zap.Error(err), zap.String("uuid", uuid))
	}
	back.Result(c, data, err)
}
```

**AI开发Prompt：**

```
任务：修改 internal/modules/ai/interface/http/assistant_handler.go

1. 新增方法 GetSystemSession：
   - 路由处理：GET /ai/assistant/system-session
   - 从JWT提取uuid
   - 调用 h.svc.GetOrCreateSystemSession(ctx, uuid)
   - 返回结果或错误

2. 修改 ListSessions 方法：
   - 新增查询参数解析：sessionType := c.Query("type")
   - 将调用从 h.svc.ListSessions() 改为 h.svc.ListSessionsWithFilter()
   - 传入 sessionType 参数

3. 不要修改现有的其他方法

4. 添加清晰的注释说明接口用途和参数

5. 导入必要的包
```

---

**步骤 5.2：注册新路由**

**文件位置：** 项目的路由注册文件（通常在 `internal/router` 或主入口）

**改动内容：**

在AI模块路由组中新增：

```go
// AI Assistant Routes
aiGroup := authed.Group("/ai/assistant")
{
	aiGroup.GET("/system-session", assistantHandler.GetSystemSession)  // 新增
	aiGroup.GET("/sessions", assistantHandler.ListSessions)            // 已有，支持type参数
	aiGroup.GET("/agents", assistantHandler.ListAgents)
	aiGroup.GET("/sessions/:session_id/messages", assistantHandler.GetSessionMessages)
	aiGroup.POST("/chat", assistantHandler.Chat)
	aiGroup.POST("/chat/stream", assistantHandler.ChatStream)
	aiGroup.POST("/agents", assistantHandler.CreateAgent)
	aiGroup.POST("/sessions", assistantHandler.CreateSession)
}
```

**AI开发Prompt：**

```
任务：在项目的路由注册文件中新增AI接口路由

1. 找到AI模块的路由组（通常在 /ai/assistant 前缀下）

2. 新增路由：
   GET /ai/assistant/system-session → assistantHandler.GetSystemSession

3. 确保该路由在鉴权中间件保护下（authed.Group）

4. 不要修改现有路由
```

---

#### **阶段六：用户注册钩子集成**

**步骤 6.1：在用户模块中调用AI初始化**

**文件位置：** `internal/modules/user/application/service/user_info_service.go`（或注册逻辑所在文件）

**改动内容：**

在用户注册成功后，调用AI模块的初始化服务：

```go
// 伪代码示例（需根据实际项目结构调整）
func (s *userInfoServiceImpl) Register(ctx context.Context, req RegisterRequest) error {
	// ... 现有注册逻辑 ...
	
	// 插入用户数据到数据库
	newUser := &entity.UserInfo{
		Uuid:     util.GenerateID("U"),
		Username: req.Username,
		// ... 其他字段 ...
	}
	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return err
	}
	
	// 【新增】调用AI模块初始化全局助手
	aiLifecycleService := GetAILifecycleService() // 从DI容器获取
	if err := aiLifecycleService.InitializeUserAIAssistant(ctx, newUser.Uuid); err != nil {
		// 失败仅记录日志，不阻断注册流程（降级处理）
		zlog.Error("failed to initialize user AI assistant", zap.Error(err), zap.String("uuid", newUser.Uuid))
	}
	
	return nil
}
```

**注意：**
- 需要在用户模块的依赖注入中引入 `UserLifecycleService`
- 建议异步处理初始化（如通过消息队列），避免阻塞注册流程
- 初始化失败不应导致注册失败（降级处理）

**AI开发Prompt：**

```
任务：在用户注册逻辑中集成AI助手初始化

1. 找到用户注册成功后的代码位置（通常在 user_info_service.go 的 Register 方法）

2. 在用户数据插入数据库成功后，新增调用：
   - 从依赖注入容器获取 UserLifecycleService 实例
   - 调用 aiLifecycleService.InitializeUserAIAssistant(ctx, newUser.Uuid)
   - 如果失败，仅记录错误日志，不阻断注册流程

3. 修改用户模块的依赖注入配置，注入 UserLifecycleService

4. 添加注释说明此处为AI模块集成点

5. 不要修改现有注册逻辑的核心流程
```

---

**步骤 6.2：（可选）实现事件驱动初始化**

**文件位置：** `internal/modules/ai/interface/events/user_registered_listener.go`（新建）

**说明：**
- 如果项目已有事件总线机制，可以通过事件监听器异步处理
- 用户模块在注册成功后发布 `UserRegistered` 事件
- AI模块监听该事件并执行初始化

**内容：**

```go
package events

import (
	"context"
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/zlog"
	"go.uber.org/zap"
)

// UserRegisteredEvent 用户注册事件
type UserRegisteredEvent struct {
	TenantUserID string
}

// UserRegisteredListener 用户注册事件监听器
type UserRegisteredListener struct {
	lifecycleService service.UserLifecycleService
}

func NewUserRegisteredListener(lifecycleService service.UserLifecycleService) *UserRegisteredListener {
	return &UserRegisteredListener{
		lifecycleService: lifecycleService,
	}
}

// Handle 处理用户注册事件
func (l *UserRegisteredListener) Handle(ctx context.Context, event UserRegisteredEvent) error {
	zlog.Info("handling user registered event", zap.String("tenant_user_id", event.TenantUserID))
	
	if err := l.lifecycleService.InitializeUserAIAssistant(ctx, event.TenantUserID); err != nil {
		zlog.Error("failed to initialize user AI assistant", zap.Error(err), zap.String("tenant_user_id", event.TenantUserID))
		return err
	}
	
	zlog.Info("user AI assistant initialized successfully", zap.String("tenant_user_id", event.TenantUserID))
	return nil
}
```

**AI开发Prompt：**

```
任务：（可选）新建事件监听器 internal/modules/ai/interface/events/user_registered_listener.go

1. 定义 UserRegisteredEvent 结构体，包含字段 TenantUserID

2. 定义 UserRegisteredListener 结构体，依赖 UserLifecycleService

3. 实现 Handle 方法：
   - 接收 UserRegisteredEvent 事件
   - 调用 lifecycleService.InitializeUserAIAssistant()
   - 记录日志（开始、成功、失败）

4. 实现构造函数 NewUserRegisteredListener

5. 在项目的事件总线中注册该监听器（需根据项目实际事件机制调整）

6. 仅在项目已有事件总线机制时实现，否则跳过
```

---

#### **阶段七：依赖注入配置**

**步骤 7.1：更新DI容器配置**

**文件位置：** 项目的依赖注入配置文件（如 `wire.go` 或 `provider.go`）

**改动内容：**

添加新服务的Provider：

```go
// AI模块的Provider Set
var AIProviderSet = wire.NewSet(
	// 现有Provider保持不变...
	
	// 新增
	persistence.NewNotificationRepository,      // 通知仓储（预留）
	service.NewUserLifecycleService,            // 用户生命周期服务
	service.NewNotificationService,             // 通知服务（预留）
)
```

**AI开发Prompt：**

```
任务：更新项目的依赖注入配置

1. 找到AI模块的Provider配置文件（通常是 wire.go 或类似文件）

2. 新增以下Provider：
   - persistence.NewNotificationRepository
   - service.NewUserLifecycleService
   - service.NewNotificationService（如果已实现）

3. 确保新服务的依赖关系正确（如 UserLifecycleService 依赖 AgentRepository, SessionRepository, RAGRepository）

4. 重新生成依赖注入代码（如运行 wire gen）

5. 不要修改现有Provider
```

---

## 四、前端代码改造方案

### 4.1 改造目标

**核心变更：**
1. **取消独立AI页面**：移除 `/assistant` 路由和 `Assistant.vue` 页面
2. **融合到IM主界面**：在 `Chat.vue` 中集成AI功能
3. **会话列表统一**：IM会话和AI会话统一展示，系统助手会话置顶且不可删除
4. **Agent管理入口**：在IM主界面添加Agent管理入口

### 4.2 目录结构调整

```
web/src/
├── views/
│   ├── Chat.vue                      # [修改] 主界面，融合IM+AI
│   └── Assistant.vue                 # [删除] 不再需要独立AI页面
├── components/
│   ├── chat/
│   │   ├── SessionList.vue           # [修改] 会话列表支持AI会话
│   │   ├── ChatWindow.vue            # [修改] 聊天窗口支持AI消息渲染
│   │   └── AgentManageDialog.vue     # [新建] Agent管理弹窗
│   └── ai/                           # [新建] AI专用组件
│       ├── AgentCard.vue             # Agent卡片组件
│       └── CitationPanel.vue         # 引用来源面板
├── api/
│   └── ai.js                         # [修改] 新增接口
├── router/
│   └── index.js                      # [修改] 移除 /assistant 路由
└── store/
    └── index.js                      # [修改] 整合AI会话状态
```

### 4.3 详细改造步骤

---

#### **前端阶段一：API层改造**

**步骤 F1.1：修改 `src/api/ai.js`**

**改动内容：**

1. 新增接口：获取系统助手会话

```javascript
/**
 * Get system AI assistant session
 * @returns {Promise}
 */
export const getSystemSession = () => {
  return request.get('/ai/assistant/system-session')
}
```

2. 修改 `getSessions` 接口，支持类型过滤

```javascript
/**
 * Get user's AI assistant sessions (support filtering by type)
 * @param {Object} params - { limit, offset, type }
 * @returns {Promise}
 */
export const getSessions = (params = {}) => {
  return request.get('/ai/assistant/sessions', { params })
}
```

**AI开发Prompt：**

```
任务：修改 web/src/api/ai.js

1. 新增函数 getSystemSession：
   - 发送 GET 请求到 /ai/assistant/system-session
   - 返回 Promise

2. 修改 getSessions 函数：
   - 修改函数签名：export const getSessions = (params = {})
   - 传递 params 参数（支持 limit, offset, type）
   - 保持现有调用兼容（params默认为空对象）

3. 添加JSDoc注释说明参数和返回值

4. 不要修改其他接口
```

---

#### **前端阶段二：Vuex状态管理改造**

**步骤 F2.1：修改 `src/store/index.js`**

**改动内容：**

1. 在state中新增AI相关状态：

```javascript
const store = createStore({
  state: {
    // ... 现有状态保持不变 ...
    
    // 【新增】AI相关状态
    systemAISession: null,        // 系统助手会话信息
    aiSessions: [],               // AI会话列表
    showAgentManage: false,       // 是否显示Agent管理弹窗
  },
  
  mutations: {
    // ... 现有mutations保持不变 ...
    
    // 【新增】AI相关mutations
    setSystemAISession(state, session) {
      state.systemAISession = session
    },
    setAISessions(state, sessions) {
      state.aiSessions = sessions
    },
    setShowAgentManage(state, show) {
      state.showAgentManage = show
    },
  },
  
  actions: {
    // ... 现有actions保持不变 ...
    
    // 【新增】加载系统助手会话
    async loadSystemAISession({ commit }) {
      try {
        const res = await getSystemSession()
        if (res.data && res.data.code === 200) {
          commit('setSystemAISession', res.data.data)
        }
      } catch (error) {
        console.error('Failed to load system AI session:', error)
      }
    },
    
    // 【新增】加载AI会话列表
    async loadAISessions({ commit }, params = {}) {
      try {
        const res = await getSessions(params)
        if (res.data && res.data.code === 200) {
          commit('setAISessions', res.data.data?.sessions || [])
        }
      } catch (error) {
        console.error('Failed to load AI sessions:', error)
      }
    },
  }
})
```

**AI开发Prompt：**

```
任务：修改 web/src/store/index.js

1. 在 state 中新增三个字段：
   - systemAISession: null（系统助手会话信息）
   - aiSessions: []（AI会话列表）
   - showAgentManage: false（Agent管理弹窗显示状态）

2. 在 mutations 中新增三个方法：
   - setSystemAISession(state, session)
   - setAISessions(state, sessions)
   - setShowAgentManage(state, show)

3. 在 actions 中新增两个方法：
   - loadSystemAISession：调用 getSystemSession() API，提交mutation
   - loadAISessions：调用 getSessions(params) API，提交mutation

4. 导入 getSystemSession 和 getSessions（从 ../api/ai）

5. 不要修改现有的state、mutations、actions
```

---

#### **前端阶段三：组件改造**

**步骤 F3.1：修改 `src/components/chat/SessionList.vue`**

**改动内容：**

1. 在会话列表中融合AI会话：

```vue
<template>
  <div class="session-list glass-panel">
    <div class="header">
      <h3 class="title">会话</h3>
      <div class="header-actions">
        <!-- 创建群组 -->
        <el-button circle icon="Plus" size="small" @click="emit('show-create-group')" />
        <!-- 【新增】Agent管理入口 -->
        <el-button circle icon="Setting" size="small" @click="openAgentManage" title="Agent管理" />
      </div>
    </div>

    <div class="list-content custom-scrollbar">
      <!-- 【新增】系统AI助手会话（置顶，不可删除） -->
      <div 
        v-if="systemAISession" 
        class="list-item system-ai-session"
        :class="{ active: currentSessionId === systemAISession.session_id }"
        @click="handleSelectAISession(systemAISession)"
      >
        <div class="item-icon ai-icon">
          <el-icon><MagicStick /></el-icon>
        </div>
        <div class="item-info">
          <div class="item-top">
            <span class="name">{{ systemAISession.title }}</span>
            <el-tag size="small" type="primary" effect="plain">AI</el-tag>
          </div>
          <div class="desc text-ellipsis">您的专属智能助理</div>
        </div>
      </div>

      <!-- 【新增】用户自定义AI会话 -->
      <div 
        v-for="aiSession in aiSessions" 
        :key="'ai-' + aiSession.session_id"
        class="list-item ai-session"
        :class="{ active: currentSessionId === aiSession.session_id }"
        @click="handleSelectAISession(aiSession)"
      >
        <div class="item-icon ai-icon">
          <el-icon><UserFilled /></el-icon>
        </div>
        <div class="item-info">
          <div class="item-top">
            <span class="name">{{ aiSession.title }}</span>
            <el-tag size="small" type="info" effect="plain">AI</el-tag>
          </div>
          <div class="desc text-ellipsis">{{ aiSession.summary || '点击开始对话' }}</div>
        </div>
      </div>

      <!-- 现有IM会话列表保持不变 -->
      <div 
        v-for="session in imSessions" 
        :key="'im-' + session.session_id"
        class="list-item"
        :class="{ active: currentSessionId === session.session_id }"
        @click="handleSelectSession(session)"
      >
        <!-- 现有IM会话UI保持不变 -->
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useStore } from 'vuex'
import { MagicStick, UserFilled, Plus, Setting } from '@element-plus/icons-vue'

const store = useStore()
const emit = defineEmits(['select-session', 'show-create-group'])

// 系统AI助手会话
const systemAISession = computed(() => store.state.systemAISession)

// 用户自定义AI会话（过滤掉系统会话）
const aiSessions = computed(() => 
  store.state.aiSessions.filter(s => s.session_type !== 'system_global')
)

// IM会话列表
const imSessions = computed(() => store.state.sessionList)

const currentSessionId = computed(() => store.state.currentSessionId)

// 选择AI会话
const handleSelectAISession = (session) => {
  emit('select-session', { ...session, type: 'ai' })
}

// 选择IM会话
const handleSelectSession = (session) => {
  emit('select-session', { ...session, type: 'im' })
}

// 打开Agent管理弹窗
const openAgentManage = () => {
  store.commit('setShowAgentManage', true)
}

// 初始化加载
onMounted(async () => {
  await store.dispatch('loadSystemAISession')
  await store.dispatch('loadAISessions')
})
</script>

<style scoped>
/* 新增样式 */
.header-actions {
  display: flex;
  gap: 8px;
}

.system-ai-session {
  background: linear-gradient(135deg, rgba(138, 43, 226, 0.1) 0%, rgba(65, 105, 225, 0.05) 100%);
  border-left: 3px solid #8a2be2;
}

.ai-session .ai-icon {
  background: linear-gradient(135deg, #FF9A9E 0%, #FECFEF 100%);
}

.system-ai-session .ai-icon {
  background: linear-gradient(135deg, #8a2be2 0%, #4169e1 100%);
}

/* 现有样式保持不变 */
</style>
```

**AI开发Prompt：**

```
任务：修改 web/src/components/chat/SessionList.vue

1. 在模板中新增三个部分（在IM会话列表之前）：
   a. 系统AI助手会话（v-if="systemAISession"）
      - 使用特殊样式 system-ai-session
      - 图标：MagicStick
      - 标签：<el-tag type="primary">AI</el-tag>
      - 点击事件：handleSelectAISession(systemAISession)
   
   b. 用户自定义AI会话列表（v-for="aiSession in aiSessions"）
      - 样式：ai-session
      - 图标：UserFilled
      - 标签：<el-tag type="info">AI</el-tag>
      - 点击事件：handleSelectAISession(aiSession)
   
   c. 在header新增Agent管理按钮
      - 图标：Setting
      - 点击事件：openAgentManage

2. 在 <script setup> 中：
   - 新增计算属性：systemAISession, aiSessions（过滤session_type != 'system_global'）
   - 新增方法：handleSelectAISession（emit 'select-session'，附加 type: 'ai'）
   - 新增方法：openAgentManage（commit 'setShowAgentManage', true）
   - 在 onMounted 中调用：store.dispatch('loadSystemAISession') 和 loadAISessions

3. 在 <style scoped> 中新增样式：
   - .system-ai-session：紫色渐变背景
   - .ai-session .ai-icon：粉色渐变背景
   - .header-actions：flex布局

4. 不要修改现有IM会话的模板和逻辑
```

---

**步骤 F3.2：修改 `src/views/Chat.vue`**

**改动内容：**

1. 处理AI会话选择：

```vue
<script setup>
// ... 现有代码保持不变 ...

// 修改 handleSelectSession 方法
const handleSelectSession = (session) => {
  if (session.type === 'ai') {
    // AI会话
    store.commit('setCurrentSession', { 
      sessionId: session.session_id, 
      peerId: null, // AI会话无peerId
      isAISession: true
    })
    loadAIMessages(session.session_id)
  } else {
    // IM会话（现有逻辑保持不变）
    const peerId = session.peer_id
    store.commit('setCurrentSession', { 
      sessionId: session.session_id, 
      peerId: peerId,
      isAISession: false
    })
    historyPageMap.value[peerId] = 1
    historyNoMoreMap.value[peerId] = false
    loadHistoryMessages(peerId, 1, false)
  }
}

// 新增：加载AI会话消息
const loadAIMessages = async (sessionId) => {
  try {
    const res = await getSessionMessages(sessionId, { limit: 100, offset: 0 })
    if (res.data && res.data.code === 200) {
      store.commit('setAIMessages', { sessionId, messages: res.data.data.messages || [] })
    }
  } catch (error) {
    console.error('Failed to load AI messages:', error)
  }
}

// ... 现有代码保持不变 ...
</script>
```

2. 在模板中添加Agent管理弹窗：

```vue
<template>
  <div class="chat-container">
    <!-- ... 现有内容保持不变 ... -->

    <!-- 【新增】Agent管理弹窗 -->
    <AgentManageDialog v-model:visible="showAgentManage" />
  </div>
</template>

<script setup>
import AgentManageDialog from '../components/chat/AgentManageDialog.vue'
import { getSessionMessages } from '../api/ai'

const showAgentManage = computed(() => store.state.showAgentManage)

// ... 其他代码 ...
</script>
```

**AI开发Prompt：**

```
任务：修改 web/src/views/Chat.vue

1. 修改 handleSelectSession 方法：
   - 判断 session.type 是否为 'ai'
   - 如果是AI会话：
     - commit 'setCurrentSession'，附加 isAISession: true
     - 调用 loadAIMessages(session.session_id)
   - 如果是IM会话：保持现有逻辑不变

2. 新增 loadAIMessages 方法：
   - 调用 getSessionMessages(sessionId, { limit: 100, offset: 0 })
   - commit 'setAIMessages'（需在store中新增此mutation）

3. 在模板中新增 <AgentManageDialog v-model:visible="showAgentManage" />

4. 导入 AgentManageDialog 组件和 getSessionMessages API

5. 新增计算属性 showAgentManage（从store获取）

6. 不要修改现有的IM会话处理逻辑
```

---

**步骤 F3.3：新建 `src/components/chat/AgentManageDialog.vue`**

**文件内容：**

```vue
<template>
  <el-dialog
    v-model="dialogVisible"
    title="Agent 管理"
    width="800px"
    append-to-body
  >
    <div class="agent-manage-content">
      <!-- Agent列表 -->
      <div class="agent-list-section">
        <div class="section-header">
          <h4>我的 Agent</h4>
          <el-button size="small" type="primary" @click="openCreateAgent">
            <el-icon><Plus /></el-icon> 创建新 Agent
          </el-button>
        </div>

        <div class="agent-grid">
          <div 
            v-for="agent in agents" 
            :key="agent.agent_id"
            class="agent-card"
            @click="selectAgent(agent)"
          >
            <div class="agent-card-header">
              <el-icon class="agent-icon"><UserFilled /></el-icon>
              <el-tag v-if="agent.is_system_global" size="small" type="primary">系统</el-tag>
            </div>
            <div class="agent-card-body">
              <h5>{{ agent.name }}</h5>
              <p class="agent-desc">{{ agent.description || '暂无描述' }}</p>
              <div class="agent-meta">
                <el-tag size="small" effect="plain">
                  {{ agent.kb_type === 'global' ? '全局知识库' : '私有知识库' }}
                </el-tag>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Agent会话列表 -->
      <div class="agent-sessions-section" v-if="selectedAgent">
        <div class="section-header">
          <h4>{{ selectedAgent.name }} 的会话</h4>
          <el-button 
            size="small" 
            @click="createSessionForAgent"
            :disabled="selectedAgent.is_system_global"
          >
            <el-icon><Plus /></el-icon> 新建会话
          </el-button>
        </div>

        <el-empty v-if="agentSessions.length === 0" description="暂无会话" />
        <div v-else class="session-list-mini">
          <div 
            v-for="session in agentSessions" 
            :key="session.session_id"
            class="session-item-mini"
          >
            <span>{{ session.title }}</span>
            <el-button 
              link 
              type="danger" 
              size="small"
              v-if="session.is_deletable"
              @click="deleteSession(session.session_id)"
            >
              删除
            </el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 创建Agent弹窗（复用现有逻辑） -->
    <!-- ... 省略，参考 Assistant.vue 的实现 ... -->
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useStore } from 'vuex'
import { Plus, UserFilled } from '@element-plus/icons-vue'
import { getAgents, getSessions, createSession } from '../../api/ai'
import { ElMessage } from 'element-plus'

const props = defineProps({
  visible: Boolean
})

const emit = defineEmits(['update:visible'])

const store = useStore()

const dialogVisible = computed({
  get: () => props.visible,
  set: (val) => emit('update:visible', val)
})

const agents = ref([])
const selectedAgent = ref(null)
const agentSessions = ref([])

// 加载Agents
const loadAgents = async () => {
  try {
    const res = await getAgents()
    if (res.data && res.data.code === 200) {
      agents.value = res.data.data?.agents || []
    }
  } catch (error) {
    console.error('Failed to load agents:', error)
  }
}

// 选择Agent
const selectAgent = async (agent) => {
  selectedAgent.value = agent
  
  // 加载该Agent的会话列表
  try {
    const res = await getSessions({ agent_id: agent.agent_id })
    if (res.data && res.data.code === 200) {
      agentSessions.value = res.data.data?.sessions || []
    }
  } catch (error) {
    console.error('Failed to load agent sessions:', error)
  }
}

// 创建会话
const createSessionForAgent = async () => {
  if (!selectedAgent.value) return
  
  try {
    const res = await createSession({
      agent_id: selectedAgent.value.agent_id,
      title: '新对话'
    })
    if (res.data && res.data.code === 200) {
      ElMessage.success('会话创建成功')
      selectAgent(selectedAgent.value) // 刷新会话列表
    }
  } catch (error) {
    ElMessage.error('创建会话失败')
  }
}

// 打开创建Agent弹窗
const openCreateAgent = () => {
  // TODO: 实现创建Agent逻辑（复用 Assistant.vue 的实现）
}

// 监听弹窗打开
watch(dialogVisible, (val) => {
  if (val) {
    loadAgents()
  }
})
</script>

<style scoped>
.agent-manage-content {
  display: flex;
  gap: 20px;
  min-height: 400px;
}

.agent-list-section {
  flex: 1;
}

.agent-sessions-section {
  flex: 1;
  border-left: 1px solid #eee;
  padding-left: 20px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 15px;
}

.agent-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 15px;
}

.agent-card {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 15px;
  cursor: pointer;
  transition: all 0.3s;
}

.agent-card:hover {
  border-color: #8a2be2;
  box-shadow: 0 4px 12px rgba(138, 43, 226, 0.1);
}

.agent-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.agent-icon {
  font-size: 24px;
  color: #8a2be2;
}

.agent-card-body h5 {
  margin: 0 0 8px;
  font-size: 16px;
}

.agent-desc {
  font-size: 12px;
  color: #999;
  margin-bottom: 10px;
}

.session-list-mini {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.session-item-mini {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px;
  background: #f5f5f5;
  border-radius: 4px;
}
</style>
```

**AI开发Prompt：**

```
任务：新建文件 web/src/components/chat/AgentManageDialog.vue

1. 创建一个el-dialog组件，包含两列布局：
   - 左列：Agent列表（网格布局，每个Agent显示为卡片）
   - 右列：选中Agent的会话列表

2. 左列功能：
   - 显示所有Agent（调用 getAgents()）
   - 每个Agent卡片显示：名称、描述、知识库类型、是否系统Agent
   - 点击卡片选中Agent，加载其会话列表

3. 右列功能：
   - 显示选中Agent的会话列表（调用 getSessions({ agent_id })）
   - 支持创建新会话（调用 createSession，系统Agent禁用）
   - 支持删除会话（仅is_deletable=true的会话）

4. 使用 v-model:visible 实现弹窗显示控制

5. 样式：
   - 两列布局（flex 1:1）
   - Agent卡片网格（grid，最小200px）
   - hover效果：边框紫色，阴影

6. 导入必要的图标和API
```

---

**步骤 F3.4：修改 `src/router/index.js`**

**改动内容：**

移除 `/assistant` 路由：

```javascript
const routes = [
  {
    path: '/',
    redirect: '/login'
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('../views/access/Login.vue')
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('../views/access/Register.vue')
  },
  {
    path: '/chat',
    name: 'Chat',
    component: () => import('../views/Chat.vue'),
    meta: { requiresAuth: true }
  },
  // 【删除】/assistant 路由
  // {
  //   path: '/assistant',
  //   name: 'Assistant',
  //   component: () => import('../views/Assistant.vue'),
  //   meta: { requiresAuth: true }
  // }
]
```

**AI开发Prompt：**

```
任务：修改 web/src/router/index.js

1. 移除 /assistant 路由配置（注释掉或删除）

2. 保持其他路由不变

3. 如果用户登录后默认跳转到 /assistant，需修改为跳转到 /chat
```

---

#### **前端阶段四：清理和优化**

**步骤 F4.1：删除 `src/views/Assistant.vue`**

**操作：**
- 删除文件 `web/src/views/Assistant.vue`
- 该页面的功能已完全整合到 `Chat.vue` 和 `AgentManageDialog.vue`

**AI开发Prompt：**

```
任务：清理代码

1. 删除文件 web/src/views/Assistant.vue

2. 检查项目中是否还有其他地方引用 Assistant.vue，如有则移除引用

3. 运行项目检查是否有编译错误
```

---

**步骤 F4.2：更新 Vuex Store（补充AI消息管理）**

**改动内容：**

在 `store/index.js` 中新增AI消息的管理逻辑：

```javascript
const store = createStore({
  state: {
    // ... 现有状态 ...
    aiMessages: {}, // { sessionId: [messages] }
  },
  
  mutations: {
    // ... 现有mutations ...
    
    setAIMessages(state, { sessionId, messages }) {
      state.aiMessages[sessionId] = messages
    },
    
    appendAIMessage(state, { sessionId, message }) {
      if (!state.aiMessages[sessionId]) {
        state.aiMessages[sessionId] = []
      }
      state.aiMessages[sessionId].push(message)
    },
  },
  
  getters: {
    // ... 现有getters ...
    
    currentAIMessages: (state) => {
      const sessionId = state.currentSessionId
      return state.aiMessages[sessionId] || []
    },
  },
})
```

**AI开发Prompt：**

```
任务：修改 web/src/store/index.js

1. 在 state 中新增字段：
   - aiMessages: {}（存储AI会话消息，key为sessionId）

2. 在 mutations 中新增方法：
   - setAIMessages({ sessionId, messages })：设置指定会话的消息列表
   - appendAIMessage({ sessionId, message })：追加单条消息

3. 在 getters 中新增：
   - currentAIMessages：根据currentSessionId返回当前AI会话的消息

4. 不要修改现有的IM消息管理逻辑
```

---

## 五、测试验证方案

### 5.1 后端测试

**测试项 1：数据库迁移验证**

```bash
# 执行迁移脚本
mysql -u root -p omnilink < internal/modules/ai/migrations/001_add_system_global_fields.sql

# 验证表结构
SHOW CREATE TABLE ai_agent;
SHOW CREATE TABLE ai_assistant_session;
SHOW CREATE TABLE ai_assistant_message;
SHOW CREATE TABLE ai_system_notification;
```

**测试项 2：用户注册后自动创建全局助手**

```bash
# 注册新用户
curl -X POST http://localhost:8000/api/user/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "123456",
    "nickname": "测试用户"
  }'

# 验证数据库中是否创建了全局Agent和系统会话
SELECT * FROM ai_agent WHERE owner_id = 'U_xxx' AND is_system_global = 1;
SELECT * FROM ai_assistant_session WHERE tenant_user_id = 'U_xxx' AND session_type = 'system_global';
```

**测试项 3：API接口测试**

```bash
# 获取系统助手会话
curl -X GET http://localhost:8000/ai/assistant/system-session \
  -H "Authorization: Bearer <token>"

# 获取会话列表（过滤系统会话）
curl -X GET "http://localhost:8000/ai/assistant/sessions?type=system_global" \
  -H "Authorization: Bearer <token>"

# 发送消息到系统助手
curl -X POST http://localhost:8000/ai/assistant/chat \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  -d '{
    "question": "你好，请介绍一下你的功能",
    "session_id": "AS_xxx"
  }'
```

### 5.2 前端测试

**测试项 1：页面加载和路由**

1. 访问 `/chat`，检查是否正常显示
2. 检查 `/assistant` 路由是否已移除（访问应404或重定向）
3. 检查会话列表是否同时显示IM会话和AI会话

**测试项 2：系统助手会话**

1. 登录后检查是否自动显示"🤖 AI助手"会话（置顶）
2. 点击系统助手会话，检查是否正常加载历史消息
3. 发送消息，检查是否正常收到AI回复
4. 检查系统助手会话是否无删除按钮

**测试项 3：Agent管理**

1. 点击会话列表的"设置"按钮，打开Agent管理弹窗
2. 检查是否显示系统全局Agent和用户自定义Agent
3. 创建新Agent，检查是否成功
4. 选择Agent后创建新会话，检查是否成功
5. 删除普通会话，检查是否成功（系统会话不可删除）

### 5.3 集成测试

**场景 1：新用户注册流程**

1. 注册新用户
2. 登录后自动跳转到 `/chat`
3. 检查会话列表顶部是否显示"🤖 AI助手"
4. 点击进入，发送"你好"，检查是否收到回复

**场景 2：多会话隔离**

1. 基于全局助手创建新会话A
2. 在会话A中发送若干消息
3. 创建新会话B
4. 检查会话B是否无会话A的历史消息（上下文隔离）

**场景 3：用户自定义Agent**

1. 创建私有知识库Agent
2. 上传文档到私有知识库
3. 基于该Agent创建会话
4. 提问文档相关内容，检查是否正确召回

---

## 六、扩展性与兼容性设计

### 6.1 为后续模块预留的接口

#### 6.1.1 模块二：自定义Agent工厂

**已预留：**
- `ai_agent.capabilities_json`：用于配置Persona、Mimicry等高级功能
- `ai_agent.config_json`：用于存储推理参数（如temperature、top_p）
- 私有知识库隔离机制（`kb_type='agent_private'`）

**后续扩展点：**
- 在 `CreateAgentRequest` 中新增字段：`mimicry_user_id`（数字替身目标用户）
- 实现微调服务接口（基于聊天记录训练小模型）

#### 6.1.2 模块三：AI微服务/小工具

**已预留：**
- `ai_assistant_message.render_type` 和 `render_data_json`：用于动态UI渲染
- 前端组件结构支持插槽式扩展

**后续扩展点：**
- 新建 `SmartInputService`（智能补全、润色）
- 新建 `SummarizeService`（消息摘要）
- 前端新增 `SmartInputToolbar.vue` 组件

#### 6.1.3 模块四：智能指令系统

**已预留：**
- `ai_agent.capabilities_json` 中可配置命令权限
- 系统通知表 `ai_system_notification`（用于定时提醒）

**后续扩展点：**
- 新建 `CommandParser` 服务（解析 `/todo`、`/remind` 等指令）
- 实现定时任务调度器（触发通知推送）

#### 6.1.4 模块五：动态上下文画布

**已预留：**
- `ai_assistant_message.render_type` 和 `render_data_json`
- 前端消息渲染逻辑支持动态组件

**后续扩展点：**
- 定义 `RenderProtocol`（JSON Schema）
- 实现前端动态组件注册机制（如 `VoteCard.vue`, `MapMarker.vue`）

#### 6.1.5 模块六：群组智能协作

**已预留：**
- RAG检索范围可配置（`context_config_json`）
- Agent可绑定群组（扩展 `owner_type` 支持 `group`）

**后续扩展点：**
- 新建 `GroupModeratorAgent`（群组级Agent）
- 实现 `GroupWikiService`（群维基自动更新）

#### 6.1.6 模块七：动态AI档案

**已预留：**
- `ai_assistant_session.metadata_json`：可存储用户画像数据
- RAG支持关系范围检索（Shared_Context）

**后续扩展点：**
- 新建 `UserProfileAnalyzer`（异步分析用户关系）
- 实现 `OfflineAvatarAgent`（离线托管代理）

### 6.2 数据库扩展性设计

**字段命名规范：**
- 所有预留字段以 `_json` 结尾，采用JSON格式存储
- 避免频繁 ALTER TABLE，通过JSON扩展字段应对需求变化

**索引设计：**
- 复合索引 `idx_user_type_pinned`（支持按用户+类型+置顶查询）
- 复合索引 `idx_owner_system_global`（快速查询用户的系统Agent）

**分区预留（可选）：**
- `ai_assistant_message` 表数据量大，可按月份分区
- `ai_system_notification` 表可按状态分区（待推送/已推送/已读）

### 6.3 API接口版本管理

**建议：**
- 当前接口使用 `/ai/assistant/v1/...`前缀（预留版本号）
- 后续破坏性变更时，新增 `/ai/assistant/v2/...`
- 保持v1接口兼容，逐步迁移客户端

**示例：**
```
/ai/assistant/v1/chat          # 当前版本
/ai/assistant/v2/chat          # 未来版本（支持流式+工具调用）
```

---

## 七、分阶段实施计划

### 第一阶段：数据库和后端核心（预计2-3天）

**任务清单：**
1. [ ] 执行数据库迁移脚本
2. [ ] 修改领域实体层（agent、assistant、notification）
3. [ ] 修改仓储层（新增方法并实现）
4. [ ] 实现 `UserLifecycleService`
5. [ ] 修改 `AssistantService`（新增方法）
6. [ ] 单元测试（仓储层、服务层）

**验收标准：**
- 数据库表结构正确，索引创建成功
- 单元测试覆盖率 > 80%
- 手动注册用户后，数据库中自动创建全局Agent和系统会话

---

### 第二阶段：HTTP接口和用户集成（预计1-2天）

**任务清单：**
1. [ ] 修改HTTP Handler（新增接口）
2. [ ] 注册路由
3. [ ] 在用户模块集成AI初始化调用
4. [ ] 配置依赖注入
5. [ ] 接口测试（Postman/curl）

**验收标准：**
- 所有新增接口测试通过
- 用户注册流程正常，AI助手自动创建
- 接口响应时间 < 500ms（P99）

---

### 第三阶段：前端整合（预计3-4天）

**任务清单：**
1. [ ] 修改API层（ai.js）
2. [ ] 修改Vuex Store（新增AI状态管理）
3. [ ] 修改 SessionList.vue（融合AI会话）
4. [ ] 修改 Chat.vue（支持AI会话切换）
5. [ ] 新建 AgentManageDialog.vue
6. [ ] 删除 Assistant.vue 和 /assistant 路由
7. [ ] UI样式调整和优化

**验收标准：**
- 前端正常显示系统助手会话（置顶）
- Agent管理弹窗功能完整
- AI会话和IM会话切换流畅
- 无console错误

---

### 第四阶段：测试和优化（预计1-2天）

**任务清单：**
1. [ ] 集成测试（完整用户流程）
2. [ ] 性能测试（并发注册、消息发送）
3. [ ] 边界情况测试（异常处理、幂等性）
4. [ ] 文档完善（API文档、使用手册）
5. [ ] Code Review

**验收标准：**
- 所有测试用例通过
- 无P0/P1级别bug
- 文档完整，可交付

---

## 八、AI开发Prompt汇总

### 8.1 后端开发Prompt总览

```
## 阶段一：数据库迁移
Prompt: 执行SQL脚本 internal/modules/ai/migrations/001_add_system_global_fields.sql，验证表结构是否正确

## 阶段二：领域实体层
Prompt 2.1: 修改 domain/agent/entities.go，新增字段和常量
Prompt 2.2: 修改 domain/assistant/entities.go，新增字段和常量
Prompt 2.3: 新建 domain/notification/entities.go

## 阶段三：仓储层
Prompt 3.1: 修改 domain/repository/agent_repository.go，新增接口方法
Prompt 3.2: 修改 infrastructure/persistence/agent_repository_impl.go，实现方法
Prompt 3.3: 修改 domain/repository/assistant_repository.go，新增接口方法
Prompt 3.4: 修改 infrastructure/persistence/assistant_repository_impl.go，实现方法
Prompt 3.5: 新建 domain/repository/notification_repository.go（预留）
Prompt 3.6: 新建 infrastructure/persistence/notification_repository_impl.go（预留）

## 阶段四：应用服务层
Prompt 4.1: 新建 application/service/user_lifecycle_service.go
Prompt 4.2: 修改 application/service/assistant_service.go，新增方法
Prompt 4.3: 修改 application/dto/respond/assistant_respond.go，新增结构体

## 阶段五：HTTP接口层
Prompt 5.1: 修改 interface/http/assistant_handler.go，新增接口
Prompt 5.2: 在路由文件中注册新接口

## 阶段六：用户集成
Prompt 6.1: 在用户注册逻辑中调用 UserLifecycleService.InitializeUserAIAssistant()
Prompt 6.2: （可选）新建事件监听器 user_registered_listener.go

## 阶段七：依赖注入
Prompt 7.1: 更新DI配置，新增Provider
```

### 8.2 前端开发Prompt总览

```
## 前端阶段一：API层
Prompt F1.1: 修改 src/api/ai.js，新增接口和修改参数

## 前端阶段二：Vuex
Prompt F2.1: 修改 src/store/index.js，新增AI状态管理

## 前端阶段三：组件
Prompt F3.1: 修改 src/components/chat/SessionList.vue，融合AI会话
Prompt F3.2: 修改 src/views/Chat.vue，支持AI会话切换
Prompt F3.3: 新建 src/components/chat/AgentManageDialog.vue
Prompt F3.4: 修改 src/router/index.js，移除 /assistant 路由

## 前端阶段四：清理
Prompt F4.1: 删除 src/views/Assistant.vue
Prompt F4.2: 补充 Vuex Store AI消息管理
```

---

## 九、注意事项与风险控制

### 9.1 开发注意事项

1. **数据迁移风险**：
   - 在生产环境执行迁移前，务必备份数据库
   - 先在测试环境验证SQL脚本

2. **幂等性保证**：
   - `InitializeUserAIAssistant` 必须幂等（重复调用不报错）
   - 所有创建操作前先检查是否已存在

3. **错误处理**：
   - AI初始化失败不应阻断用户注册
   - 所有异步操作需记录详细日志

4. **性能优化**：
   - 用户注册时的AI初始化建议异步执行（消息队列）
   - 会话列表查询需优化索引，避免N+1问题

### 9.2 兼容性风险

1. **现有用户数据**：
   - 已注册用户没有系统助手会话，需执行数据回填脚本
   - 回填脚本：批量为现有用户调用 `InitializeUserAIAssistant`

2. **前端缓存**：
   - 部署后清理浏览器缓存，避免旧路由残留
   - 使用版本号标识静态资源（如 `app.v2.js`）

3. **API兼容性**：
   - 现有 `/ai/assistant/sessions` 接口新增 `type` 参数为可选
   - 旧客户端调用不受影响（向后兼容）

### 9.3 监控与回滚

**监控指标：**
- 用户注册成功率（AI初始化失败不影响注册）
- 系统助手会话创建成功率
- API响应时间（P50/P95/P99）

**回滚方案：**
- 如出现严重bug，可暂时禁用AI初始化（开关控制）
- 数据库回滚：执行反向迁移脚本（DROP COLUMN）

---

## 十、总结

本方案实现了以下核心目标：

1. **✅ 系统级全局AI助手**：每个用户自动创建唯一的全局助手和系统会话
2. **✅ 前后端融合**：AI功能完全整合到IM主界面，无独立入口
3. **✅ 会话隔离**：支持基于Agent创建多个会话，上下文独立
4. **✅ 扩展性设计**：为后续7个模块预留字段、接口和组件结构
5. **✅ 一步到位**：无过渡方案，直接达到最终架构形态

**关键设计亮点：**
- 数据库通过JSON字段预留扩展，避免频繁修改表结构
- 后端通过仓储模式和服务层分离，便于后续模块复用
- 前端通过组件化和Vuex集中管理状态，易于扩展
- 幂等性和降级处理保证系统健壮性

**后续扩展方向：**
- 模块一完善：离线总结、主动通知、MCP工具调用
- 模块三实现：智能补全、润色、消息摘要
- 模块四实现：智能指令解析、定时任务调度
- 模块五实现：动态UI渲染协议、前端组件注册
- 模块六实现：群组AI助手、群维基
- 模块七实现：用户画像分析、离线托管

---

**文档版本：** v2.0  
**最后更新：** 2026-01-29  
**作者：** OmniLink开发团队  
**审核状态：** 待审核
