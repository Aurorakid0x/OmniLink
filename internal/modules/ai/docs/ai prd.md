1. 项目背景与目标
当前状态：项目是一个基于 Golang 的 IM 系统，已具备私聊、群聊、好友管理等基础通讯功能。 升级目标：在现有架构基础上，深度集成 AI 能力，打造一个 "AI-Native" 的即时通讯平台。不仅仅是简单的对话，而是包括 全能助手、自定义 Agent、即时辅助工具、智能指令 四大核心板块。

2. 核心功能需求 (Functional Requirements)
2.1 模块一：全局 AI 个人助手 (Global AI Assistant)
定义：每个用户专属的超级管家，拥有该用户的所有上帝视角（权限内）。

核心能力：

全域 RAG (Retrieval-Augmented Generation)：

数据源：好友列表、群组列表、所有历史聊天记录（私聊+群聊）。

场景：用户提问“我对张三在1月2号说了什么？总结一下”、“我是否向李四承诺过某事？”，AI 需检索历史记录并精准回答。

Agent Action (Function Calling)：

能力：通过自然语言调用 IM 内部 API。

场景：用户指令“帮我给王五发个消息说我不去了”、“帮我加入‘Golang交流群’”，AI 自动执行操作。

技术实现（MCP）：采用 Model Context Protocol (MCP) 架构。OmniLink 作为一个 MCP Client，连接内置或外部的 MCP Servers（如 IM-Action-Server）。

扩展性：通过 MCP 协议，未来可零代码修改接入 GitHub、Notion、Google Calendar 等外部工具。

2.2 模块二：自定义 AI Agent 工厂 (Customizable AI Agents)
定义：用户创建和配置的独立 AI 实体。

核心能力：

角色扮演 (Persona Configuration)：用户可配置提示词 (System Prompt)，例如“萌妹子语气”、“严厉的老师”。

数字替身 (Digital Twin)：

模仿学习：支持读取与特定好友的聊天记录，分析语言风格，创建一个模仿该好友语气的 AI。

私有知识库 (Personal Knowledge Base)：

用户上传文档 (PDF/MD/TXT)，系统自动进行切片 (Chunking) 和 向量化 (Embedding) 存储。

该 Agent 聊天时仅基于该知识库回答问题。

2.3 模块三：AI 微服务/小工具 (AI Micro-Utilities)
定义：嵌入在 UI 交互流程中的轻量级 AI 功能，无处不在。

核心能力：

智能输入辅助：

消息补全：根据当前输入内容，预测后续文本。

输入润色：提供三个快捷操作——“更礼貌”、“翻译成英文”、“内容扩写”。

信息降噪：

群聊摘要：当群聊未读消息超过阈值（如 50 条），自动触发“AI 总结”，生成摘要展示给用户。

2.4 模块四：智能指令系统 (Smart Command System)
定义：基于 / 触发的快速意图识别与任务分发系统。

核心能力：

触发机制：在任意会话输入框输入 / 弹起智能菜单。

NLP 解析：支持自然语言参数。例如 /todo 明早10点提醒我开会。

任务调度：AI 解析时间与意图，通过 MCP 调用日历/提醒服务。任务触发时，由 模块一（全局助手） 进行消息推送提醒。

3. 非功能性需求 (Non-functional Requirements)
架构一致性：AI 模块必须融入现有的 Golang 项目目录结构，避免破坏原有代码的整洁性。

高可扩展性 (Scalability)：

AI 服务层需设计为接口模式（Interface），支持切换底层模型（OpenAI, Claude, DeepSeek, 本地 Ollama 等）。

向量数据库需考虑未来数据量膨胀，设计合理的索引结构。

响应速度：输入辅助类功能（模块三）要求极低延迟，需考虑流式输出 (Streaming) 设计。

数据隐私：向量化存储的数据需包含 user_id 隔离，确保用户只能检索到自己的数据。

---

4. 全局 AI 个人助手：全域 RAG 落地路线图（开发用工作文档）

4.1 目标与边界
目标：实现「全局 AI 个人助手」中的“全域 RAG”，让用户可以基于“权限内”的全量数据（好友/群组/历史聊天）进行语义检索并回答。
边界：本阶段只做 RAG 基建闭环（数据入库 + 检索 + 生成回答），不做异步消费、群聊摘要自动触发、智能指令、工具调用等模块；但必须为它们预留扩展点，不回头删改已完成的代码。

4.2 当前项目进度（已完成）
你已经完成了 AI 模块落地的最关键三件事：
- AI 模块目录结构：按领域/应用/基础设施分层，后续功能可按模块新增文件，不需要改旧代码。
- MySQL 表结构 + GORM AutoMigrate：已为知识库、数据源、chunk、向量记录、入库事件等建好了核心表（ai_knowledge_*、ai_vector_record、ai_ingest_event 等）。
- Milvus 基建：已具备 Milvus 初始化（连接/集合/索引）与向量写入/检索的 store（MilvusStore），并开始做 Eino 的最小适配（后续可用 Eino 编排）。

4.3 设计原则（保证扩展性，不回头删改）
必须长期坚持的“结构约束”，参考 SuperBizAgent 的思路：
- 上层只依赖接口：Application/Pipeline 只依赖 domain 定义的接口（Embedder、VectorStore、Chunker、SourceReader），不直接依赖 Milvus SDK / OpenAI SDK。
- 基建可替换：MilvusStore 是一种实现；未来要换 pgvector/ES/Weaviate，只新增实现文件与 wiring，不改 pipeline。
- 数据模型“稳定、可演进”：表结构保持不变或仅做向后兼容扩展字段；事件表（ai_ingest_event）提前预埋，为后续从同步切异步做准备。
- 多租户与权限隔离内建：向量记录必须带 tenant_user_id / kb_id / source_type/source_key 等过滤维度，检索时一律带过滤条件。
- 观测优先：埋点/日志/trace 是“功能的一部分”，否则 RAG 调优会失控。

4.4 全域 RAG 的最小闭环（你下一步要做什么）
把“全域 RAG”拆成两个可独立验收的闭环：入库闭环、检索闭环。

4.4.1 入库闭环（Ingestion）——把数据变成可检索的向量
输入：某个用户的历史消息 + 联系人/群组信息（权限内）。
输出：MySQL 里有可追溯的 chunk/记录；Milvus 里有对应向量；两者可关联。

你需要补齐的组件（按优先级）：
A) Source Reader（数据读取）
- 目标：把“聊天记录/联系人/群组”统一抽象成可入库的文档流。
- 要点：全域 RAG 不是只读 message，还要读“结构化列表”（好友/群组），但可先从 message 起步。
- 关键约束：读取时就要确定 tenant_user_id、source_type、source_key（例如：私聊=contact_id、群聊=group_id）。

B) Transformer（清洗/聚合）
- 目标：把碎片化对话整理成“语义完整”的 chunk。
- 为什么需要：聊天是一句一句的，单句 embedding 检索命中率很差；需要把若干轮对话合并成一个段落（例如按时间窗口、按 N 轮、按对话分隔符）。
- 这不是“额外需求”，它属于“生成 chunk”步骤的质量增强版。
- Eino 很擅长：Eino 的 Document Transformer 就是干这个的，你可以先用“最简单切分”跑通闭环，再把 Transformer 替换为“多轮聚合 + 再切分”的高级版本，上层不动。

C) Chunker（切片）
- 目标：把长文本切成可控大小（例如 200-500 tokens/几百字），并保留 overlap。
- 产物：每个 chunk 有 chunk_key、chunk_index、content_hash、metadata_json。

D) Embedder（向量化）
- 目标：把 chunk.Content -> 向量。
- 里程碑策略：
  1) 先用 Mock 向量打通入库/检索（你已接近完成）；
  2) 再接真实 embedding（OpenAI/DeepSeek/本地模型），但接口不变。
- 关键：向量维度 dim 必须与 Milvus collection 的 vectorDim 一致。

E) VectorStore（写入 Milvus）
- 目标：Upsert 向量并返回 vector_id。
- 要点：vector_id 必须稳定可复用（建议用可复现的 ID 规则：tenant + chunk_id + provider/model/version 组合）。

F) 元数据落库（写 MySQL）
- 目标：写 ai_knowledge_source、ai_knowledge_chunk、ai_vector_record。
- 要点：
  - chunk 表记录“文本与结构”；
  - vector_record 记录“向量存储状态、provider/model/dim、vector_id、collection”；
  - 任何错误要写 error_msg + embed_status，便于重试。

验收标准（入库闭环）：
- 随便选一个 user_id，跑一次“回填”，Milvus 里能查到该用户的向量数量增长；MySQL 里 chunk 与 vector_record 能对应起来。

4.4.2 检索闭环（Retrieval + Answer）——从向量命中到生成答案
输入：用户问题（query） + user_id（以及可选：会话范围/群范围）。
输出：答案 + 引用来源（至少包含命中的 chunk_id/来源信息），可用于前端展示“引用”。

你需要补齐的组件（按优先级）：
A) Query Embedder
- 目标：把 query -> 向量（同一个 embedding provider/model）。
- 关键：query 向量维度必须与存储一致。

B) Vector Retrieval
- 目标：用 Milvus 搜索 topK。
- 过滤表达式（必须）：tenant_user_id = 当前用户 AND（可选 kb_id / source_type / source_key 过滤）。
- 产物：SearchHit 列表（chunk_id、score、content、metadata）。

C) Context Builder（上下文构建）
- 目标：把命中的 chunks 组织成“可喂给 LLM 的上下文”。
- 要点：
  - 去重（同 chunk 多命中只保留一次）
  - 排序（按 score 或按时间）
  - 截断（控制 token 上限）
  - 保留引用信息（chunk_id/source_key）

D) LLM Answer（生成回答）
- 目标：把「用户问题 + 上下文」发送给 LLM，得到回答。
- 扩展性：未来的 Agent Action/工具调用会在这里接入，但现在只需要最简单的“问答”。

验收标准（检索闭环）：
- 提问一个你确定在历史消息出现过的信息（人名/日期/承诺），能返回包含引用的答案。

4.5 “回填”怎么做（同步触发，全量扫历史）
为什么需要：你现在的库里有大量历史消息，不回填就没有可检索数据。

建议做法（先同步，稳定后再异步）：
- 提供一个“内部 HTTP”或“临时命令”：输入 user_id + 可选时间范围，执行：
  1) 扫描该用户权限内的历史消息（分页）
  2) 每页/每批进入 Ingest Pipeline
  3) 记录进度与错误（便于断点续跑）

同步版本的好处：可控、易调试、快速验证。
异步版本（未来）：触发时只写 ai_ingest_event，由 worker 消费；表结构与 store 不需要改，这就是提前预埋 ai_ingest_event 的意义。

4.6 组件边界与推荐落点（目录层级建议）
建议把“变化快的策略”放在 application/pipeline，把“稳定的实体与接口”放在 domain，把“可替换的实现”放在 infrastructure：
- domain/rag：
  - 实体（已完成）
  - 接口：Embedder、VectorStore、Chunker、Transformer、SourceReader、Repository（下一步补齐）
- infrastructure：
  - embedding：provider_openai/provider_mock（实现 Embedder）
  - vectordb：milvus_store（已完成）+ eino adapter（已完成基础修复）
  - reader：chat_message_reader/contact_reader（读取数据源）
- application/service 或 pipeline：
  - ingest_pipeline：把 reader/transformer/chunker/embedder/store/repository 串起来
  - retrieve_pipeline：把 query embed + store.Search + context builder + llm 串起来

4.7 里程碑（建议按这 6 个小目标推进）
M1：跑通回填（Mock embedding）→ Milvus 可搜到结果（不要求准）
M2：接入真实 embedding（维度对齐、配置化）→ 结果开始“看起来像样”
M3：加入 Transformer（多轮聚合）→ 命中率显著提升
M4：检索回答输出引用（chunk_id/source_key）→ 前端可做“引用卡片”
M5：权限隔离与范围检索（按群/按联系人过滤）→ 满足“全域但权限内”
M6：从同步入库升级为事件异步（ai_ingest_event + worker）→ 用户体验与吞吐提升

4.8 常见踩坑（提前避坑）
- 维度不一致：embedding dim 与 Milvus collection dim 不一致会导致 Upsert/Search 直接失败。
- 过滤条件缺失：不带 tenant_user_id 过滤会造成数据越权，这是必须内建的安全约束。
- chunk 太碎或太大：太碎命中差，太大上下文塞不下；用 Transformer + 合理 chunker 迭代。
- 没有引用信息：回答再好也无法解释来源，后续无法做“可信度/可追溯”。

4.9 与后续 PRD 功能的衔接（保证不删改）
- Agent Action / 工具调用：将采用 MCP (Model Context Protocol) 架构。
  - 核心 IM 操作（发消息/加群）：封装为内置的 MCP Server（In-Process 或 独立进程均可）。
  - 外部扩展（日历/文件）：直接连接社区现成的 MCP Server。
  - 衔接点：在“LLM Answer”阶段，LLM 输出工具调用指令，由 MCP Client 转发给对应 Server。
- 自定义 Agent / 私有知识库：复用同一套 Ingest Pipeline，只是 SourceReader 从“消息”换成“用户上传文件”，KBId 切换为 agent 专属。
- 群聊摘要/输入辅助：复用 Query Embed + Retrieve，但会有更严格的延迟与流式输出要求，可在 application 层新增一个“轻量链路”，不动底层 store 与表结构。

4.10 MCP 架构落地指引（新增）
未来开发 MCP 功能时的部署形态选择：
1. 内置 MCP Server (推荐起步)：
   - 场景：操作 IM 内部数据（发消息、查联系人）。
   - 实现：在 OmniLink 单体进程内，启动一个 Goroutine 运行 MCP Server 逻辑，通过 stdio 或 pipe 与主程序（LLM Client）通信。
   - 优势：无 RPC 开销，部署简单（不需要额外部署微服务），开发体验像写本地函数。
2. 独立 MCP Server (进阶)：
   - 场景：耗资源任务（爬虫）、外部服务对接（GitHub/Notion）。
   - 实现：独立部署的 Go/Python 服务，OmniLink 通过网络连接它。
   - 优势：故障隔离，不拖累主进程；可复用社区现成 Docker 镜像。

4.11 下一次开发的“第一周任务清单”（照着做就不会晕）
- 任务 1：把“读取消息 → 生成 chunk → mock embedding → 写 Milvus + MySQL”串成一次可运行的 Ingest Pipeline（先只支持私聊消息）。
- 任务 2：做一个内部回填入口（HTTP/命令二选一），能对指定 user_id 扫历史消息分页入库。
- 任务 3：做检索接口：query → embedding → Milvus search → 返回命中 chunks（先不接 LLM）。
- 任务 4：接 LLM：把命中 chunks 拼 context，输出答案 + 引用信息。
- 任务 5：加入最小观测：每次回填/检索打印关键指标（耗时、topK、过滤条件、命中 chunk_id）。

（结束）

4.11 实施细化附录：创建哪些文件 / 改哪些文件 / 每一步怎么跑
本附录不改变 4.1-4.10 的任何结论，只把“落到你项目里要怎么写”说清楚，避免下次开发时反复迷路。

4.11.1 Wiring（入口与依赖注入）要改哪里
- 入口文件：[https_server.go](file:///c:/Users/chenjun/goProject/OmniLink/api/http/https_server.go)
  - 在 init() 内新增：AI 模块的 repo / service / handler 的构造与路由注册。
  - 依赖来源：MySQL 用 initial.GormDB；Milvus 用 initial.MilvusClient；配置用 config.GetConfig().MilvusConfig。
- 路由建议（先做内部能力，后续再对外开放）：
  - /ai/internal/rag/backfill：触发回填
  - /ai/rag/query：检索 + 返回命中 chunks（先不接 LLM 也行）

4.11.2 Task 1 的“读取消息”到底怎么读（读几条 / 什么时候读）
你现在 chat 模块已经有很成熟的“会话”和“消息”存储方式，RAG 读数据建议按“会话→分页消息”走，这样天然满足权限隔离。

- 读取入口（建议新增，不要改 chat 旧逻辑）：
  - 新建：c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\infrastructure\reader\chat_session_reader.go
  - 目的：把「用户能看到的所有会话」枚举出来，并把每个会话的消息按批读出来。

- 依赖复用（尽量用现成 repository，而不是直接写 SQL）：
  - 会话枚举：复用 chat 的 SessionRepository
    - 已有：ListUserSessionsBySendID / ListGroupSessionsBySendID（见 chat\infrastructure\persistence\session_repository_impl.go）
    - 用法：user_id 作为 send_id，拿到他所有私聊/群聊会话列表（这就是“权限内”）。
  - 消息分页：复用 chat 的 MessageRepository
    - 已有：ListPrivateMessages(userOneID,userTwoID,page,pageSize)、ListGroupMessages(groupID,page,pageSize)

- “读几条消息”：
  - 对回填：全量读（直到没有下一页）。
  - 对增量：只读最新（例如每个会话最近 N 条，或者 since last_ingested_at）。
  - pageSize 建议：200（chat service 里也做了最大 200 的限制，符合现状）。

- “什么时候读”：
  - 回填模式：收到 backfill 请求时立刻读（同步跑通优先）。
  - 线上增量（后续）：用户发消息成功后（SendPrivateMessage/SendGroupMessage）可以先同步入库，稳定后再改成写 ai_ingest_event 异步。

- “哪些消息要读”：
  - 先只读 Type=0 的文本消息（message.type==0），其他多媒体先跳过（否则你要处理 url/file 等内容抽取）。
  - 过滤空 content、过滤明显的系统提示（可从最简单做起：content.Trim()=="" 直接跳过）。

4.11.3 Task 1 的“生成 chunk”怎么做（先最小可用，再迭代）
建议把 chunk 生成分成 2 层：先聚合（Transformer），再切片（Chunker）。先跑通时可以只做 Chunker。

- 新建（最小版 chunker）：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\infrastructure\chunking\simple_chunker.go
  - 行为：按字符长度切（例如每 600-1200 字一段，overlap 50-100 字），保证 chunk 不会过长。

- 新建（可选，第二周再做的 transformer，多轮聚合）：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\infrastructure\transform\chat_turn_merger.go
  - 行为（推荐默认策略）：
    - 按 session（私聊/群聊）分组
    - 按时间窗口合并（例如 3-5 分钟内的连续消息合并成一个段落）
    - 段落里保留角色与时间（例如 “A(10:01):...\nB(10:02):...”）
  - 价值：解决“50万”这种缺上下文的碎片句子检索不到的问题。
  - 这一步属于 4.4.1 的 Transformer，不是额外需求；先不做也能跑通，但效果会差。

4.11.4 Task 1 的“Embedder 调用 + 写 Milvus + 写 MySQL”落在哪些文件
你要做的是一个“入库用例（UseCase）”，建议放在 application 层，由 infrastructure 提供能力。

- 新建：c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\application\pipeline\ingest_pipeline.go
  - 输入建议：tenant_user_id + source_type + source_key + 一批 messages（或聚合后的 docs）
  - 输出建议：写入的 chunk_id 列表 + vector_id 列表 + 统计信息（便于日志/回填进度展示）

- 需要新增/完善的仓储（MySQL 写表）：
  - 新建：c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\domain\rag\repository.go（接口）
  - 新建：c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\infrastructure\persistence\rag_repository_impl.go（实现，使用 gorm）
  - 典型写入顺序（推荐事务）：
    1) upsert/确保 ai_knowledge_base（owner_type=user, owner_id=tenant_user_id, kb_type=global）
    2) upsert ai_knowledge_source（kb_id + source_type + source_key + tenant_user_id）
    3) insert ai_knowledge_chunk（source_id + chunk_index + content + content_hash + metadata_json）
    4) embed chunk.Content（mock/真实 provider）
    5) MilvusStore.Upsert（UpsertItem 必填：ID、Vector、TenantUserID、KBID、SourceType、SourceKey、ChunkID、Content、MetadataJSON）
    6) insert ai_vector_record（chunk_id、vector_id、provider/model、dim、collection、embed_status、embedded_at）

- “UpsertItem 的 ID 从哪来”：
  - 必须稳定：建议用 “v_{tenantUserID}_{chunkID}_{provider}_{model}” 这种可复现规则（你文档 4.4.1E 已提到稳定 ID）。

4.11.5 Task 2：内部回填入口（HTTP/命令）具体改哪里
你项目当前是 Gin，所有路由都在 https_server.go 里注册，所以最省心的是做一个内部 HTTP。

- 新建 DTO：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\interface\http\dto\backfill_request.go
  - 字段建议：user_id（可选为空=用 token 的 uuid）、since、until、page_size、dry_run（只打印不写库）

- 新建 Handler：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\interface\http\rag_internal_handler.go
  - 行为：
    - 从 JWT 中取 uuid（保持和现有 handler 一致：c.GetString("uuid")）
    - 调用 BackfillService
    - 返回进度（会话数、消息数、chunk 数、错误数）

- 新建 Service：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\application\service\backfill_service.go
  - 回填策略（第一版就这么做，简单且可控）：
    1) 列出 user 的所有私聊 session（ListUserSessionsBySendID）和群聊 session（ListGroupSessionsBySendID）
    2) 对每个 session 分页拉消息（page=1..n, pageSize=200, 直到空）
    3) 每一页消息交给 ingest_pipeline（让 pipeline 负责：聚合/切片/写库/写 Milvus）

4.11.6 Task 3：检索接口具体改哪里（先返回 chunks，后接 LLM）
- 新建 DTO：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\interface\http\dto\rag_query_request.go
  - 字段建议：query、top_k、scope（all/contact/group）、source_key（可选）、kb_id（可选）

- 新建 Handler：
  - c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\interface\http\rag_query_handler.go
  - 行为：
    1) uuid := c.GetString("uuid")
    2) query embedding
    3) MilvusStore.Search(vector, topK, expr)
    4) 返回命中 chunks（content + chunk_id + source_type/source_key + score）

- 过滤 expr 怎么组（必须）：
  - tenant_user_id == uuid
  - 可选再加：source_type/source_key、kb_id

4.11.7 Task 4：接 LLM 的落点（现在只要知道挂哪就行）
- 新建 Service：c:\Users\chenjun\goProject\OmniLink\internal\modules\ai\application\service\rag_answer_service.go
- 行为：把 Task 3 的命中 chunks 拼成 context，再调用 LLM（先用一个最小实现返回模板也行），并把引用（chunk_id/source_key）原样带回。
- 扩展性保证：未来 Agent Action/工具调用就在这个 service 的“调用 LLM”位置扩展，不动入库/检索。

4.11.8 Task 5：最小观测应该在哪里打（否则很容易越写越晕）
- 建议在这三个地方打结构化日志（zlog）：
  - backfill_service：每个 session 的进度（第几页、读了多少条、累计 chunk 数）
  - ingest_pipeline：每批输入产生了多少 chunks、多少向量写入成功/失败
  - query_handler：topK、expr、命中数、耗时

4.11.9 下一次开发时的“最小落地顺序”（避免卡住）
- 先做 Task 2（回填入口）但先不真正写库：dry_run 只统计“会话数/消息数”确保读取没问题。
- 再做 Task 1（pipeline）先只写 MySQL chunk，不写 Milvus：确认 chunk 生成策略无 bug。
- 再把 mock embedding + Milvus upsert 接上：完成入库闭环。
- 最后做 Task 3：能检索出 chunks 就算胜利；LLM 生成回答放到最后做。



### 第一部分：全域 RAG 数据源扩展技术方案
你的目标是将结构化的“关系数据”（个人/好友/群组 Profile 与关系）转化为 RAG 可理解的“非结构化文档”。这需要从基础设施层到应用层进行系统化扩展。
 1. 基础设施层：完善数据库查询接口 (Infrastructure Layer)
为了让 RAG 能“看到”这些信息，必须先在各业务模块的 Repository 中暴露批量查询能力。

- 用户模块 (User Module)
  - 目标 ：获取自己或好友的详细 Profile（包括表结构中有，但是还没开放用户自己设置的个性签名）。
  - 扩展 UserInfoRepository ：
    - GetUserInfo(userID) : 获取单人完整信息（昵称、生日、签名、头像等）。
    - GetBatchUserInfo(userIDs) : 批量获取（用于群成员详情或好友列表详情，避免 N+1 查询）。
- 联系人模块 (Contact Module)
  - 目标 ：获取“我有多少好友”、“好友列表详情”。
  - 扩展 ContactRepository ：
    - ListContactsWithInfo(userID) : 获取某人的所有好友， 并联表查询 拿到好友的 UserInfo（昵称、签名等）。RAG 需要知道“昵称”，因为用户只会问“老张是谁”，而不会问“User123是谁”。
- 群组模块 (Group Module)
  - 目标 ：获取“我有多少群”、“群成员详情”。
  - 扩展 GroupRepository ：
    - ListJoinedGroups(userID) : 获取我加入的所有群（包括群名、公告、群主ID）。
    - GetGroupMembersWithInfo(groupID) : 获取某群的所有成员列表， 并联表查询 拿到成员的 UserInfo 和群内昵称。这是回答“群里有xxx吗？”的关键。 
    
2. 领域/基础设施层：新建专用 Reader (Domain/Infrastructure Layer)
在 internal/modules/ai/infrastructure/reader/ 下新增针对结构化数据的 Reader。它们的职责是将数据库对象（Struct）转换成自然语言描述（String/Document）。
- ContactProfileReader
  - 输入 ： tenant_user_id
  - 逻辑 ：
    1. 调用 ListContactsWithInfo 。
    2. 生成 Document ：
       - 策略 A（合并汇总） ：生成一篇“我的通讯录”文档。
         - 内容示例 ：“我的好友列表如下：1. 张三（备注：老张），个性签名：‘厚德载物’，生日：1990-01-01。2. 李四，个性签名：‘前端大神’...”
         - 适用场景 ：回答“我有多少好友”、“谁是做前端的”。
       - 策略 B（单人单档） ：每个好友生成一个 Document。
         - 内容示例 ：“好友详情：张三（备注：老张），UUID：U123，个性签名...”。
         - 适用场景 ：精准检索某人详情。
  - 建议 ： 策略 B 更好，利用元数据过滤更精准，且更新时只需重写单人。
- GroupProfileReader
  - 输入 ： tenant_user_id
  - 逻辑 ：
    1. 调用 ListJoinedGroups 。
    2. 针对每个群，调用 GetGroupMembersWithInfo 。
    3. 生成 Document （每个群一个文档）：
       - 内容示例 ：“群组档案：‘Golang交流群’（ID: G1001）。群公告：‘禁止发广告’。群主：王五。包含成员 50 人，活跃成员包括：赵六（签名：找工作）、钱七...”。
- SelfProfileReader
  - 输入 ： tenant_user_id
  - 逻辑 ：
    1. 调用 GetUserInfoWithoutPassword 。
    2. 生成 Document ：
       - 内容示例 ：“我的个人档案：昵称 ChenJun，生日 1995-05-20，个性签名‘Hello World’，注册时间 2024-01-01。” 3. 应用层：集成至 Ingest Service (Application Layer)
    3. 注意：需要对密码这种敏感信息进行脱敏


- 修改 IngestService.Backfill
  - 原有逻辑 ：只跑 SessionReader （聊天记录）。
  - 新增逻辑 ：
    - 并行（或顺序）调用 SelfProfileReader 、 ContactProfileReader 、 GroupProfileReader 。
    - 将它们生成的 Documents 同样送入 pipeline.Ingest 。
  - Pipeline 适配 ：
    - Pipeline 需要识别 SourceType 。
    - 如果 SourceType 是 contact_profile 或 group_profile ， Chunker 可能需要特殊配置（例如：不需要按 800 字切分，而是尽量保持完整，或者按条目切分）。目前的 RecursiveChunker 对这种结构化文本也适用（会按换行符切），所以暂时可以复用。
### 第二部分：RAG 复用与隔离策略（针对 PRD 愿景）
针对 PRD 中的其他 AI 功能（自定义 Agent、数字替身等）是否复用 RAG 及其隔离方案，我的建议是： “底层复用，逻辑隔离” 。
 1. 哪些功能会用到 RAG？
根据 PRD，以下功能强依赖 RAG：

- 全域 RAG（全局助手） ：查所有权限内数据。
- 自定义 Agent（私有知识库） ：用户上传 PDF/文档，Agent 基于此回答。
- 数字替身（模仿学习） ：检索特定好友的历史语料来模仿语气。 2. 复用策略：共用一套基建与表结构
不需要 为每个功能建一套新的向量库或 MySQL 表。目前的架构（ ai_knowledge_base / source / chunk / vector ）已经完全能够支撑。

- 复用点 ：
  - Milvus Collection ( ai_kb_vectors ) ：所有向量都存这里，不用建新表。
  - Pipeline ( IngestPipeline ) ：入库流程是一样的（读 -> 切 -> 存）。
  - Search Logic ：检索逻辑是一样的（Embedding -> Search -> Filter）。 3. 隔离策略：通过 KBId 和 SourceType 实现逻辑隔离
通过数据打标（Metadata）来实现业务上的隔离，而不是物理分库。

- 全域 RAG（全局助手）
  
  - KBType : global
  - KBId : 1 (假设每个用户有一个默认的 Global KB)
  - 检索范围 ： kb_id = 1 AND tenant_user_id = me
- 自定义 Agent（私有知识库）
  
  - 场景 ：你建了一个“法律顾问 Agent”，上传了《民法典.pdf》。
  - 存储 ：
    - KBType : agent_private
    - OwnerId : AgentUUID (或者 UserUUID 但标记为 Agent 专用)
    - KBId : 2 (新生成的 ID)
    - SourceType : file_upload
  - 检索 ：
    - 当用户跟这个 Agent 聊天时，检索条件强制加上： kb_id = 2 。
    - 这样它 只能 搜到《民法典》，绝对搜不到你的聊天记录或好友列表。
- 数字替身（模仿学习）
  
  - 场景 ：模仿“张三”的语气。
  - 存储 ：其实复用的是全域 RAG 里的聊天记录数据。
  - 检索 ：
    - 不需要重新入库。
    - 检索时加条件： kb_id = 1 AND source_type = chat_private AND source_key = 张三的UUID 。
    - 这样 AI 就只看到你和张三的对话，从而学习他的语气。 4. 结论：怎么整最好？
1. 坚持“大宽表”思想 ：Milvus 里只维护一个 ai_kb_vectors ，通过 kb_id 、 source_type 、 owner_id 区分一切。
2. Pipeline 保持通用 ：入库 Pipeline 不需要知道它是“全域助手”还是“法律 Agent”，它只管把 Document 变成 Vector。业务含义由 Request 中的 KBId 和 SourceType 决定。
3. Service 层做业务分发 ：
   - GlobalAssistantService ：调用 Search 时，查 KBType=global。
   - AgentService ：调用 Search 时，查 KBId=Agent对应的KBId。
这种方案维护成本最低，扩展性最强。未来如果要加“群聊摘要 RAG”，也只是加一种检索过滤条件而已。