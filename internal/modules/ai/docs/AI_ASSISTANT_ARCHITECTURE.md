# OmniLink AI Assistant 后端架构深度解析

## 文档概览

本文档面向开发者，深入讲解 OmniLink 全局 AI 个人助手的后端实现，包括：
- **完整的代码架构**（DDD 分层设计）
- **请求链路全流程**（从 HTTP 到数据库）
- **Eino Graph 编排详解**（5节点 Pipeline）
- **核心技术栈与 API**（Go、Eino、Milvus、Kafka）
- **设计决策与最佳实践**

---

## 目录

1. [系统概述](#1-系统概述)
2. [技术栈](#2-技术栈)
3. [DDD 架构分层](#3-ddd-架构分层)
4. [数据库设计](#4-数据库设计)
5. [核心流程：Eino Graph Pipeline](#5-核心流程eino-graph-pipeline)
6. [完整请求链路](#6-完整请求链路)
7. [代码详解](#7-代码详解)
8. [关键设计决策](#8-关键设计决策)
9. [性能优化](#9-性能优化)
10. [扩展性设计](#10-扩展性设计)

---

## 1. 系统概述

### 1.1 功能定位

**OmniLink AI Assistant** 是一个全局智能助手，具备以下特性：

- **RAG 增强对话**：基于用户聊天历史、联系人、群组信息检索相关上下文
- **多轮对话管理**：维护会话状态，支持上下文延续
- **流式/非流式**：支持 SSE 实时流式输出和传统 HTTP 请求
- **租户隔离**：多租户架构，数据严格隔离
- **Agent 扩展**：支持多 Agent，可自定义人设（Persona）

### 1.2 核心特点

| 特性 | 说明 |
|------|------|
| **独立性** | 与 IM 聊天分离，独立的会话系统 |
| **RAG 集成** | 复用现有 RAG 管道，检索私域知识 |
| **Eino 编排** | 基于 Cloudwego Eino 框架构建 Graph Pipeline |
| **DDD 架构** | 严格分层：Domain → Application → Infrastructure → Interface |
| **高性能** | 流式输出，低延迟首 Token 响应 |

---

## 2. 技术栈

### 2.1 核心框架

```go
// Web 框架
github.com/gin-gonic/gin          // HTTP 路由和中间件

// AI 编排框架
github.com/cloudwego/eino          // Graph 编排核心
github.com/cloudwego/eino-ext      // LLM/Embedding 组件

// 数据存储
gorm.io/gorm                       // ORM
github.com/milvus-io/milvus-sdk-go // 向量数据库

// 消息队列
github.com/IBM/sarama              // Kafka 客户端

// 工具库
go.uber.org/zap                    // 日志
```

### 2.2 外部依赖

| 组件 | 用途 | 配置位置 |
|------|------|----------|
| **MySQL** | 会话、消息、Agent 数据 | `configs/config_local.toml` `[mysqlConfig]` |
| **Milvus** | 向量检索（RAG） | `[milvusConfig]` |
| **Kafka** | 异步事件队列（Ingest） | `[kafkaConfig]` |
| **LLM API** | Ark / OpenAI / DashScope | `[aiConfig.chatModel]` |
| **Embedding API** | DashScope / Ark | `[aiConfig.embedding]` |

---

## 3. DDD 架构分层

### 3.1 层次划分

```
┌─────────────────────────────────────────────────┐
│  Interface Layer (接口层)                       │
│  - HTTP Handlers (gin.Context)                 │
│  - DTO Request/Respond                         │
└─────────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────┐
│  Application Layer (应用层)                     │
│  - Service 接口 & 实现                          │
│  - 编排业务逻辑                                 │
└─────────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────┐
│  Infrastructure Layer (基础设施层)              │
│  - Repository 实现 (GORM)                       │
│  - Pipeline (Eino Graph)                       │
│  - LLM/Embedding Provider                      │
│  - VectorDB (Milvus)                           │
└─────────────────────────────────────────────────┘
                     ↓
┌─────────────────────────────────────────────────┐
│  Domain Layer (领域层)                          │
│  - Entities (纯数据结构)                        │
│  - Repository 接口 (抽象)                       │
└─────────────────────────────────────────────────┘
```

### 3.2 目录结构

```
internal/modules/ai/
├── domain/                          # 领域层
│   ├── assistant/
│   │   └── entities.go              # AIAssistantSession, AIAssistantMessage
│   ├── agent/
│   │   └── entities.go              # AIAgent
│   └── repository/
│       ├── assistant_repository.go  # 接口定义
│       └── agent_repository.go      # 接口定义
│
├── application/                     # 应用层
│   ├── service/
│   │   └── assistant_service.go     # AssistantService 接口 & 实现
│   └── dto/
│       ├── request/                 # 请求 DTO
│       └── respond/                 # 响应 DTO
│
├── infrastructure/                  # 基础设施层
│   ├── persistence/
│   │   ├── assistant_repository_impl.go
│   │   └── agent_repository_impl.go
│   ├── pipeline/
│   │   ├── assistant_pipeline.go    # Pipeline 主体
│   │   └── assistant_graph.go       # 5 个节点实现
│   ├── llm/
│   │   └── provider.go              # LLM 初始化
│   ├── embedding/
│   │   └── provider.go              # Embedding 初始化
│   └── vectordb/
│       └── milvus_store.go          # Milvus 封装
│
└── interface/                       # 接口层
    └── http/
        └── assistant_handler.go     # HTTP Handlers
```

---

## 4. 数据库设计

### 4.1 表结构

#### **ai_assistant_session** (会话表)

```sql
CREATE TABLE `ai_assistant_session` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `session_id` char(20) NOT NULL COMMENT '会话ID (AS前缀)',
  `tenant_user_id` char(20) NOT NULL COMMENT '租户用户ID',
  `title` varchar(100) DEFAULT NULL COMMENT '会话标题',
  `summary` text COMMENT '会话摘要',
  `agent_id` char(20) DEFAULT NULL COMMENT '绑定的Agent ID',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '状态 1:active 2:archived',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_session_id` (`session_id`),
  KEY `idx_tenant_user_id` (`tenant_user_id`),
  KEY `idx_updated_at` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**设计要点：**
- `session_id` 使用 `AS` 前缀（Assistant Session），20字符唯一ID
- `tenant_user_id` 租户隔离，所有查询必须带此字段
- `updated_at` 索引用于会话列表排序（最近聊天优先）

#### **ai_assistant_message** (消息表)

```sql
CREATE TABLE `ai_assistant_message` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `session_id` char(20) NOT NULL COMMENT '会话ID',
  `role` varchar(20) NOT NULL COMMENT 'user/assistant/system',
  `content` text NOT NULL COMMENT '消息内容',
  `citations_json` text COMMENT '引用JSON数组',
  `tokens_json` text COMMENT 'Token统计JSON',
  `created_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  KEY `idx_session_id` (`session_id`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**设计要点：**
- `citations_json` 存储 RAG 引用（柔性 Schema）
- `tokens_json` 存储 Token 统计（prompt + completion）
- 通过 `created_at` 倒序查询历史消息

#### **ai_agent** (Agent 表)

```sql
CREATE TABLE `ai_agent` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `agent_id` char(20) NOT NULL COMMENT 'Agent唯一ID',
  `owner_type` varchar(20) NOT NULL COMMENT 'system/user/group',
  `owner_id` char(20) DEFAULT NULL COMMENT '所有者ID',
  `name` varchar(50) NOT NULL COMMENT 'Agent名称',
  `description` text COMMENT 'Agent描述',
  `persona_prompt` text COMMENT '人设提示词',
  `config_json` text COMMENT '配置JSON',
  `status` tinyint NOT NULL DEFAULT 1 COMMENT '1:enabled 2:disabled',
  `created_at` datetime NOT NULL,
  `updated_at` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_agent_id` (`agent_id`),
  KEY `idx_owner` (`owner_type`, `owner_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

**设计要点：**
- `persona_prompt` 定义 Agent 角色（系统提示词）
- `config_json` 扩展配置（未来支持 Function Calling、Tools 等）

---

## 5. 核心流程：Eino Graph Pipeline

### 5.1 Pipeline 概览

**Eino Graph** 是字节跳动开源的 AI 应用编排框架，支持 DAG（有向无环图）流程定义。

```
用户问题 → LoadMemory → Retrieve → BuildPrompt → ChatModel → Persist → 返回结果
```

### 5.2 五个节点详解

#### **Node 1: LoadMemory**

**职责：** 加载会话上下文

```go
func (p *AssistantPipeline) loadMemoryNode(ctx context.Context, req *AssistantRequest, _ ...any) (*assistantState, error) {
    // 1. 校验参数
    // 2. 判断是否新会话
    //    - 新会话：创建 Session 记录
    //    - 旧会话：加载最近 6 轮（12 条消息）
    // 3. 返回 assistantState
}
```

**输入：**
- `AssistantRequest`（包含 `SessionID`, `TenantUserID`, `Question`）

**输出：**
- `assistantState`（包含 `SessionID`, `IsNewSession`, `Messages`）

**关键代码：**
```go
// 创建新会话
newSession := &assistant.AIAssistantSession{
    SessionId:    util.GenerateID("AS"), // AS + 11位随机字符
    TenantUserId: req.TenantUserID,
    Title:        truncateTitle(req.Question), // 截取前30字符
    Status:       assistant.SessionStatusActive,
    CreatedAt:    now,
    UpdatedAt:    now,
}
p.sessionRepo.CreateSession(ctx, newSession)

// 加载历史消息（最近12条）
messages, _ := p.messageRepo.ListRecentMessages(ctx, sessionID, 12)
```

---

#### **Node 2: Retrieve**

**职责：** RAG 检索相关上下文

```go
func (p *AssistantPipeline) retrieveNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
    // 1. 构建 RetrieveRequest（question, topK, scope）
    // 2. 调用 RetrievePipeline.Retrieve()
    // 3. 转换结果为 CitationEntry 并存入 state
}
```

**输入：**
- `assistantState.Req.Question`（用户问题）
- `TopK`（召回数量，默认 5）
- `Scope`（检索范围：global / chat_private / chat_group）

**输出：**
- `assistantState.RetrievedCtx`（召回的 chunks）
- `assistantState.Citations`（最终返回给前端的引用）

**关键代码：**
```go
// 调用现有 RAG Pipeline
result, err := p.retrievePipe.Retrieve(ctx, &RetrieveRequest{
    TenantUserID: req.TenantUserID,
    Question:     req.Question,
    TopK:         5,
    KBType:       "global",
})

// 转换为引用格式
citations := make([]respond.CitationEntry, 0)
for _, chunk := range result.Chunks {
    citations = append(citations, respond.CitationEntry{
        ChunkID:    fmt.Sprintf("%d", chunk.ChunkID),
        SourceType: chunk.SourceType,
        SourceKey:  chunk.SourceKey,
        Score:      chunk.Score,
        Content:    truncateContent(chunk.Content, 200),
    })
}
```

**为什么复用 RAG Pipeline？**
- RAG Pipeline 已包含完整的 `Embedding → VectorSearch → Rerank` 流程
- 避免重复实现，保持代码 DRY 原则

---

#### **Node 3: BuildPrompt**

**职责：** 构建 LLM 输入 Prompt

```go
func (p *AssistantPipeline) buildPromptNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
    // 1. 系统提示词（Persona）
    // 2. 历史消息（最近 N 轮）
    // 3. Retrieved Context（如果有）
    // 4. 当前用户问题
}
```

**Prompt 结构：**

```
[System Message]
你是 OmniLink 的全局 AI 个人助手，回答必须基于用户权限内的聊天/联系人/群组信息。
（如果有 Agent）Agent 的 PersonaPrompt

[History Messages]
User: 你好
Assistant: 你好！我是 OmniLink 助手
User: 我想了解...
Assistant: ...

[System Message - Retrieved Context]
以下是检索到的相关上下文信息（请基于这些信息回答）：
[chunk:C123] 用户在群聊中讨论了 XXX (来源: chat_group/G456, 得分: 0.892)
[chunk:C789] ...

[User Message]
请详细介绍一下量子计算
```

**关键代码：**
```go
promptMsgs := make([]schema.Message, 0)

// 1. System Persona
promptMsgs = append(promptMsgs, schema.Message{
    Role:    schema.System,
    Content: defaultPersonaPrompt, // + Agent Persona
})

// 2. History
for _, msg := range st.Messages {
    role := schema.User
    if msg.Role == "assistant" {
        role = schema.Assistant
    }
    promptMsgs = append(promptMsgs, schema.Message{
        Role:    role,
        Content: msg.Content,
    })
}

// 3. Retrieved Context
if len(st.RetrievedCtx) > 0 {
    contextStr := buildContextString(st.RetrievedCtx)
    promptMsgs = append(promptMsgs, schema.Message{
        Role:    schema.System,
        Content: fmt.Sprintf("以下是检索到的相关上下文信息：\n%s", contextStr),
    })
}

// 4. Current Question
promptMsgs = append(promptMsgs, schema.Message{
    Role:    schema.User,
    Content: st.Req.Question,
})
```

---

#### **Node 4: ChatModel**

**职责：** 调用 LLM 生成回答

```go
func (p *AssistantPipeline) chatModelNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
    // 1. 转换 Prompt 为指针数组
    // 2. 调用 chatModel.Generate()
    // 3. 提取 Answer 和 Token 统计
}
```

**两种模式：**

1. **非流式（Invoke）**
   ```go
   resp, err := p.chatModel.Generate(ctx, promptMsgs)
   st.Answer = resp.Content
   st.Tokens = TokenStats{
       PromptTokens:     resp.ResponseMeta.Usage.PromptTokens,
       CompletionTokens: resp.ResponseMeta.Usage.CompletionTokens,
       TotalTokens:      resp.ResponseMeta.Usage.TotalTokens,
   }
   ```

2. **流式（Stream）**
   ```go
   streamReader, err := p.chatModel.Stream(ctx, promptMsgs)
   // 在 Service 层处理流式读取（见下文）
   ```

**LLM Provider 初始化：**
```go
// infrastructure/llm/provider.go
func NewChatModelFromConfig(ctx context.Context, conf *config.Config) (model.BaseChatModel, error) {
    provider := conf.AIConfig.ChatModel.Provider // "ark" / "openai"
    
    switch provider {
    case "ark":
        return arkModel.NewChatModel(ctx, arkModel.Config{
            APIKey:  conf.AIConfig.ChatModel.APIKey,
            BaseURL: conf.AIConfig.ChatModel.BaseURL,
            Model:   conf.AIConfig.ChatModel.Model,
        })
    case "openai":
        return openaiModel.NewChatModel(ctx, openaiModel.Config{
            APIKey:  conf.AIConfig.ChatModel.APIKey,
            Model:   conf.AIConfig.ChatModel.Model,
        })
    }
}
```

---

#### **Node 5: Persist**

**职责：** 持久化消息到数据库

```go
func (p *AssistantPipeline) persistNode(ctx context.Context, st *assistantState, _ ...any) (*AssistantResult, error) {
    // 1. 保存 User Message
    // 2. 保存 Assistant Message（包含 Citations 和 Tokens JSON）
    // 3. 更新 Session.updated_at
    // 4. 返回 AssistantResult
}
```

**关键代码：**
```go
// 保存用户消息
userMsg := &assistant.AIAssistantMessage{
    SessionId: st.SessionID,
    Role:      "user",
    Content:   st.Req.Question,
    CreatedAt: now,
}
p.messageRepo.SaveMessage(ctx, userMsg)

// 保存 AI 回答
citationsJSON, _ := json.Marshal(st.Citations)
tokensJSON, _ := json.Marshal(st.Tokens)

assistantMsg := &assistant.AIAssistantMessage{
    SessionId:     st.SessionID,
    Role:          "assistant",
    Content:       st.Answer,
    CitationsJson: string(citationsJSON),
    TokensJson:    string(tokensJSON),
    CreatedAt:     now,
}
p.messageRepo.SaveMessage(ctx, assistantMsg)

// 更新会话时间戳
p.sessionRepo.UpdateSessionUpdatedAt(ctx, st.SessionID)
```

---

### 5.3 Graph 编排代码

```go
// infrastructure/pipeline/assistant_pipeline.go
func (p *AssistantPipeline) buildGraph(ctx context.Context) (compose.Runnable[*AssistantRequest, *AssistantResult], error) {
    const (
        LoadMemory  = "LoadMemory"
        Retrieve    = "Retrieve"
        BuildPrompt = "BuildPrompt"
        ChatModel   = "ChatModel"
        Persist     = "Persist"
    )
    
    g := compose.NewGraph[*AssistantRequest, *AssistantResult]()
    
    // 添加节点
    g.AddLambdaNode(LoadMemory, compose.InvokableLambdaWithOption(p.loadMemoryNode))
    g.AddLambdaNode(Retrieve, compose.InvokableLambdaWithOption(p.retrieveNode))
    g.AddLambdaNode(BuildPrompt, compose.InvokableLambdaWithOption(p.buildPromptNode))
    g.AddLambdaNode(ChatModel, compose.InvokableLambdaWithOption(p.chatModelNode))
    g.AddLambdaNode(Persist, compose.InvokableLambdaWithOption(p.persistNode))
    
    // 定义边（流程）
    g.AddEdge(compose.START, LoadMemory)
    g.AddEdge(LoadMemory, Retrieve)
    g.AddEdge(Retrieve, BuildPrompt)
    g.AddEdge(BuildPrompt, ChatModel)
    g.AddEdge(ChatModel, Persist)
    g.AddEdge(Persist, compose.END)
    
    // 编译为 Runnable
    return g.Compile(ctx, 
        compose.WithGraphName("AssistantPipeline"),
        compose.WithNodeTriggerMode(compose.AllPredecessor))
}
```

**Eino Graph 优势：**
- 声明式编排，清晰的 DAG 结构
- 节点间解耦，便于测试和扩展
- 支持条件分支（未来可扩展为 Agent Router）

---

## 6. 完整请求链路

### 6.1 非流式请求链路

```
[前端]
  ↓ POST /ai/assistant/chat
  ↓ Body: {"question": "量子计算是什么?", "top_k": 5}
  ↓ Header: Authorization: Bearer <JWT>
  
[Interface Layer - HTTP Handler]
  ↓ gin.Context → BindJSON
  ↓ 提取 JWT 中的 uuid (tenant_user_id)
  ↓ 调用 AssistantService.Chat()
  
[Application Layer - Service]
  ↓ 构建 AssistantRequest
  ↓ 调用 Pipeline.Execute(ctx, req)
  
[Infrastructure Layer - Pipeline]
  ↓ Node 1: LoadMemory
  │   ├─ sessionRepo.CreateSession() / GetSessionByID()
  │   └─ messageRepo.ListRecentMessages()
  ↓ Node 2: Retrieve
  │   ├─ RetrievePipeline.Retrieve()
  │   │   ├─ Embedding API (DashScope)
  │   │   ├─ Milvus VectorSearch
  │   │   └─ Rerank
  │   └─ 返回 Citations
  ↓ Node 3: BuildPrompt
  │   └─ 组装 Prompt (Persona + History + Context + Question)
  ↓ Node 4: ChatModel
  │   ├─ LLM API (Ark / OpenAI)
  │   └─ 返回 Answer + Tokens
  ↓ Node 5: Persist
  │   ├─ messageRepo.SaveMessage() (user + assistant)
  │   └─ sessionRepo.UpdateSessionUpdatedAt()
  ↓ 返回 AssistantResult
  
[Application Layer - Service]
  ↓ 转换为 AssistantChatRespond
  
[Interface Layer - HTTP Handler]
  ↓ back.Result(c, data, err)
  ↓ JSON Response
  
[前端]
  ↓ 接收 JSON: {session_id, answer, citations, timing, tokens}
```

---

### 6.2 流式请求链路

```
[前端]
  ↓ POST /ai/assistant/chat/stream
  ↓ Body: {"question": "详细介绍量子计算"}
  ↓ Header: Authorization: Bearer <JWT>
  
[Interface Layer - HTTP Handler]
  ↓ gin.Context → BindJSON
  ↓ 设置 SSE Headers
  │   Content-Type: text/event-stream
  │   Cache-Control: no-cache
  │   Connection: keep-alive
  ↓ 调用 AssistantService.ChatStream()
  
[Application Layer - Service]
  ↓ 返回 eventChan (<-chan StreamEvent)
  ↓ 启动 goroutine 执行流式处理
  │   ├─ Pipeline.ExecuteStream(ctx, req)
  │   │   ├─ 手动执行 Node 1~3
  │   │   └─ 返回 StreamReader + assistantState
  │   ├─ 循环读取 StreamReader
  │   │   for chunk := range streamReader.Recv() {
  │   │       eventChan <- StreamEvent{Event: "delta", Data: chunk}
  │   │   }
  │   ├─ Pipeline.PersistStreamResult()
  │   └─ eventChan <- StreamEvent{Event: "done", Data: result}
  
[Interface Layer - HTTP Handler]
  ↓ 读取 eventChan
  ↓ 逐个发送 SSE 事件
  │   event: delta
  │   data: {"token": "量子"}
  │   
  │   event: delta
  │   data: {"token": "计算"}
  │   
  │   event: done
  │   data: {"session_id": "AS...", "citations": [...]}
  
[前端]
  ↓ ReadableStream 读取
  ↓ 解析 SSE 格式
  ↓ 实时更新 UI
```

**流式实现关键代码：**

```go
// Service Layer
func (s *assistantServiceImpl) ChatStream(...) (<-chan StreamEvent, error) {
    eventChan := make(chan StreamEvent, 100)
    
    go func() {
        defer close(eventChan)
        
        // 执行前3个节点
        streamReader, st, err := s.pipeline.ExecuteStream(ctx, pipeReq)
        
        // 读取流式输出
        fullAnswer := ""
        for {
            chunk, err := streamReader.Recv()
            if err != nil {
                break // EOF
            }
            token := chunk.Content
            fullAnswer += token
            eventChan <- StreamEvent{Event: "delta", Data: map[string]string{"token": token}}
        }
        
        // 持久化完整结果
        result, _ := s.pipeline.PersistStreamResult(ctx, st, fullAnswer, llmMs)
        eventChan <- StreamEvent{Event: "done", Data: result}
    }()
    
    return eventChan, nil
}
```

```go
// HTTP Handler
func (h *AssistantHandler) ChatStream(c *gin.Context) {
    eventChan, err := h.svc.ChatStream(c.Request.Context(), req, uuid)
    
    for event := range eventChan {
        switch event.Event {
        case "delta":
            c.SSEvent("delta", event.Data)
            c.Writer.Flush()
        case "done":
            c.SSEvent("done", event.Data)
            c.Writer.Flush()
        case "error":
            c.SSEvent("error", event.Data)
            c.Writer.Flush()
        }
    }
}
```

---

## 7. 代码详解

### 7.1 依赖注入（Dependency Injection）

**初始化流程（api/http/https_server.go）：**

```go
// 1. 初始化 Repositories
sessionRepo := aiPersistence.NewAssistantSessionRepository(initial.GormDB)
messageRepo := aiPersistence.NewAssistantMessageRepository(initial.GormDB)
agentRepo := aiPersistence.NewAgentRepository(initial.GormDB)
ragRepo := aiPersistence.NewRAGRepository(initial.GormDB)

// 2. 初始化 RAG Pipeline
retrievePipe, _ := aiPipeline.NewRetrievePipeline(
    ragRepo, 
    embeddingModel, // 从 config 初始化
    vectorStore,    // Milvus
)

// 3. 初始化 ChatModel
chatModel, chatMeta, _ := aiLLM.NewChatModelFromConfig(ctx, conf)

// 4. 初始化 Assistant Pipeline
assistantPipe, _ := aiPipeline.NewAssistantPipeline(
    sessionRepo,
    messageRepo,
    agentRepo,
    ragRepo,
    retrievePipe,
    chatModel,
    chatMeta,
)

// 5. 初始化 Service
assistantService := aiService.NewAssistantService(
    sessionRepo,
    messageRepo,
    agentRepo,
    assistantPipe,
)

// 6. 初始化 Handler
aiAssistantH := aiHTTP.NewAssistantHandler(assistantService)

// 7. 注册路由
authed.GET("/ai/assistant/sessions", aiAssistantH.ListSessions)
authed.POST("/ai/assistant/chat", aiAssistantH.Chat)
authed.POST("/ai/assistant/chat/stream", aiAssistantH.ChatStream)
```

**设计优势：**
- 所有依赖显式传递，便于测试
- 单一职责：每层只关注自己的逻辑
- 可替换性：Repository 可替换为 Mock 实现

---

### 7.2 Repository 实现（GORM）

```go
// infrastructure/persistence/assistant_repository_impl.go
type assistantSessionRepositoryImpl struct {
    db *gorm.DB
}

func (r *assistantSessionRepositoryImpl) CreateSession(ctx context.Context, session *assistant.AIAssistantSession) error {
    return r.db.WithContext(ctx).Create(session).Error
}

func (r *assistantSessionRepositoryImpl) GetSessionByID(ctx context.Context, sessionID, tenantUserID string) (*assistant.AIAssistantSession, error) {
    var sess assistant.AIAssistantSession
    err := r.db.WithContext(ctx).
        Where("session_id = ? AND tenant_user_id = ?", sessionID, tenantUserID). // 租户隔离
        First(&sess).Error
    if errors.Is(err, gorm.ErrRecordNotFound) {
        return nil, nil
    }
    return &sess, err
}

func (r *assistantSessionRepositoryImpl) ListSessions(ctx context.Context, tenantUserID string, limit, offset int) ([]*assistant.AIAssistantSession, error) {
    var sessions []*assistant.AIAssistantSession
    err := r.db.WithContext(ctx).
        Where("tenant_user_id = ?", tenantUserID).
        Order("updated_at DESC"). // 最近聊天优先
        Limit(limit).
        Offset(offset).
        Find(&sessions).Error
    return sessions, err
}
```

**租户隔离关键：**
```go
// 所有查询必须带 tenant_user_id
Where("tenant_user_id = ?", tenantUserID)
```

---

### 7.3 错误处理

**分层错误处理策略：**

```go
// Domain Layer - 返回业务错误
if session == nil {
    return nil, fmt.Errorf("session not found")
}

// Infrastructure Layer - 包装底层错误
if err := r.db.Create(session).Error; err != nil {
    return fmt.Errorf("failed to create session: %w", err)
}

// Application Layer - 转换为用户友好错误
if err != nil {
    return nil, fmt.Errorf("无法创建会话")
}

// Interface Layer - 映射为 HTTP 状态码
if err != nil {
    if strings.Contains(err.Error(), "not found") {
        back.Error(c, xerr.NotFound, "会话不存在")
    } else {
        back.Error(c, xerr.InternalServerError, "服务器错误")
    }
}
```

---

### 7.4 JSON 字段处理

**Citations 和 Tokens 存储为 JSON：**

```go
// 保存时序列化
citationsJSON := "{}"
if len(st.Citations) > 0 {
    if b, err := json.Marshal(st.Citations); err == nil {
        citationsJSON = string(b)
    }
}

// 读取时反序列化（Service 层）
var citations []respond.CitationEntry
if msg.CitationsJson != "" {
    json.Unmarshal([]byte(msg.CitationsJson), &citations)
}
```

**为什么使用 JSON 字段？**
- 灵活的 Schema（未来可扩展字段）
- 避免多表 JOIN（性能考虑）
- Citations 结构可能频繁变化

---

## 8. 关键设计决策

### 8.1 为什么在 Infrastructure 层实现 Pipeline？

**错误做法：**
```go
// ❌ 在 Service 层编排复杂逻辑
func (s *assistantServiceImpl) Chat(...) {
    // 加载历史
    messages := s.messageRepo.ListRecentMessages(...)
    // RAG 检索
    chunks := s.ragService.Retrieve(...)
    // 构建 Prompt
    prompt := buildPrompt(messages, chunks, ...)
    // 调用 LLM
    answer := s.llmClient.Generate(prompt)
    // 保存消息
    s.messageRepo.SaveMessage(...)
}
```

**正确做法：**
```go
// ✅ Service 层只调用 Pipeline
func (s *assistantServiceImpl) Chat(...) {
    result, err := s.pipeline.Execute(ctx, req)
    return convertToRespond(result), err
}

// ✅ Pipeline 在 Infrastructure 层实现
// infrastructure/pipeline/assistant_pipeline.go
```

**理由：**
- **DDD 原则**：Application 层编排业务用例，Infrastructure 层实现技术细节
- **复杂度隔离**：Eino Graph 属于技术实现，不应污染业务层
- **可测试性**：Pipeline 可独立测试，Service 仅测试协调逻辑

---

### 8.2 为什么流式和非流式分离？

**两种模式对比：**

| 特性 | 非流式 (Invoke) | 流式 (Stream) |
|------|----------------|---------------|
| **用户体验** | 等待完整响应 | 实时看到输出 |
| **实现复杂度** | 简单（同步） | 复杂（异步 + Channel） |
| **适用场景** | 简短问答 | 长文本生成 |
| **网络占用** | 单次请求/响应 | 持续连接 |

**设计选择：**
- 提供两个 API，让前端选择
- 流式模式需要手动执行节点（无法用 Graph.Invoke）
- 流式结果在 Service 层处理（避免 Pipeline 耦合 Channel）

---

### 8.3 为什么使用 Channel 传递流式事件？

**Go Channel 优势：**
```go
// 生产者（Service Layer）
eventChan := make(chan StreamEvent, 100) // 带缓冲
go func() {
    defer close(eventChan) // 自动关闭
    eventChan <- StreamEvent{Event: "delta", Data: token}
}()

// 消费者（HTTP Handler）
for event := range eventChan { // 自动退出循环
    c.SSEvent(event.Event, event.Data)
}
```

**对比其他方案：**
- **Callback 函数**：难以处理错误和退出
- **Iterator 接口**：Go 原生支持较弱
- **Channel**：惯用语法，类型安全，自动管理生命周期

---

### 8.4 Session ID 设计

**格式：** `AS` + 11位随机字符（总共 13-20 字符）

```go
util.GenerateID("AS") // → "AS1a2b3c4d5e6f7"
```

**为什么用前缀？**
- 快速识别类型（AS=Assistant Session, U=User, G=Group）
- 数据库索引友好（char(20) 定长）
- URL 安全（不包含特殊字符）

---

## 9. 性能优化

### 9.1 数据库优化

**索引策略：**
```sql
-- Session 表
KEY `idx_tenant_user_id` (`tenant_user_id`)      -- 租户隔离查询
KEY `idx_updated_at` (`updated_at`)              -- 会话列表排序

-- Message 表
KEY `idx_session_id` (`session_id`)              -- 会话消息查询
KEY `idx_created_at` (`created_at`)              -- 时间排序
```

**查询优化：**
```go
// 仅加载最近12条消息（6轮对话）
messageRepo.ListRecentMessages(ctx, sessionID, 12)

// 使用 LIMIT 避免扫描全表
.Order("created_at DESC").Limit(12)
```

---

### 9.2 RAG 检索优化

**参数调优：**
```go
req := &RetrieveRequest{
    TopK:           5,              // 召回数量（不宜过大）
    ScoreThreshold: 0.7,            // 过滤低分结果
    MaxContentChars: 200 * 5,       // 限制总字符数（避免超 LLM 窗口）
}
```

**缓存策略（未来扩展）：**
- 缓存 Embedding 结果（相同问题复用）
- 缓存 VectorSearch 结果（TTL 5分钟）

---

### 9.3 LLM 调用优化

**Timeout 设置：**
```toml
[aiConfig.chatModel]
timeoutSeconds = 120  # 避免长时间挂起
```

**流式输出优势：**
- 降低首 Token 延迟（用户感知更快）
- 减少客户端等待时间

---

## 10. 扩展性设计

### 10.1 多 Agent 支持

**当前实现：**
```go
// 请求中指定 agent_id
req := AssistantChatRequest{
    AgentID: "AG123...",
    Question: "...",
}

// BuildPrompt 节点加载 Agent 的 PersonaPrompt
if agentID != "" {
    agent := p.agentRepo.GetAgentByID(ctx, agentID)
    personaPrompt = agent.PersonaPrompt
}
```

**未来扩展：Agent Router**
```
User Question → Agent Router → 选择合适的 Agent → Execute Pipeline
```

---

### 10.2 Function Calling / Tools

**扩展方案：**
1. 在 `AIAgent.config_json` 中定义 Tools
2. 在 `ChatModel` 节点后添加 `ToolCall` 节点
3. 循环执行：`ChatModel → ToolCall → ChatModel` 直到完成

**Eino Graph 支持条件分支：**
```go
g.AddEdge(ChatModel, ToolCall, compose.Condition(func(st) bool {
    return st.HasToolCalls
}))
```

---

### 10.3 多租户隔离增强

**数据库分片（未来）：**
```go
// 根据 tenant_user_id 分库
dbShard := selectShard(tenantUserID)
repo := NewRepository(dbShard)
```

**Milvus 隔离：**
```go
// 使用 Partition 按租户隔离
partition := fmt.Sprintf("tenant_%s", tenantUserID)
vectorStore.Search(ctx, partition, vector, topK)
```

---

## 总结

### 核心要点回顾

1. **架构设计**
   - DDD 四层分离（Domain → Application → Infrastructure → Interface）
   - Eino Graph 编排复杂流程
   - Repository 抽象数据访问

2. **Pipeline 节点**
   - LoadMemory：会话管理
   - Retrieve：RAG 检索
   - BuildPrompt：Prompt 工程
   - ChatModel：LLM 调用
   - Persist：数据持久化

3. **关键技术**
   - Cloudwego Eino（Graph 编排）
   - GORM（ORM）
   - Milvus（向量检索）
   - Gin（HTTP 框架）
   - Go Channel（流式传输）

4. **设计原则**
   - 单一职责
   - 依赖注入
   - 接口抽象
   - 租户隔离

---

### 学习建议

1. **理解 Eino Graph**
   - 阅读 [Eino 官方文档](https://github.com/cloudwego/eino)
   - 尝试修改节点顺序，观察行为变化

2. **实践 DDD**
   - 识别各层职责
   - 尝试添加新 Repository 方法

3. **调试流式输出**
   - 在 Service 层打断点
   - 观察 StreamReader.Recv() 的数据流

4. **性能分析**
   - 使用 `pprof` 分析 CPU/内存
   - 优化数据库查询

---

### 参考资料

- [Cloudwego Eino 文档](https://github.com/cloudwego/eino)
- [GORM 官方文档](https://gorm.io/)
- [Milvus Go SDK](https://milvus.io/docs/install-go.md)
- [Gin 框架](https://gin-gonic.com/)
- [DDD 领域驱动设计](https://en.wikipedia.org/wiki/Domain-driven_design)

---

**文档版本：** v1.0  
**更新日期：** 2026-01-22  
**维护者：** OmniLink AI Team
