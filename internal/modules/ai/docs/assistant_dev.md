# 全局 AI 个人助手（Global AI Assistant）开发记录

## 项目概述

本文档记录了为 OmniLink 项目实现"全局 AI 个人助手（Global AI Assistant）"聊天功能的完整开发过程。

## 开发日期

2026-01-21

## 核心需求

1. 独立于 IM 的 AI 助手会话系统
2. 使用 Eino 框架编排 RAG + LLM Pipeline
3. 支持流式和非流式聊天
4. 会话历史管理
5. Agent 管理（预留扩展）
6. Citations 引用展示

## 一、后端实现

### 1.1 数据模型（已完成）

#### 新增三张表：

**ai_assistant_session** - AI助手会话表
- id: bigint, PK, auto increment
- session_id: char(20), unique index（对外使用）
- tenant_user_id: char(20), index
- title: varchar(64)
- status: tinyint (1=active, 0=archived)
- agent_id: char(20) nullable
- persona_id: char(20) nullable
- created_at, updated_at: datetime

**ai_assistant_message** - AI助手消息表
- id: bigint, PK, auto increment
- session_id: char(20), index
- role: varchar(16) (system/user/assistant)
- content: mediumtext
- citations_json: json
- tokens_json: json
- created_at: datetime

**ai_agent** - Agent管理表
- id: bigint, PK, auto increment
- agent_id: char(20), unique index
- owner_type: varchar(20)（user/system）
- owner_id: char(20)
- name: varchar(64)
- description: varchar(255)
- persona_prompt: mediumtext
- status: tinyint (1=enabled, 0=disabled)
- kb_type: varchar(30)（global / agent_private）
- kb_id: bigint
- tools_json: json
- created_at, updated_at: datetime

#### 文件位置：
- `internal/modules/ai/domain/assistant/entities.go`
- `internal/modules/ai/domain/agent/entities.go`

### 1.2 仓储层（已完成）

#### 仓储接口：
- `internal/modules/ai/domain/repository/assistant_repository.go`
- `internal/modules/ai/domain/repository/agent_repository.go`

#### 仓储实现：
- `internal/modules/ai/infrastructure/persistence/assistant_repository_impl.go`
- `internal/modules/ai/infrastructure/persistence/agent_repository_impl.go`

#### 核心方法：

**AssistantSessionRepository:**
- CreateSession - 创建新会话
- GetSessionByID - 获取会话（权限隔离）
- ListSessions - 获取会话列表
- UpdateSessionTitle - 更新标题
- UpdateSessionAgent - 更新绑定Agent
- UpdateSessionUpdatedAt - 更新时间戳

**AssistantMessageRepository:**
- SaveMessage - 保存消息
- ListRecentMessages - 获取最近N条消息
- CountSessionMessages - 统计消息数

**AgentRepository:**
- CreateAgent - 创建Agent
- GetAgentByID - 获取Agent（权限隔离）
- ListAgents - 获取Agent列表
- UpdateAgent - 更新Agent
- DisableAgent - 禁用Agent

### 1.3 数据库迁移（已完成）

在 `internal/initial/gorm.go` 中注册了新表：

```go
&aiAssistant.AIAssistantSession{},
&aiAssistant.AIAssistantMessage{},
&aiAgent.AIAgent{},
```

### 1.4 应用层 DTO（已完成）

#### 请求 DTO：
`internal/modules/ai/application/dto/request/assistant_request.go`
- AssistantChatRequest - 聊天请求
- AssistantSessionListRequest - 会话列表请求
- AssistantAgentListRequest - Agent列表请求

#### 响应 DTO：
`internal/modules/ai/application/dto/respond/assistant_respond.go`
- AssistantChatRespond - 聊天响应
- AssistantStreamDoneEvent - 流式完成事件
- CitationEntry - 引用条目
- TimingInfo - 耗时统计
- AssistantSessionItem/ListRespond - 会话列表
- AssistantAgentItem/ListRespond - Agent列表

### 1.5 核心服务层（待实现）

#### AssistantService - 核心编排服务

**文件位置：** `internal/modules/ai/application/service/assistant_service.go`

**核心功能：**

1. **Chat() - 非流式聊天**
   - 输入：question, session_id, top_k, scope, source_keys, agent_id
   - 输出：answer, citations, query_id, timing

2. **ChatStream() - 流式聊天（SSE）**
   - 输入：同上
   - 输出：SSE事件流
     - event: delta → data: { token: "..." }
     - event: done → data: { session_id, answer, citations, query_id, timing }

3. **ListSessions() - 获取会话列表**

4. **ListAgents() - 获取Agent列表**

#### Eino Graph 设计（5个节点）

**节点顺序：**
1. **LoadMemory** - 加载历史消息（最近6轮=12条）
2. **Retrieve** - RAG召回（复用RetrievePipeline）
3. **BuildPrompt** - 构建Prompt
   - System: "你是 OmniLink 的全局 AI 个人助手，回答必须基于用户权限内的聊天/联系人/群组信息。"
   - Memory: user/assistant轮次拼接
   - Retrieved Context: [chunk:ID] 标记
4. **ChatModel** - 调用LLM（复用provider.go）
5. **Persist** - 保存消息和token统计

**Graph状态结构：**
```go
type assistantState struct {
    Req           *AssistantRequest
    SessionID     string
    TenantUserID  string
    Messages      []*assistant.AIAssistantMessage  // 历史消息
    RetrievedCtx  []respond.RAGChunkHit           // 召回结果
    Prompt        []model.Message                  // 最终Prompt
    Answer        string                           // LLM回答
    Citations     []respond.CitationEntry          // 引用列表
    Tokens        TokenStats                       // Token统计
    QueryID       string
    Timing        TimingInfo
    Start         time.Time
    Err           error
}
```

**实现要点：**
- 复用现有 `RetrievePipeline` 进行检索
- 复用 `NewChatModelFromConfig` 初始化LLM
- StreamReader 处理流式输出
- 支持 SSE (Server-Sent Events)
- Citations 为空不报错
- IM 聊天功能不受影响

### 1.6 HTTP Handler（待实现）

**文件位置：** `internal/modules/ai/interface/http/assistant_handler.go`

**路由（需在 api/http/https_server.go 注册）：**
- POST /ai/assistant/chat - 非流式聊天
- POST /ai/assistant/chat/stream - 流式聊天（SSE）
- GET /ai/assistant/sessions - 获取会话列表
- GET /ai/assistant/agents - 获取Agent列表

**Handler实现要点：**
- 从 gin.Context 提取 tenant_user_id (uuid)
- SSE 响应头设置：
  ```go
  c.Header("Content-Type", "text/event-stream")
  c.Header("Cache-Control", "no-cache")
  c.Header("Connection", "keep-alive")
  ```
- SSE 事件格式：
  ```
  event: delta
  data: {"token":"..."}
  
  event: done
  data: {"session_id":"...","answer":"...","citations":[...],...}
  ```

### 1.7 路由注册（待实现）

在 `api/http/https_server.go` 的 `authed` 分组下添加：

```go
if aiAssistantH != nil {
    authed.POST("/ai/assistant/chat", aiAssistantH.Chat)
    authed.POST("/ai/assistant/chat/stream", aiAssistantH.ChatStream)
    authed.GET("/ai/assistant/sessions", aiAssistantH.ListSessions)
    authed.GET("/ai/assistant/agents", aiAssistantH.ListAgents)
}
```

## 二、前端实现（待实现）

### 2.1 路由配置

**文件：** `web/src/router/index.js`

```javascript
{
  path: '/assistant',
  name: 'Assistant',
  component: () => import('../views/Assistant.vue'),
  meta: { requiresAuth: true }
}
```

### 2.2 SideBar 导航入口

**文件：** `web/src/components/chat/SideBar.vue`

在现有导航项中添加AI助手入口：

```vue
<div 
  class="nav-item" 
  :class="{ active: activeTab === 'assistant' }"
  @click="$router.push('/assistant')"
>
  <el-icon><MagicStick /></el-icon>
</div>
```

### 2.3 Assistant.vue 主页面

**文件：** `web/src/views/Assistant.vue`

**布局结构：**
```
┌─────────────┬────────────────────────────┐
│             │  Agent 选择（顶部）          │
│  会话列表    ├────────────────────────────┤
│  (左侧)     │  聊天窗口（中间）            │
│             ├────────────────────────────┤
│             │  Citations（底部，可折叠）   │
└─────────────┴────────────────────────────┘
```

**核心功能：**
1. 会话列表展示（左侧）
   - 按updated_at倒序
   - 显示title
   - 点击切换会话
   - 新建会话按钮

2. 聊天窗口（右侧）
   - 消息列表（user/assistant）
   - 输入框（Enter发送，Shift+Enter换行）
   - "AI正在思考"加载状态
   - SSE流式追加token

3. Citations区域
   - 可折叠卡片列表
   - 展示chunk_id、source_type、source_key、score、content摘要

**样式要求：**
- 复用 `glass-panel` 风格
- AI会话列表颜色区分（浅紫/浅蓝渐变）
- Citations卡片：圆角卡片 + hover效果

### 2.4 SSE 客户端实现

```javascript
const eventSource = new EventSource('/ai/assistant/chat/stream', {
  // POST请求需要通过fetch + ReadableStream处理
})

eventSource.addEventListener('delta', (e) => {
  const data = JSON.parse(e.data)
  currentAnswer += data.token
  // 追加到UI
})

eventSource.addEventListener('done', (e) => {
  const data = JSON.parse(e.data)
  // 更新citations、queryID、timing
  eventSource.close()
})
```

**注意：** SSE标准不支持POST，需使用 fetch + ReadableStream + EventSource polyfill 或直接解析流。

## 三、技术栈

### 后端
- Go 1.25
- Gin (HTTP框架)
- GORM (ORM)
- Eino (AI编排框架)
  - cloudwego/eino
  - cloudwego/eino-ext (ChatModel扩展)
- Milvus (向量数据库)

### 前端
- Vue 3
- Vite
- Element Plus
- Vue Router
- EventSource (SSE)

## 四、关键设计决策

### 4.1 为什么独立于 IM？

- **需求隔离：** AI助手是全局知识检索，IM是点对点/群组通信
- **数据模型：** Assistant需要citations、tokens等AI特有字段
- **扩展性：** 预留Agent、Persona、Tools等AI特性
- **性能：** 避免IM表过度膨胀

### 4.2 为什么使用 Eino Graph？

- **可组合：** 节点可插拔（未来可插入ToolRouter）
- **可观测：** 每个节点耗时独立统计
- **可复用：** Retrieve节点直接复用RetrievePipeline
- **流式支持：** Eino原生支持StreamReader

### 4.3 Session ID 生成策略

```go
util.GenerateID("AS")  // AS + 11位随机字符
```

与现有ID体系一致：
- U: User
- G: Group
- S: IM Session
- M: Message
- A: Apply
- AS: Assistant Session (新增)

### 4.4 权限隔离

所有查询必须包含 `tenant_user_id` 过滤：

```go
Where("session_id = ? AND tenant_user_id = ?", sessionId, tenantUserId)
```

防止越权访问其他用户的会话。

## 五、扩展性预留

### 5.1 Agent 支持

- Agent表已创建
- Session表预留 `agent_id` 字段
- BuildPrompt节点可根据agent_id动态加载persona_prompt

### 5.2 Tool 调用（MCP）

- Agent表预留 `tools_json` 字段
- Graph可在Retrieve和BuildPrompt之间插入ToolRouter节点

### 5.3 Persona 定制

- Session表预留 `persona_id` 字段
- 可关联独立的persona表

## 六、测试要点

### 后端测试

1. **数据库迁移**
   ```bash
   go run ./cmd/OmniLink/main.go
   # 检查三张表是否创建成功
   ```

2. **非流式聊天**
   ```bash
   curl -X POST http://localhost:8000/ai/assistant/chat \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"question":"你好","top_k":5}'
   ```

3. **流式聊天**
   ```bash
   curl -X POST http://localhost:8000/ai/assistant/chat/stream \
     -H "Authorization: Bearer YOUR_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"question":"介绍一下这个项目"}'
   ```

4. **会话列表**
   ```bash
   curl http://localhost:8000/ai/assistant/sessions \
     -H "Authorization: Bearer YOUR_TOKEN"
   ```

### 前端测试

1. 新建会话 → 发送消息 → 查看流式输出
2. 切换会话 → 验证历史加载
3. 查看Citations → 验证引用展示
4. 刷新页面 → 验证会话持久化

## 七、已完成项

- [x] 创建领域实体（Assistant、Agent）
- [x] 创建仓储接口
- [x] 实现仓储实现
- [x] 注册数据库表
- [x] 创建HTTP DTO

## 八、待实现项

- [ ] AssistantService核心服务（Eino Graph）
- [ ] Assistant HTTP Handler
- [ ] 路由注册
- [ ] 前端路由配置
- [ ] Assistant.vue主页面
- [ ] SideBar导航入口
- [ ] SSE流式接收

## 九、代码参考

### ID生成
```go
// 会话ID
sessionID := util.GenerateID("AS")

// Agent ID
agentID := util.GenerateID("AG")
```

### Eino Graph 基本结构
```go
type Pipeline struct {
    r compose.Runnable[*Request, *Result]
}

func (p *Pipeline) buildGraph(ctx context.Context) (compose.Runnable[*Request, *Result], error) {
    g := compose.NewGraph[*Request, *Result]()
    
    _ = g.AddLambdaNode("Node1", compose.InvokableLambdaWithOption(p.node1), compose.WithNodeName("Node1"))
    _ = g.AddLambdaNode("Node2", compose.InvokableLambdaWithOption(p.node2), compose.WithNodeName("Node2"))
    
    _ = g.AddEdge(compose.START, "Node1")
    _ = g.AddEdge("Node1", "Node2")
    _ = g.AddEdge("Node2", compose.END)
    
    return g.Compile(ctx, compose.WithGraphName("PipelineName"))
}
```

### SSE响应
```go
func (h *Handler) Stream(c *gin.Context) {
    c.Header("Content-Type", "text/event-stream")
    c.Header("Cache-Control", "no-cache")
    c.Header("Connection", "keep-alive")
    
    for token := range tokenChan {
        c.SSEvent("delta", map[string]string{"token": token})
        c.Writer.Flush()
    }
    
    c.SSEvent("done", doneData)
    c.Writer.Flush()
}
```

## 十、常见问题

**Q: Citations为空怎么办？**
A: 不报错，返回空数组。用户可能在提问通用问题或知识库尚未回填。

**Q: 流式输出中断怎么办？**
A: 客户端需处理EventSource onerror事件，重连或提示用户。

**Q: IM聊天会受影响吗？**
A: 不会。Assistant使用独立的表和路由，与IM完全隔离。

**Q: 如何限制用户访问其他人的会话？**
A: 所有查询都带 `tenant_user_id` 过滤，数据库层面隔离。

## 十一、参考文档

- Eino官方文档: https://cloudwego.io/docs/eino/
- Gin SSE示例: https://github.com/gin-gonic/examples/tree/master/server-sent-event
- MDN EventSource: https://developer.mozilla.org/en-US/docs/Web/API/EventSource

## 十二、后续优化方向

1. **性能优化**
   - Redis缓存会话列表
   - 消息分页加载
   - 长文本截断

2. **功能增强**
   - 多轮对话上下文压缩
   - Agent Marketplace
   - 自定义Persona
   - MCP Tool调用

3. **用户体验**
   - 打字指示器
   - 消息重新生成
   - 引用跳转到原始聊天
   - 会话导出

---

**文档版本：** v1.0  
**最后更新：** 2026-01-21  
**维护者：** OmniLink AI Team
