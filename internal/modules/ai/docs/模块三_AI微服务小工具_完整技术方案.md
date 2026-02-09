# OmniLink 模块三：AI 微服务/小工具 - 完整技术方案

## 文档元数据
- **创建时间**: 2026-02-09
- **版本**: v1.0
- **作者**: AI架构组
- **状态**: 待评审

---

## 目录

1. [项目背景与目标](#1-项目背景与目标)
2. [需求分析与功能拆解](#2-需求分析与功能拆解)
3. [架构设计原则](#3-架构设计原则)
4. [分层架构设计](#4-分层架构设计)
5. [数据库设计](#5-数据库设计)
6. [核心模块详细设计](#6-核心模块详细设计)
7. [前后端接口设计](#7-前后端接口设计)
8. [扩展性设计](#8-扩展性设计)
9. [代码实现方案](#9-代码实现方案)
10. [性能优化与成本控制](#10-性能优化与成本控制)
11. [测试方案](#11-测试方案)
12. [部署与运维](#12-部署与运维)

---

## 1. 项目背景与目标

### 1.1 项目定位

根据 PRD（ai prd_new.md 模块三），AI 微服务/小工具是 **嵌入在前端交互流程中的无感 AI**，具备以下特征：

| 特征 | 说明 |
|------|------|
| **轻量级** | 无状态 LLM API Calls，不维护会话历史 |
| **低延迟** | 延迟敏感，建议使用专用小模型（7B/8B） |
| **前端驱动** | 前端直接触发，不经过复杂路由 |
| **场景嵌入** | 融入聊天输入框、消息列表等 UI 元素 |

### 1.2 核心功能

本阶段实现三大功能：

#### 功能1：智能输入辅助（Input Prediction）
- **实时预测**：读取输入框文字 + 近 N 条聊天记录 → AI 预测后半句 → 半透明显示
- **Tab 补全**：用户按 Tab 键直接补全
- **触发频率**：防抖处理（500ms），用户停止输入后触发

#### 功能2：智能润色（Polish）
- **自动检测**：输入完整句子后（或每隔 N 秒）自动触发
- **风格建议**：AI 根据上下文给出 2-3 个润色选项（更礼貌/更强硬/更委婉）
- **一键替换**：用户点击按钮，直接替换输入框内容

#### 功能3：信息降噪（Digest）
- **触发条件**：群聊消息挤压 > 50 条
- **交互方式**：浮现"查看摘要"按钮 → 用户点击 → 展示消息概况
- **摘要范围**：最近 50-200 条消息（可配置）

### 1.3 设计目标

| 目标 | 指标 | 实现方式 |
|------|------|----------|
| **低延迟** | P99 \< 500ms | 专用小模型（7B/8B）、边缘缓存 |
| **高可用** | 99.9% | 熔断降级、本地缓存兜底 |
| **低成本** | 单次调用成本 \< ¥0.001 | 小模型、批量处理、缓存复用 |
| **可扩展** | 新增功能无需重构 | 插件化架构、统一 Pipeline |

---

## 2. 需求分析与功能拆解

### 2.1 功能1：智能输入辅助

#### 2.1.1 用户交互流程

```
[前端]
  ↓ 用户在输入框输入："今天天气真不错，要不要一起"
  ↓ 500ms 防抖后触发 → WebSocket 发送请求
  ↓ 携带：当前输入文本 + 最近 10 条聊天记录
  
[后端]
  ↓ Stateless LLM API 调用（无需加载会话历史）
  ↓ Prompt: "根据上下文，补全用户输入的后半句"
  ↓ LLM 返回：" 去公园散步？"
  
[前端]
  ↓ 实时接收 Token 流
  ↓ 在输入框光标后半透明显示："去公园散步？"
  ↓ 用户按 Tab → 补全并显示实体字符
```

#### 2.1.2 技术要点

| 要点 | 实现方式 |
|------|----------|
| **防抖** | 前端 500ms debounce，避免频繁请求 |
| **上下文** | 最近 10 条消息（可配置），不超过 2000 字符 |
| **流式输出** | WebSocket + SSE，实时接收 Token |
| **取消机制** | 用户继续输入时取消上次预测 |

---

### 2.2 功能2：智能润色

#### 2.2.1 用户交互流程

```
[前端]
  ↓ 用户输入："给我发一下那个文件"
  ↓ 检测到句号/换行 → 触发润色检测
  ↓ 携带：当前句子 + 最近 10 条上下文
  
[后端]
  ↓ Prompt: "分析句子类型，给出 2-3 个润色方向"
  ↓ LLM 返回 JSON:
      {
        "polishes": [
          {"label": "更礼貌", "text": "麻烦您发一下那个文件，谢谢！"},
          {"label": "更简洁", "text": "请发文件"}
        ]
      }
  
[前端]
  ↓ 在输入框下方显示 2 个按钮：[更礼貌] [更简洁]
  ↓ 用户点击 [更礼貌] → 替换输入框内容
```

#### 2.2.2 技术要点

| 要点 | 实现方式 |
|------|----------|
| **触发时机** | 句号/问号/感叹号/换行符触发 |
| **去重** | 相同句子 30 秒内不重复润色 |
| **结构化输出** | LLM 返回 JSON，确保可解析性 |
| **多样性** | 每次最多 3 个选项，避免选择困难 |

---

### 2.3 功能3：信息降噪

#### 2.3.1 用户交互流程

```
[前端]
  ↓ 检测到群聊未读消息 > 50 条
  ↓ 在聊天窗口顶部显示：[查看摘要] 按钮
  ↓ 用户点击按钮
  
[后端]
  ↓ 读取最近 50-200 条消息（根据时间范围）
  ↓ Prompt: "总结这段群聊的主要话题和结论"
  ↓ LLM 返回 Markdown 格式摘要：
      ### 主要话题
      1. 项目进度讨论（张三提到延期）
      2. 明天团建安排
      
      ### 待办事项
      - @李四 需要提交代码（截止明天）
  
[前端]
  ↓ 在聊天窗口中插入摘要卡片（可折叠）
  ↓ 用户可点击"查看原始消息"跳转
```

#### 2.3.2 技术要点

| 要点 | 实现方式 |
|------|----------|
| **批量处理** | 不是每条消息触发，而是累积到阈值 |
| **范围控制** | 最多处理 200 条（约 20KB 文本） |
| **缓存策略** | 同一时间段摘要缓存 5 分钟 |
| **异步处理** | 用户点击后后台生成，前端显示 Loading |

---

## 3. 架构设计原则

### 3.1 核心原则

#### 原则1：无状态服务（Stateless）
- **禁止**：为每个功能创建独立 Agent 或 Session
- **要求**：所有功能共享同一套 Stateless LLM Proxy 层
- **理由**：避免状态管理开销，降低数据库压力

#### 原则2：插件化扩展（Plugin Architecture）
- **当前实现**：智能输入、润色、降噪三个功能
- **未来扩展**：语音转文本、自动翻译、情绪分析等
- **要求**：新功能只需添加 Plugin，无需修改 Core Pipeline

#### 原则3：成本优先（Cost-First）
- **禁止**：使用主力大模型（如 GPT-4）处理微功能
- **要求**：使用专用小模型（7B/8B），或 API 转发到低成本 Provider
- **目标**：单次调用成本 \< ¥0.001

#### 原则4：降级兜底（Graceful Degradation）
- **场景**：LLM 服务不可用时，不能影响 IM 核心功能
- **实现**：本地规则兜底（如简单的正则补全、预设润色模板）

---

### 3.2 与现有架构的兼容性

#### 3.2.1 与 AI Assistant 的区别

| 维度 | AI Assistant（模块一） | AI 微服务（模块三） |
|------|----------------------|-------------------|
| **状态管理** | Stateful（维护会话） | Stateless（即用即走） |
| **数据存储** | 消息持久化到数据库 | 不存储（或仅缓存） |
| **RAG 检索** | 需要检索历史记录 | 仅使用当前上下文 |
| **模型选择** | 主力模型（GPT-4/Claude） | 小模型（7B/8B） |
| **Pipeline** | 5 节点 Graph（复杂） | 单节点 Invoke（简单） |

#### 3.2.2 代码复用策略

```
复用层次：
1. LLM Provider（infrastructure/llm/provider.go）  ✅ 完全复用
2. Eino ChatModel（chatModel）                      ✅ 复用接口，不同实例
3. RAG Pipeline                                     ❌ 不使用
4. Session/Message Repository                      ❌ 不使用
```

---

## 4. 分层架构设计

### 4.1 DDD 分层方案

```
internal/modules/ai/
├── domain/
│   └── microservice/               # 微服务领域（新增）
│       └── entities.go             # 微服务相关实体（缓存、配置）
│
├── application/
│   ├── service/
│   │   └── ai_microservice.go     # 微服务编排层（新增）
│   └── dto/
│       ├── request/
│       │   └── microservice_request.go   # 请求 DTO（新增）
│       └── respond/
│           └── microservice_respond.go   # 响应 DTO（新增）
│
├── infrastructure/
│   ├── pipeline/
│   │   └── microservice_pipeline.go     # 微服务 Pipeline（新增）
│   ├── llm/
│   │   └── lightweight_provider.go      # 轻量模型 Provider（新增）
│   ├── cache/
│   │   └── redis_cache.go               # Redis 缓存（新增）
│   └── plugins/                         # 插件系统（新增）
│       ├── plugin_interface.go          # 插件接口定义
│       ├── input_prediction_plugin.go   # 智能输入插件
│       ├── polish_plugin.go             # 润色插件
│       └── digest_plugin.go             # 降噪插件
│
└── interface/
    ├── http/
    │   └── microservice_handler.go      # HTTP Handler（新增）
    └── websocket/                       # WebSocket Handler（新增）
        └── microservice_ws_handler.go
```

---

### 4.2 核心组件关系图

```
┌─────────────────────────────────────────────────────┐
│  Interface Layer (WebSocket/HTTP)                   │
│  - 接收前端请求                                       │
│  - 防抖控制、请求验证                                 │
└─────────────────┬───────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────────────────┐
│  Application Layer                                  │
│  - MicroserviceService（服务编排）                   │
│  - 根据功能类型路由到不同 Plugin                      │
└─────────────────┬───────────────────────────────────┘
                  ↓
┌─────────────────────────────────────────────────────┐
│  Infrastructure Layer - Plugin System               │
│  ┌───────────────┬──────────────┬─────────────┐    │
│  │ Input Plugin  │ Polish Plugin│ Digest Plugin│    │
│  └───────┬───────┴──────┬───────┴──────┬──────┘    │
│          ↓              ↓              ↓            │
│    ┌──────────────────────────────────────┐        │
│    │  MicroservicePipeline（统一调度）    │        │
│    │  - Prompt 构建                       │        │
│    │  - LLM 调用                          │        │
│    │  - 结果解析                           │        │
│    └──────────────┬───────────────────────┘        │
│                   ↓                                 │
│    ┌──────────────────────────────────────┐        │
│    │  LightweightChatModel                │        │
│    │  - 专用小模型 Provider                │        │
│    │  - 支持流式/非流式                     │        │
│    └──────────────┬───────────────────────┘        │
└───────────────────┼─────────────────────────────────┘
                    ↓
         ┌──────────────────────┐
         │  LLM API (外部服务)   │
         │  - DeepSeek/Doubao   │
         │  - 7B/8B 模型         │
         └──────────────────────┘
```

---

## 5. 数据库设计

### 5.1 核心表设计

#### 5.1.1 微服务配置表（ai_microservice_config）

```sql
CREATE TABLE `ai_microservice_config` (
  `id` bigint NOT NULL AUTO_INCREMENT COMMENT '主键',
  `service_type` varchar(50) NOT NULL COMMENT '服务类型：input_prediction/polish/digest',
  `is_enabled` tinyint NOT NULL DEFAULT 1 COMMENT '是否启用：1=是 0=否',
  `config_json` json NOT NULL COMMENT '服务配置（JSON格式）',
  `model_config_json` json NOT NULL COMMENT '模型配置',
  `prompt_template` mediumtext COMMENT 'Prompt 模板',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_type` (`service_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**字段说明**：
- `service_type`：功能类型（input_prediction/polish/digest）
- `config_json`：功能配置，如：
  ```json
  {
    "context_messages": 10,
    "debounce_ms": 500,
    "max_input_chars": 500,
    "cache_ttl_seconds": 300
  }
  ```
- `model_config_json`：模型配置，如：
  ```json
  {
    "provider": "doubao",
    "model": "doubao-lite-8k",
    "temperature": 0.7,
    "max_tokens": 100
  }
  ```

#### 5.1.2 微服务调用日志表（ai_microservice_call_log）

```sql
CREATE TABLE `ai_microservice_call_log` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `request_id` char(20) NOT NULL COMMENT '请求ID',
  `tenant_user_id` char(20) NOT NULL COMMENT '用户ID',
  `service_type` varchar(50) NOT NULL COMMENT '服务类型',
  `input_text` mediumtext COMMENT '输入文本',
  `output_text` mediumtext COMMENT '输出文本',
  `context_json` json COMMENT '上下文信息',
  `latency_ms` int COMMENT '延迟（毫秒）',
  `tokens_used` int COMMENT '消耗 Token 数',
  `is_cached` tinyint DEFAULT 0 COMMENT '是否使用缓存',
  `error_msg` text COMMENT '错误信息',
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_request_id` (`request_id`),
  KEY `idx_tenant_user_id` (`tenant_user_id`),
  KEY `idx_service_type` (`service_type`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**用途**：
- 性能监控（P50/P99 延迟）
- 成本分析（Token 消耗统计）
- 异常诊断（错误日志）

---

### 5.2 缓存设计（Redis）

#### 5.2.1 智能输入缓存

```
Key Pattern: ai:micro:input:{user_id}:{context_hash}
Value: {"prediction": "去公园散步？", "confidence": 0.85}
TTL: 300 秒（5 分钟）
```

**逻辑**：
- 对当前输入 + 上下文做 MD5 Hash
- 相同上下文复用缓存（减少 LLM 调用）

#### 5.2.2 润色结果缓存

```
Key Pattern: ai:micro:polish:{text_hash}
Value: {"polishes": [...]}
TTL: 1800 秒（30 分钟）
```

#### 5.2.3 摘要缓存

```
Key Pattern: ai:micro:digest:{group_id}:{time_range_hash}
Value: {"summary": "...", "topics": [...]}
TTL: 600 秒（10 分钟）
```

---

## 6. 核心模块详细设计

### 6.1 插件系统设计

#### 6.1.1 插件接口定义

```go
// internal/modules/ai/infrastructure/plugins/plugin_interface.go

package plugins

import "context"

// MicroservicePlugin 微服务插件接口
type MicroservicePlugin interface {
    // GetServiceType 获取服务类型
    GetServiceType() string
    
    // BuildPrompt 构建 Prompt
    BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error)
    
    // ParseResponse 解析 LLM 响应
    ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error)
    
    // Validate 验证请求参数
    Validate(ctx context.Context, req *PluginRequest) error
    
    // GetCacheKey 获取缓存 Key（返回空则不缓存）
    GetCacheKey(ctx context.Context, req *PluginRequest) string
    
    // GetCacheTTL 获取缓存 TTL（秒）
    GetCacheTTL() int
}

// PluginRequest 插件请求
type PluginRequest struct {
    TenantUserID   string                 // 用户 ID
    ServiceType    string                 // 服务类型
    Input          string                 // 输入文本
    Context        map[string]interface{} // 上下文信息
    CustomConfig   map[string]interface{} // 自定义配置
}

// PluginResponse 插件响应
type PluginResponse struct {
    Output        string                 // 输出文本
    Metadata      map[string]interface{} // 元数据
    CacheHit      bool                   // 是否命中缓存
    TokensUsed    int                    // 消耗 Token
}
```

---

#### 6.1.2 智能输入插件实现

```go
// internal/modules/ai/infrastructure/plugins/input_prediction_plugin.go

package plugins

import (
    "context"
    "crypto/md5"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "github.com/cloudwego/eino/schema"
)

type InputPredictionPlugin struct {
    config *InputPredictionConfig
}

type InputPredictionConfig struct {
    ContextMessages int    // 上下文消息数（默认 10）
    MaxInputChars   int    // 最大输入字符（默认 500）
    CacheTTL        int    // 缓存 TTL（秒）
}

func NewInputPredictionPlugin(config *InputPredictionConfig) *InputPredictionPlugin {
    if config == nil {
        config = &InputPredictionConfig{
            ContextMessages: 10,
            MaxInputChars:   500,
            CacheTTL:        300,
        }
    }
    return &InputPredictionPlugin{config: config}
}

func (p *InputPredictionPlugin) GetServiceType() string {
    return "input_prediction"
}

func (p *InputPredictionPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
    // 1. 提取上下文消息
    var contextMsgs []map[string]string
    if ctx, ok := req.Context["messages"].([]map[string]string); ok {
        contextMsgs = ctx
    }
    
    // 2. 限制上下文数量
    if len(contextMsgs) > p.config.ContextMessages {
        contextMsgs = contextMsgs[len(contextMsgs)-p.config.ContextMessages:]
    }
    
    // 3. 构建 System Prompt
    systemPrompt := `你是一个智能输入助手。根据用户当前输入和聊天历史，预测用户想说的后半句。

**规则**：
1. 预测内容要简短（\< 20 字）
2. 符合聊天语境和用户语气
3. 只返回补全部分，不要重复用户已输入的内容
4. 如果无法预测，返回空字符串`

    // 4. 构建上下文消息
    contextStr := ""
    for _, msg := range contextMsgs {
        contextStr += fmt.Sprintf("[%s]: %s\n", msg["role"], msg["content"])
    }
    
    // 5. 构建 User Message
    userPrompt := fmt.Sprintf(`聊天历史：
%s

用户当前输入：%s

请预测后半句（只返回补全部分）：`, contextStr, req.Input)
    
    return []schema.Message{
        {Role: schema.System, Content: systemPrompt},
        {Role: schema.User, Content: userPrompt},
    }, nil
}

func (p *InputPredictionPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
    // 直接返回 LLM 输出（已经是补全文本）
    return &PluginResponse{
        Output:     llmOutput,
        CacheHit:   false,
        TokensUsed: 0, // 由 Pipeline 层填充
    }, nil
}

func (p *InputPredictionPlugin) Validate(ctx context.Context, req *PluginRequest) error {
    if req.Input == "" {
        return fmt.Errorf("input is required")
    }
    if len(req.Input) > p.config.MaxInputChars {
        return fmt.Errorf("input too long (max %d chars)", p.config.MaxInputChars)
    }
    return nil
}

func (p *InputPredictionPlugin) GetCacheKey(ctx context.Context, req *PluginRequest) string {
    // 对输入 + 上下文生成 Hash
    data := fmt.Sprintf("%s|%v", req.Input, req.Context["messages"])
    hash := md5.Sum([]byte(data))
    return fmt.Sprintf("ai:micro:input:%s:%s", req.TenantUserID, hex.EncodeToString(hash[:]))
}

func (p *InputPredictionPlugin) GetCacheTTL() int {
    return p.config.CacheTTL
}
```

---

#### 6.1.3 润色插件实现

```go
// internal/modules/ai/infrastructure/plugins/polish_plugin.go

package plugins

import (
    "context"
    "crypto/md5"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "github.com/cloudwego/eino/schema"
)

type PolishPlugin struct {
    config *PolishConfig
}

type PolishConfig struct {
    MaxOptions   int // 最多返回选项数（默认 3）
    CacheTTL     int // 缓存 TTL（秒）
}

func NewPolishPlugin(config *PolishConfig) *PolishPlugin {
    if config == nil {
        config = &PolishConfig{
            MaxOptions: 3,
            CacheTTL:   1800,
        }
    }
    return &PolishPlugin{config: config}
}

func (p *PolishPlugin) GetServiceType() string {
    return "polish"
}

func (p *PolishPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
    systemPrompt := `你是一个智能文本润色助手。分析用户输入的句子，给出 2-3 个润色建议。

**输出格式（JSON）**：
{
  "polishes": [
    {"label": "更礼貌", "text": "润色后的文本"},
    {"label": "更简洁", "text": "润色后的文本"}
  ]
}

**规则**：
1. label 必须是："更礼貌"、"更简洁"、"更强硬"、"更委婉"之一
2. 每个选项必须与原句意思一致，只改变语气或风格
3. 如果原句已经很好，可以只返回 1-2 个选项`

    userPrompt := fmt.Sprintf("请为以下句子提供润色建议：\n\n%s", req.Input)
    
    return []schema.Message{
        {Role: schema.System, Content: systemPrompt},
        {Role: schema.User, Content: userPrompt},
    }, nil
}

func (p *PolishPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
    // 解析 JSON 响应
    var result struct {
        Polishes []struct {
            Label string `json:"label"`
            Text  string `json:"text"`
        } `json:"polishes"`
    }
    
    if err := json.Unmarshal([]byte(llmOutput), &result); err != nil {
        // JSON 解析失败，尝试从文本中提取
        return &PluginResponse{
            Output: llmOutput, // 降级处理
            Metadata: map[string]interface{}{
                "parse_error": err.Error(),
            },
        }, nil
    }
    
    // 限制选项数量
    if len(result.Polishes) > p.config.MaxOptions {
        result.Polishes = result.Polishes[:p.config.MaxOptions]
    }
    
    outputJSON, _ := json.Marshal(result)
    return &PluginResponse{
        Output: string(outputJSON),
        Metadata: map[string]interface{}{
            "options_count": len(result.Polishes),
        },
    }, nil
}

func (p *PolishPlugin) Validate(ctx context.Context, req *PluginRequest) error {
    if req.Input == "" {
        return fmt.Errorf("input is required")
    }
    return nil
}

func (p *PolishPlugin) GetCacheKey(ctx context.Context, req *PluginRequest) string {
    hash := md5.Sum([]byte(req.Input))
    return fmt.Sprintf("ai:micro:polish:%s", hex.EncodeToString(hash[:]))
}

func (p *PolishPlugin) GetCacheTTL() int {
    return p.config.CacheTTL
}
```

---

#### 6.1.4 摘要插件实现

```go
// internal/modules/ai/infrastructure/plugins/digest_plugin.go

package plugins

import (
    "context"
    "crypto/md5"
    "encoding/hex"
    "fmt"
    "strings"
    "github.com/cloudwego/eino/schema"
)

type DigestPlugin struct {
    config *DigestConfig
}

type DigestConfig struct {
    MaxMessages int // 最多处理消息数（默认 200）
    CacheTTL    int // 缓存 TTL（秒）
}

func NewDigestPlugin(config *DigestConfig) *DigestPlugin {
    if config == nil {
        config = &DigestConfig{
            MaxMessages: 200,
            CacheTTL:    600,
        }
    }
    return &DigestPlugin{config: config}
}

func (p *DigestPlugin) GetServiceType() string {
    return "digest"
}

func (p *DigestPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
    // 提取消息列表
    var messages []map[string]string
    if msgs, ok := req.Context["messages"].([]map[string]string); ok {
        messages = msgs
    }
    
    // 限制消息数量
    if len(messages) > p.config.MaxMessages {
        messages = messages[len(messages)-p.config.MaxMessages:]
    }
    
    systemPrompt := `你是一个智能群聊摘要助手。分析群聊消息，总结主要话题和关键信息。

**输出格式（Markdown）**：
### 主要话题
1. 话题1（参与人：@张三、@李四）
2. 话题2

### 重要结论
- 结论1
- 结论2

### 待办事项
- [ ] @张三 需要提交代码（截止时间：明天）

**规则**：
1. 只提取重要信息，忽略闲聊
2. 提及人名时使用 @
3. 按重要性排序`

    // 构建消息文本
    var messageTexts []string
    for _, msg := range messages {
        sender := msg["sender"]
        content := msg["content"]
        messageTexts = append(messageTexts, fmt.Sprintf("[%s]: %s", sender, content))
    }
    
    userPrompt := fmt.Sprintf("以下是群聊消息（共 %d 条）：\n\n%s\n\n请生成摘要：", 
        len(messages), strings.Join(messageTexts, "\n"))
    
    return []schema.Message{
        {Role: schema.System, Content: systemPrompt},
        {Role: schema.User, Content: userPrompt},
    }, nil
}

func (p *DigestPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
    // 直接返回 Markdown 摘要
    return &PluginResponse{
        Output: llmOutput,
        Metadata: map[string]interface{}{
            "format": "markdown",
        },
    }, nil
}

func (p *DigestPlugin) Validate(ctx context.Context, req *PluginRequest) error {
    if req.Context["messages"] == nil {
        return fmt.Errorf("messages context is required")
    }
    return nil
}

func (p *DigestPlugin) GetCacheKey(ctx context.Context, req *PluginRequest) string {
    // 对消息列表生成 Hash
    groupID := req.Context["group_id"].(string)
    messages := req.Context["messages"]
    data := fmt.Sprintf("%s|%v", groupID, messages)
    hash := md5.Sum([]byte(data))
    return fmt.Sprintf("ai:micro:digest:%s:%s", groupID, hex.EncodeToString(hash[:]))
}

func (p *DigestPlugin) GetCacheTTL() int {
    return p.config.CacheTTL
}
```

---

### 6.2 微服务 Pipeline 设计

```go
// internal/modules/ai/infrastructure/pipeline/microservice_pipeline.go

package pipeline

import (
    "context"
    "fmt"
    "time"
    
    "OmniLink/internal/modules/ai/infrastructure/plugins"
    "OmniLink/pkg/cache"
    "OmniLink/pkg/zlog"
    
    "github.com/cloudwego/eino/components/model"
    "go.uber.org/zap"
)

// MicroservicePipeline 微服务统一 Pipeline
type MicroservicePipeline struct {
    chatModel model.BaseChatModel
    cache     cache.Cache // Redis 缓存
    plugins   map[string]plugins.MicroservicePlugin
}

// NewMicroservicePipeline 创建微服务 Pipeline
func NewMicroservicePipeline(
    chatModel model.BaseChatModel,
    cache cache.Cache,
) *MicroservicePipeline {
    p := &MicroservicePipeline{
        chatModel: chatModel,
        cache:     cache,
        plugins:   make(map[string]plugins.MicroservicePlugin),
    }
    
    // 注册默认插件
    p.RegisterPlugin(plugins.NewInputPredictionPlugin(nil))
    p.RegisterPlugin(plugins.NewPolishPlugin(nil))
    p.RegisterPlugin(plugins.NewDigestPlugin(nil))
    
    return p
}

// RegisterPlugin 注册插件
func (p *MicroservicePipeline) RegisterPlugin(plugin plugins.MicroservicePlugin) {
    p.plugins[plugin.GetServiceType()] = plugin
}

// Execute 执行微服务调用（非流式）
func (p *MicroservicePipeline) Execute(ctx context.Context, req *plugins.PluginRequest) (*plugins.PluginResponse, error) {
    startTime := time.Now()
    
    // 1. 获取插件
    plugin, ok := p.plugins[req.ServiceType]
    if !ok {
        return nil, fmt.Errorf("unknown service type: %s", req.ServiceType)
    }
    
    // 2. 参数验证
    if err := plugin.Validate(ctx, req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 3. 检查缓存
    cacheKey := plugin.GetCacheKey(ctx, req)
    if cacheKey != "" && p.cache != nil {
        if cached, err := p.cache.Get(ctx, cacheKey); err == nil && cached != "" {
            zlog.Info("cache hit", zap.String("cache_key", cacheKey))
            return &plugins.PluginResponse{
                Output:   cached,
                CacheHit: true,
            }, nil
        }
    }
    
    // 4. 构建 Prompt
    promptMsgs, err := plugin.BuildPrompt(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("build prompt failed: %w", err)
    }
    
    // 5. 调用 LLM（转为指针数组）
    promptMsgPtrs := make([]*schema.Message, len(promptMsgs))
    for i := range promptMsgs {
        promptMsgPtrs[i] = &promptMsgs[i]
    }
    
    llmResp, err := p.chatModel.Generate(ctx, promptMsgPtrs)
    if err != nil {
        return nil, fmt.Errorf("llm generate failed: %w", err)
    }
    
    // 6. 解析响应
    resp, err := plugin.ParseResponse(ctx, llmResp.Content, req)
    if err != nil {
        return nil, fmt.Errorf("parse response failed: %w", err)
    }
    
    // 7. 填充 Token 统计
    if llmResp.ResponseMeta != nil && llmResp.ResponseMeta.Usage != nil {
        resp.TokensUsed = llmResp.ResponseMeta.Usage.TotalTokens
    }
    
    // 8. 写入缓存
    if cacheKey != "" && p.cache != nil {
        ttl := plugin.GetCacheTTL()
        if err := p.cache.Set(ctx, cacheKey, resp.Output, time.Duration(ttl)*time.Second); err != nil {
            zlog.Warn("cache set failed", zap.Error(err))
        }
    }
    
    // 9. 记录日志
    latencyMs := time.Since(startTime).Milliseconds()
    zlog.Info("microservice execute done",
        zap.String("service_type", req.ServiceType),
        zap.Int64("latency_ms", latencyMs),
        zap.Int("tokens", resp.TokensUsed),
        zap.Bool("cache_hit", resp.CacheHit))
    
    return resp, nil
}

// ExecuteStream 执行微服务调用（流式）
func (p *MicroservicePipeline) ExecuteStream(ctx context.Context, req *plugins.PluginRequest) (*schema.StreamReader[*schema.Message], error) {
    // 1. 获取插件
    plugin, ok := p.plugins[req.ServiceType]
    if !ok {
        return nil, fmt.Errorf("unknown service type: %s", req.ServiceType)
    }
    
    // 2. 参数验证
    if err := plugin.Validate(ctx, req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 3. 检查缓存（流式模式可选）
    // 注意：如果命中缓存，需要将缓存内容包装为 StreamReader
    
    // 4. 构建 Prompt
    promptMsgs, err := plugin.BuildPrompt(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("build prompt failed: %w", err)
    }
    
    // 5. 调用 LLM Stream
    promptMsgPtrs := make([]*schema.Message, len(promptMsgs))
    for i := range promptMsgs {
        promptMsgPtrs[i] = &promptMsgs[i]
    }
    
    streamReader, err := p.chatModel.Stream(ctx, promptMsgPtrs)
    if err != nil {
        return nil, fmt.Errorf("llm stream failed: %w", err)
    }
    
    return streamReader, nil
}
```

---

## 7. 前后端接口设计

### 7.1 WebSocket 接口（智能输入 - 流式）

#### 7.1.1 连接建立

```
WebSocket URL: wss://api.omnilink.com/ai/microservice/input/ws
Headers:
  Authorization: Bearer <JWT>
```

#### 7.1.2 请求消息格式

```json
{
  "action": "predict",
  "data": {
    "input": "今天天气真不错，要不要一起",
    "context": {
      "chat_id": "C12345",
      "messages": [
        {"role": "user", "content": "在吗？"},
        {"role": "assistant", "content": "在的，有什么事吗？"},
        {"role": "user", "content": "今天天气真不错，要不要一起"}
      ]
    }
  }
}
```

#### 7.1.3 响应消息格式（流式）

```json
// Event 1: Token 流
{
  "event": "delta",
  "data": {
    "token": "去公园"
  }
}

// Event 2: Token 流
{
  "event": "delta",
  "data": {
    "token": "散步"
  }
}

// Event 3: 完成
{
  "event": "done",
  "data": {
    "prediction": "去公园散步？",
    "latency_ms": 230,
    "cache_hit": false
  }
}
```

---

### 7.2 HTTP 接口（润色 - 非流式）

#### 7.2.1 请求

```http
POST /ai/microservice/polish
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "text": "给我发一下那个文件",
  "context": {
    "chat_id": "C12345",
    "messages": [
      {"role": "user", "content": "刚才说的那个文档"},
      {"role": "assistant", "content": "您是说会议纪要吗？"}
    ]
  }
}
```

#### 7.2.2 响应

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "polishes": [
      {
        "label": "更礼貌",
        "text": "麻烦您发一下那个文件，谢谢！"
      },
      {
        "label": "更简洁",
        "text": "请发文件"
      }
    ],
    "latency_ms": 180,
    "cache_hit": true
  }
}
```

---

### 7.3 HTTP 接口（摘要 - 非流式）

#### 7.3.1 请求

```http
POST /ai/microservice/digest
Authorization: Bearer <JWT>
Content-Type: application/json

{
  "group_id": "G12345",
  "message_count": 50,
  "time_range": {
    "start": "2026-02-09T10:00:00Z",
    "end": "2026-02-09T12:00:00Z"
  }
}
```

**说明**：前端不直接传消息内容，后端从数据库读取

#### 7.3.2 响应

```json
{
  "code": 200,
  "msg": "success",
  "data": {
    "summary": "### 主要话题\n1. 项目进度讨论（@张三提到延期）\n2. 明天团建安排\n\n### 待办事项\n- [ ] @李四 需要提交代码（截止明天）",
    "topics": ["项目进度", "团建"],
    "mentions": ["@张三", "@李四"],
    "latency_ms": 450
  }
}
```

---

## 8. 扩展性设计

### 8.1 未来功能扩展示例

#### 8.1.1 扩展：语音转文本

```go
// 新建 plugins/voice_to_text_plugin.go
type VoiceToTextPlugin struct {}

func (p *VoiceToTextPlugin) GetServiceType() string {
    return "voice_to_text"
}

func (p *VoiceToTextPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
    // 调用 Whisper API，不使用 LLM
    // 返回空 Prompt，在 ParseResponse 中处理
    return nil, nil
}

func (p *VoiceToTextPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
    // 直接调用 Whisper API
    audioData := req.Context["audio_data"].([]byte)
    text := callWhisperAPI(audioData)
    return &PluginResponse{Output: text}, nil
}
```

**注册插件**：
```go
pipeline.RegisterPlugin(NewVoiceToTextPlugin())
```

---

#### 8.1.2 扩展：情绪分析

```go
type SentimentPlugin struct {}

func (p *SentimentPlugin) GetServiceType() string {
    return "sentiment"
}

func (p *SentimentPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
    systemPrompt := "分析文本情绪，返回 JSON：{\"sentiment\": \"positive/negative/neutral\", \"score\": 0.8}"
    userPrompt := fmt.Sprintf("文本：%s", req.Input)
    return []schema.Message{
        {Role: schema.System, Content: systemPrompt},
        {Role: schema.User, Content: userPrompt},
    }, nil
}

func (p *SentimentPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
    // 解析 JSON
    var result map[string]interface{}
    json.Unmarshal([]byte(llmOutput), &result)
    return &PluginResponse{Output: llmOutput, Metadata: result}, nil
}
```

---

### 8.2 与 PRD 后续模块的兼容

#### 8.2.1 与模块四（智能指令）的对接

**场景**：用户输入 `/todo 明天10点开会`

```
1. 前端检测到 / 开头 → 调用智能指令 API（模块四）
2. 模块四解析意图 → 调用 MCP 工具创建定时任务
3. 任务触发时 → 调用模块一（全局助手）→ 通过助手会话发送提醒
```

**兼容点**：
- 模块三的**润色插件**可以被模块四复用（润色指令文本）
- 模块三的**缓存层**可以共享（避免重复 LLM 调用）

#### 8.2.2 与模块五（动态UI）的对接

**场景**：AI 识别到"投票"意图 → 返回渲染指令

```json
{
  "type": "widget",
  "component": "vote_card",
  "data": {
    "options": ["周五", "周六", "周日"]
  }
}
```

**兼容方案**：
- 在 `PluginResponse.Metadata` 中添加 `render_type` 字段
- 前端根据 `render_type` 决定渲染方式（文本 / 卡片 / 白板）

---

## 9. 代码实现方案

### 9.1 Application Layer - Service

```go
// internal/modules/ai/application/service/ai_microservice.go

package service

import (
    "context"
    "fmt"
    
    "OmniLink/internal/modules/ai/application/dto/request"
    "OmniLink/internal/modules/ai/application/dto/respond"
    "OmniLink/internal/modules/ai/infrastructure/pipeline"
    "OmniLink/internal/modules/ai/infrastructure/plugins"
    "OmniLink/pkg/zlog"
    
    "go.uber.org/zap"
)

// AIMicroserviceService AI 微服务接口
type AIMicroserviceService interface {
    // Predict 智能输入预测（非流式）
    Predict(ctx context.Context, req request.PredictRequest, tenantUserID string) (*respond.PredictRespond, error)
    
    // PredictStream 智能输入预测（流式）
    PredictStream(ctx context.Context, req request.PredictRequest, tenantUserID string) (<-chan StreamEvent, error)
    
    // Polish 文本润色
    Polish(ctx context.Context, req request.PolishRequest, tenantUserID string) (*respond.PolishRespond, error)
    
    // Digest 消息摘要
    Digest(ctx context.Context, req request.DigestRequest, tenantUserID string) (*respond.DigestRespond, error)
}

type aiMicroserviceImpl struct {
    pipeline *pipeline.MicroservicePipeline
}

func NewAIMicroserviceService(pipe *pipeline.MicroservicePipeline) AIMicroserviceService {
    return &aiMicroserviceImpl{pipeline: pipe}
}

func (s *aiMicroserviceImpl) Predict(ctx context.Context, req request.PredictRequest, tenantUserID string) (*respond.PredictRespond, error) {
    pluginReq := &plugins.PluginRequest{
        TenantUserID: tenantUserID,
        ServiceType:  "input_prediction",
        Input:        req.Input,
        Context: map[string]interface{}{
            "messages": req.Context.Messages,
        },
    }
    
    resp, err := s.pipeline.Execute(ctx, pluginReq)
    if err != nil {
        return nil, err
    }
    
    return &respond.PredictRespond{
        Prediction: resp.Output,
        CacheHit:   resp.CacheHit,
        TokensUsed: resp.TokensUsed,
    }, nil
}

func (s *aiMicroserviceImpl) PredictStream(ctx context.Context, req request.PredictRequest, tenantUserID string) (<-chan StreamEvent, error) {
    eventChan := make(chan StreamEvent, 100)
    
    go func() {
        defer close(eventChan)
        
        pluginReq := &plugins.PluginRequest{
            TenantUserID: tenantUserID,
            ServiceType:  "input_prediction",
            Input:        req.Input,
            Context: map[string]interface{}{
                "messages": req.Context.Messages,
            },
        }
        
        streamReader, err := s.pipeline.ExecuteStream(ctx, pluginReq)
        if err != nil {
            eventChan <- StreamEvent{Event: "error", Data: map[string]string{"error": err.Error()}}
            return
        }
        
        fullPrediction := ""
        for {
            chunk, err := streamReader.Recv()
            if err != nil {
                break
            }
            token := chunk.Content
            fullPrediction += token
            eventChan <- StreamEvent{Event: "delta", Data: map[string]string{"token": token}}
        }
        
        eventChan <- StreamEvent{Event: "done", Data: map[string]string{"prediction": fullPrediction}}
    }()
    
    return eventChan, nil
}

func (s *aiMicroserviceImpl) Polish(ctx context.Context, req request.PolishRequest, tenantUserID string) (*respond.PolishRespond, error) {
    pluginReq := &plugins.PluginRequest{
        TenantUserID: tenantUserID,
        ServiceType:  "polish",
        Input:        req.Text,
        Context: map[string]interface{}{
            "messages": req.Context.Messages,
        },
    }
    
    resp, err := s.pipeline.Execute(ctx, pluginReq)
    if err != nil {
        return nil, err
    }
    
    // 解析 JSON 响应
    var polishes []respond.PolishOption
    if err := json.Unmarshal([]byte(resp.Output), &polishes); err != nil {
        return nil, fmt.Errorf("parse polishes failed: %w", err)
    }
    
    return &respond.PolishRespond{
        Polishes:   polishes,
        CacheHit:   resp.CacheHit,
        TokensUsed: resp.TokensUsed,
    }, nil
}

func (s *aiMicroserviceImpl) Digest(ctx context.Context, req request.DigestRequest, tenantUserID string) (*respond.DigestRespond, error) {
    // 1. 从数据库读取消息（这里需要注入 IM 模块的 MessageRepository）
    // 2. 构建 PluginRequest
    // 3. 调用 Pipeline
    // （省略实现细节）
    
    return &respond.DigestRespond{
        Summary: "摘要内容",
    }, nil
}
```

---

### 9.2 Interface Layer - HTTP Handler

```go
// internal/modules/ai/interface/http/microservice_handler.go

package http

import (
    "net/http"
    
    "OmniLink/internal/modules/ai/application/dto/request"
    "OmniLink/internal/modules/ai/application/service"
    "OmniLink/pkg/back"
    "OmniLink/pkg/xerr"
    
    "github.com/gin-gonic/gin"
)

type MicroserviceHandler struct {
    svc service.AIMicroserviceService
}

func NewMicroserviceHandler(svc service.AIMicroserviceService) *MicroserviceHandler {
    return &MicroserviceHandler{svc: svc}
}

// Polish 文本润色
func (h *MicroserviceHandler) Polish(c *gin.Context) {
    var req request.PolishRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        back.Error(c, xerr.InvalidParam, "参数错误")
        return
    }
    
    // 从 JWT 获取用户 ID
    uuid, exists := c.Get("uuid")
    if !exists {
        back.Error(c, xerr.Unauthorized, "未登录")
        return
    }
    tenantUserID := uuid.(string)
    
    resp, err := h.svc.Polish(c.Request.Context(), req, tenantUserID)
    if err != nil {
        back.Error(c, xerr.InternalServerError, err.Error())
        return
    }
    
    back.Result(c, resp, nil)
}

// Digest 消息摘要
func (h *MicroserviceHandler) Digest(c *gin.Context) {
    var req request.DigestRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        back.Error(c, xerr.InvalidParam, "参数错误")
        return
    }
    
    uuid, exists := c.Get("uuid")
    if !exists {
        back.Error(c, xerr.Unauthorized, "未登录")
        return
    }
    tenantUserID := uuid.(string)
    
    resp, err := h.svc.Digest(c.Request.Context(), req, tenantUserID)
    if err != nil {
        back.Error(c, xerr.InternalServerError, err.Error())
        return
    }
    
    back.Result(c, resp, nil)
}
```

---

### 9.3 Interface Layer - WebSocket Handler

```go
// internal/modules/ai/interface/websocket/microservice_ws_handler.go

package websocket

import (
    "context"
    "encoding/json"
    "net/http"
    
    "OmniLink/internal/modules/ai/application/dto/request"
    "OmniLink/internal/modules/ai/application/service"
    "OmniLink/pkg/zlog"
    
    "github.com/gin-gonic/gin"
    "github.com/gorilla/websocket"
    "go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        return true // 生产环境需要严格校验
    },
}

type MicroserviceWSHandler struct {
    svc service.AIMicroserviceService
}

func NewMicroserviceWSHandler(svc service.AIMicroserviceService) *MicroserviceWSHandler {
    return &MicroserviceWSHandler{svc: svc}
}

// InputPrediction 智能输入 WebSocket
func (h *MicroserviceWSHandler) InputPrediction(c *gin.Context) {
    // 1. Upgrade to WebSocket
    conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
    if err != nil {
        zlog.Error("websocket upgrade failed", zap.Error(err))
        return
    }
    defer conn.Close()
    
    // 2. 从 JWT 获取用户 ID
    uuid, exists := c.Get("uuid")
    if !exists {
        conn.WriteJSON(map[string]string{"event": "error", "error": "未登录"})
        return
    }
    tenantUserID := uuid.(string)
    
    // 3. 循环接收消息
    for {
        var wsMsg struct {
            Action string                 `json:"action"`
            Data   request.PredictRequest `json:"data"`
        }
        
        if err := conn.ReadJSON(&wsMsg); err != nil {
            zlog.Warn("websocket read failed", zap.Error(err))
            break
        }
        
        if wsMsg.Action != "predict" {
            continue
        }
        
        // 4. 调用流式服务
        eventChan, err := h.svc.PredictStream(context.Background(), wsMsg.Data, tenantUserID)
        if err != nil {
            conn.WriteJSON(map[string]string{"event": "error", "error": err.Error()})
            continue
        }
        
        // 5. 发送流式响应
        for event := range eventChan {
            if err := conn.WriteJSON(event); err != nil {
                zlog.Warn("websocket write failed", zap.Error(err))
                break
            }
        }
    }
}
```

---

## 10. 性能优化与成本控制

### 10.1 性能优化策略

#### 10.1.1 多级缓存

```
L1 缓存（本地内存） → 100ms TTL
  ↓ Miss
L2 缓存（Redis）     → 300s TTL
  ↓ Miss
LLM API 调用
```

**实现**：
```go
type TieredCache struct {
    local  *sync.Map           // 本地缓存
    redis  cache.Cache         // Redis 缓存
    localTTL time.Duration     // 本地缓存 TTL
}

func (c *TieredCache) Get(ctx context.Context, key string) (string, error) {
    // L1: 本地缓存
    if val, ok := c.local.Load(key); ok {
        return val.(string), nil
    }
    
    // L2: Redis 缓存
    val, err := c.redis.Get(ctx, key)
    if err == nil && val != "" {
        // 写入本地缓存
        c.local.Store(key, val)
        time.AfterFunc(c.localTTL, func() { c.local.Delete(key) })
        return val, nil
    }
    
    return "", fmt.Errorf("cache miss")
}
```

---

#### 10.1.2 批量处理（摘要功能）

**问题**：50 条消息 × 3 次请求 = 150 次 LLM 调用

**优化**：合并请求
```go
// 批量生成摘要（定时任务）
func (s *DigestService) BatchGenerate(ctx context.Context) {
    // 1. 查询所有未读 > 50 条的群聊
    groups := s.findGroupsNeedDigest(ctx)
    
    // 2. 批量调用 LLM（每 5 个群聊一批）
    for i := 0; i < len(groups); i += 5 {
        batch := groups[i:min(i+5, len(groups))]
        s.generateBatch(ctx, batch)
    }
}
```

---

### 10.2 成本控制

#### 10.2.1 模型选择策略

| 功能 | 模型选择 | 理由 |
|------|----------|------|
| 智能输入 | DeepSeek-7B | 低成本（¥0.0003/1K tokens），速度快 |
| 润色 | Doubao-Lite-8K | 性价比高（¥0.0005/1K tokens） |
| 摘要 | Doubao-Pro-32K | 长上下文，质量更好（¥0.003/1K tokens） |

#### 10.2.2 成本监控

```go
// 每日成本统计
type CostAnalyzer struct {
    db *gorm.DB
}

func (a *CostAnalyzer) DailyReport(ctx context.Context, date time.Time) (*CostReport, error) {
    var logs []MicroserviceCallLog
    a.db.Where("DATE(created_at) = ?", date.Format("2006-01-02")).Find(&logs)
    
    totalTokens := 0
    totalCalls := len(logs)
    cacheHitRate := 0.0
    
    for _, log := range logs {
        totalTokens += log.TokensUsed
        if log.IsCached {
            cacheHitRate += 1.0
        }
    }
    
    cacheHitRate = cacheHitRate / float64(totalCalls) * 100
    
    // 按 ¥0.0005/1K tokens 计算
    estimatedCost := float64(totalTokens) / 1000 * 0.0005
    
    return &CostReport{
        Date:         date,
        TotalCalls:   totalCalls,
        TotalTokens:  totalTokens,
        CacheHitRate: cacheHitRate,
        EstimatedCost: estimatedCost,
    }, nil
}
```

---

## 11. 测试方案

### 11.1 单元测试

#### 11.1.1 插件测试

```go
// internal/modules/ai/infrastructure/plugins/input_prediction_plugin_test.go

func TestInputPredictionPlugin_BuildPrompt(t *testing.T) {
    plugin := NewInputPredictionPlugin(nil)
    
    req := &PluginRequest{
        Input: "今天天气真不错，要不要一起",
        Context: map[string]interface{}{
            "messages": []map[string]string{
                {"role": "user", "content": "在吗？"},
            },
        },
    }
    
    msgs, err := plugin.BuildPrompt(context.Background(), req)
    assert.NoError(t, err)
    assert.Len(t, msgs, 2) // System + User
    assert.Contains(t, msgs[1].Content, "今天天气真不错")
}
```

---

### 11.2 集成测试

```go
func TestMicroservicePipeline_Execute(t *testing.T) {
    // 1. Mock ChatModel
    mockModel := &MockChatModel{
        Response: "去公园散步？",
    }
    
    // 2. 创建 Pipeline
    pipe := NewMicroservicePipeline(mockModel, nil)
    
    // 3. 执行请求
    req := &plugins.PluginRequest{
        ServiceType: "input_prediction",
        Input:       "今天天气真不错，要不要一起",
    }
    
    resp, err := pipe.Execute(context.Background(), req)
    assert.NoError(t, err)
    assert.Equal(t, "去公园散步？", resp.Output)
}
```

---

### 11.3 压力测试

```go
// 使用 go-stress-testing 工具
func BenchmarkPredictAPI(b *testing.B) {
    client := &http.Client{}
    
    for i := 0; i < b.N; i++ {
        req, _ := http.NewRequest("POST", "http://localhost:8080/ai/microservice/predict", 
            strings.NewReader(`{"input": "test"}`))
        req.Header.Set("Authorization", "Bearer "+testToken)
        
        resp, err := client.Do(req)
        if err != nil || resp.StatusCode != 200 {
            b.Fail()
        }
        resp.Body.Close()
    }
}
```

**目标指标**：
- QPS > 1000（单机）
- P99 延迟 < 500ms
- 错误率 < 0.1%

---

## 12. 部署与运维

### 12.1 部署架构

```
┌─────────────────────────────────────────┐
│  前端（Web/移动端）                      │
└─────────────┬───────────────────────────┘
              ↓ HTTPS/WSS
┌─────────────────────────────────────────┐
│  Nginx（负载均衡 + SSL终止）              │
└─────────────┬───────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│  OmniLink Backend（Go）                  │
│  - AI Microservice Module                │
│  - 部署 3 个实例（水平扩展）              │
└─────────┬───────────────┬───────────────┘
          ↓               ↓
    ┌──────────┐    ┌──────────┐
    │  MySQL   │    │  Redis   │
    └──────────┘    └──────────┘
          ↓
    ┌──────────────────┐
    │  LLM API         │
    │  (DeepSeek/Doubao)│
    └──────────────────┘
```

---

### 12.2 配置管理

```toml
# configs/config_production.toml

[aiConfig.microservice]
enabled = true

  [aiConfig.microservice.input_prediction]
  model_provider = "doubao"
  model = "doubao-lite-8k"
  temperature = 0.7
  max_tokens = 100
  context_messages = 10
  debounce_ms = 500
  cache_ttl = 300

  [aiConfig.microservice.polish]
  model_provider = "doubao"
  model = "doubao-lite-8k"
  temperature = 0.8
  max_options = 3
  cache_ttl = 1800

  [aiConfig.microservice.digest]
  model_provider = "doubao"
  model = "doubao-pro-32k"
  temperature = 0.5
  max_messages = 200
  cache_ttl = 600
```

---

### 12.3 监控指标

#### 12.3.1 关键指标

| 指标 | 告警阈值 | 处理方式 |
|------|----------|----------|
| P99 延迟 | > 1000ms | 扩容/切换模型 |
| 错误率 | > 1% | 触发熔断 |
| 缓存命中率 | < 30% | 检查缓存配置 |
| QPS | > 5000 | 自动扩容 |
| Token 消耗 | > 10M/day | 成本告警 |

#### 12.3.2 Prometheus 监控

```go
// 注册 Metrics
var (
    microserviceCallTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "ai_microservice_calls_total",
            Help: "Total number of AI microservice calls",
        },
        []string{"service_type", "status"},
    )
    
    microserviceLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ai_microservice_latency_ms",
            Help:    "AI microservice latency in milliseconds",
            Buckets: []float64{50, 100, 200, 500, 1000, 2000},
        },
        []string{"service_type"},
    )
)
```

---

## 13. 总结

### 13.1 核心优势

| 优势 | 说明 |
|------|------|
| **插件化** | 新增功能只需实现 Plugin 接口 |
| **无状态** | 不依赖会话管理，降低复杂度 |
| **成本可控** | 专用小模型 + 多级缓存 |
| **高性能** | WebSocket 流式 + Redis 缓存 |
| **可扩展** | 与 PRD 后续模块完全兼容 |

---

### 13.2 实施路线图

#### 阶段一：核心功能（2 周）
- [ ] 实现插件系统框架
- [ ] 实现智能输入插件
- [ ] 实现润色插件
- [ ] 实现摘要插件
- [ ] 数据库表创建 + 迁移

#### 阶段二：性能优化（1 周）
- [ ] 集成 Redis 缓存
- [ ] 实现多级缓存
- [ ] 性能测试 + 调优

#### 阶段三：前端对接（1 周）
- [ ] HTTP API 对接
- [ ] WebSocket 实时流式对接
- [ ] UI 交互优化

#### 阶段四：监控与上线（1 周）
- [ ] Prometheus 监控接入
- [ ] 日志完善
- [ ] 灰度发布

---

### 13.3 风险与应对

| 风险 | 应对方案 |
|------|----------|
| LLM API 不稳定 | 实现熔断降级（本地规则兜底） |
| 成本超预算 | 严格缓存策略 + 小模型优先 |
| 延迟过高 | 边缘缓存 + CDN 加速 |
| 插件冲突 | 严格的接口约束 + 单元测试 |

---

## 附录

### A. 数据库完整 DDL

```sql
-- 微服务配置表
CREATE TABLE `ai_microservice_config` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `service_type` varchar(50) NOT NULL COMMENT '服务类型',
  `is_enabled` tinyint NOT NULL DEFAULT 1,
  `config_json` json NOT NULL,
  `model_config_json` json NOT NULL,
  `prompt_template` mediumtext,
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_service_type` (`service_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 微服务调用日志表
CREATE TABLE `ai_microservice_call_log` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `request_id` char(20) NOT NULL,
  `tenant_user_id` char(20) NOT NULL,
  `service_type` varchar(50) NOT NULL,
  `input_text` mediumtext,
  `output_text` mediumtext,
  `context_json` json,
  `latency_ms` int,
  `tokens_used` int,
  `is_cached` tinyint DEFAULT 0,
  `error_msg` text,
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_request_id` (`request_id`),
  KEY `idx_tenant_user_id` (`tenant_user_id`),
  KEY `idx_service_type` (`service_type`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

---

### B. 配置示例

```toml
[redisConfig]
addr = "localhost:6379"
password = ""
db = 1
pool_size = 100

[aiConfig.microservice]
enabled = true
log_calls = true

  [aiConfig.microservice.input_prediction]
  model_provider = "doubao"
  model = "doubao-lite-8k"
  api_key = "your-api-key"
  base_url = "https://ark.cn-beijing.volces.com/api/v3"
  temperature = 0.7
  max_tokens = 100
  timeout_seconds = 5

  [aiConfig.microservice.polish]
  model_provider = "doubao"
  model = "doubao-lite-8k"
  api_key = "your-api-key"
  temperature = 0.8

  [aiConfig.microservice.digest]
  model_provider = "doubao"
  model = "doubao-pro-32k"
  api_key = "your-api-key"
  temperature = 0.5
```

---

**文档结束**

---

## 下一步行动

### 开发者指南

1. **阅读本文档**：深入理解架构设计
2. **环境准备**：安装 Redis、配置 LLM API Key
3. **代码实现**：按照 9. 代码实现方案 逐步编码
4. **测试验证**：编写单元测试 + 集成测试
5. **性能调优**：压力测试 + 缓存优化
6. **前端对接**：提供 API 文档给前端团队

### 审核要点

- [ ] 架构是否符合 DDD 分层原则
- [ ] 是否与现有 AI 模块兼容
- [ ] 插件系统是否足够灵活
- [ ] 性能指标是否达标
- [ ] 成本控制方案是否可行
- [ ] 是否为后续模块留足扩展位置

---

**声明**：本文档为生产级技术方案，所有设计均基于 OmniLink 现有架构，确保一步到位，无需后期重构。
