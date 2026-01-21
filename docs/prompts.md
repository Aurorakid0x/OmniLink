- 你是资深 Golang 工程师。项目在 c:\Users\chenjun\goProject\OmniLink ，遵循 DDD 分层（domain/application/infrastructure/interface）。我处于 Chat Mode（只读），你需要：先用检索/阅读梳理现有架构、相关模块依赖与代码风格；再给出改动方案；对每个已存在文件的修改必须先给出 diff 预览；新文件直接给完整内容。
- 需求：{把你的需求写这里，包含接口、字段、错误码、权限等}
- 约束：不引入新第三方库（除非项目已使用）；敏感信息不得出现在返回/日志；遵循现有 xerr/back/zlog 习惯；提供必要的单元测试或最小可运行验证步骤（命令以 Windows 为准）。





现在 OmniLink 中存在 {描述 bug/性能问题/代码坏味道}。请先定位触发路径（从 interface→application→domain→infrastructure），给出最小改动方案并实现。要求：不改动公共行为（除非说明），新增逻辑要有测试或可复现步骤；所有修改按文件 diff 预览输出。

- 你在 Builder 模式，拥有写文件与运行命令能力。请在 c:\Users\chenjun\goProject\OmniLink 按 DDD 分层新增功能：{需求}。
- 必做：
- interface/http：新增 handler/路由（遵循现有 gin 风格）；
- application：新增 service 与 DTO request/response；
- domain：补齐 entity/repository interface（如需要）；
- infrastructure：实现 gorm repository；
- 鉴权：如果需要登录态，复用现有 jwt middleware；
- 验证：本地跑 go test ./... （或给出能跑通的最小命令），并提供一个 curl/Postman 示例（Windows 友好）。
- 约束：不引入新库（除非已存在）；敏感字段永不返回；错误处理用 xerr/back 。




- 你在 Builder 模式。前端在 c:\Users\chenjun\goProject\OmniLink\web （Vue 项目）。请实现功能：{需求，例如“好友详情弹窗/群成员搜索/会话列表增强”}。
- 必做：
- 先阅读现有组件与请求封装（如 web\src\utils\request.js 、 web\src\api\im.js ），沿用现有代码风格；
- 新增/修改组件（如 web\src\components\chat\... ）；
- 若需要路由/状态管理，按现有 router/store 写法接入；
- 用 npm install / npm run dev （Windows）跑起来并给出访问路径与验证步骤；
- 不新增依赖（除非项目已用）。


todo：
- 梳理项目
- 把现有同步 /ai/internal/rag/backfill 改成“创建 ai_backfill_job + 写入多条 ai_ingest_event （pending）”，由 Outbox Relay 投递 Kafka
- 实现 Kafka consumer worker：从 omnilink.ai.ingest 消费 event_id ，回查 DB 拿 payload，然后调用 reader + pipeline.Ingest，再更新 ai_ingest_event.status 和 ai_backfill_job 统计
- 

你是 OmniLink 项目的开发者，请先通读项目已有的目录架构和重要代码，实现“全局 AI 个人助手（Global AI Assistant）”聊天功能，使用 Eino 框架，输出必要的代码修改。所有关键设计细节必须按下述要求执行，不能省略或随意改动。

## 0) 项目背景与约束
- 后端：Go 1.25 + Gin + GORM + DDD 分层。
- 前端：Vue 3 + Vite + Element Plus（目录 web/）。
- 现有 RAG 召回链路已完成（Eino compose Graph），必须复用，不允许重写。
- ChatModel Provider 已封装在 internal/modules/ai/infrastructure/llm/provider.go 。
- 不允许新增 README / .md 文档文件。
## 1) 后端数据模型（必须新增，字段固定）
### 1.1 全局助手会话与消息（独立于 IM）
新增两张表，必须独立于 IM 的 session/message：

ai_assistant_session

- id: bigint, PK, auto increment
- session_id: char(20), unique index（对外使用）
- tenant_user_id: char(20), index
- title: varchar(64)
- status: tinyint (1=active, 0=archived)
- agent_id: char(20) nullable（后续 Agent 支持）
- persona_id: char(20) nullable（后续 persona 支持）
- created_at, updated_at: datetime
ai_assistant_message

- id: bigint, PK, auto increment
- session_id: char(20), index
- role: varchar(16) (system/user/assistant)
- content: mediumtext
- citations_json: json（本轮检索引用）
- tokens_json: json（prompt_tokens/answer_tokens/total_tokens）
- created_at: datetime
### 1.2 Agent 管理表（必须新增）
为后续扩展新增统一 Agent 管理表：

ai_agent

- id: bigint, PK, auto increment
- agent_id: char(20), unique index
- owner_type: varchar(20)（user/system）
- owner_id: char(20)（若为用户 agent）
- name: varchar(64)
- description: varchar(255)
- persona_prompt: mediumtext
- status: tinyint (1=enabled, 0=disabled)
- kb_type: varchar(30)（global / agent_private）
- kb_id: bigint
- tools_json: json（MCP 工具授权列表）
- created_at, updated_at: datetime
### 1.3 迁移注册
在 internal/initial/gorm.go 的 AutoMigrate 中注册：

- ai_assistant_session
- ai_assistant_message
- ai_agent
## 2) 后端领域与仓储（必须实现）
新增以下目录与结构（DDD 风格保持与现有 AI 模块一致）：

```
internal/modules/ai/domain/assistant/    // 会话与消息实
体（如可以新增entities文件，像RAG那样）
internal/modules/ai/domain/agent/    // Agent 实体（如可以新增entities文件，像RAG那样）
internal/modules/ai/domain/repository/   // 新增仓储接口（参考RAG）
internal/modules/ai/infrastructure/persistence/ // 仓储
实现（参考RAG）
```
### 必须提供的仓储接口方法
AssistantSessionRepository

- CreateSession(tenant_user_id, title, agent_id) -> session_id
- GetSessionByID(session_id, tenant_user_id)
- ListSessions(tenant_user_id, limit, offset)
- UpdateSessionTitle(session_id, title)
- UpdateSessionAgent(session_id, agent_id)
AssistantMessageRepository

- SaveMessage(session_id, role, content, citations_json, tokens_json)
- ListRecentMessages(session_id, limit)
AgentRepository

- CreateAgent(agent)
- GetAgentByID(agent_id, owner_id)
- ListAgents(owner_id, limit, offset)
- UpdateAgent(agent_id, fields...)
- DisableAgent(agent_id)
仓储实现风格对齐 rag_repository_impl.go 。

## 3) Assistant 服务编排（Eino）
新增 AssistantService（application/service），必须使用 Eino compose Graph：

固定节点顺序：

1. LoadMemory
2. Retrieve（复用现有 RetrievePipeline）
3. BuildPrompt
4. ChatModel
5. Persist
### LoadMemory
- 按 session_id 拉取最近 N 轮（默认 6 轮 = 12 条消息）
### Retrieve
- 复用 RetrievePipeline （保证 tenant_user_id + kb_id 过滤）
### BuildPrompt（规则固定）
- system persona 固定文本： <br/> “你是 OmniLink 的全局 AI 个人助手，回答必须基于用户权限内的聊天/联系人/群组信息。”
- memory 以 user/assistant 轮次拼接
- retrieved context 以 [chunk:ID] 标记
### Persist
- 保存 user 消息与 assistant 消息
- citations_json 存引用数组
- tokens_json 记录 token 统计
## 4) 后端 HTTP 接口（必须提供）
### 4.1 POST /ai/assistant/chat
请求

- session_id（可空，不传则创建）
- question（必填）
- top_k（默认 5）
- scope（global | chat_private | chat_group）
- source_keys（可选，限制检索范围）
- agent_id（可选，若传入则绑定该 agent）
响应

- session_id
- answer
- citations[]
- query_id
- timing（embedding/search/postprocess/llm）
### 4.2 POST /ai/assistant/chat/stream
- SSE 流式输出
- event: delta → data: { token: "..." }
- event: done → data: { session_id, answer, citations, query_id, timing }
### 4.3 GET /ai/assistant/sessions
- 返回 AI 会话列表（title, session_id, updated_at）
### 4.4 GET /ai/assistant/agents
- 返回当前用户的 agent 列表（agent_id, name, status）
## 5) 前端功能与交互（必须实现）
### 路由与入口
- SideBar 新增 AI 入口
- 新路由 /assistant
- 新页面 Assistant.vue
### 页面布局
- 左侧：AI 会话列表（独立于 IM）
- 右侧：聊天窗口 + 引用区域
- 顶部：可选 Agent 下拉选择（未来扩展）
### 交互规则
- Enter 发送
- Shift+Enter 换行
- 发送后显示“AI 正在思考”
- SSE 流式追加 token
- 完成后追加 citations 区域（可折叠）
### 样式要求
- 复用 glass-panel 风格
- AI 会话列表颜色区分（浅紫/浅蓝渐变）
- 引用卡片展示 chunk_id + 摘要 + source_type/source_key
## 6) 扩展性要求（必须预留）
- AssistantChain 必须可插入 ToolRouter（MCP）节点
- Agent 表与 session 中预留 agent_id/persona_id
- scope 只影响检索过滤，不改底层表结构
## 7) 验收标准
- 新建会话 → 问答 → 下一轮可读取历史
- citations 为空不报错
- 流式 SSE 正常输出
- IM 聊天功能不受影响
交付要求

- 只修改、新增必要文件
- 在开发完成后，在C:\Users\chenjun\goProject\OmniLink\internal\modules\ai\docs新增开发文档， 内包含修改点，开发过程等等记录，除此之外不许新增文档/README
- 可引入Eino相关依赖
- 风格保持项目一致
完成所有以上要求。