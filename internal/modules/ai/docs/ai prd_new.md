AI-Native 即时通讯平台 - 核心功能架构文档 (v2.0)
1. 项目背景与目标
当前状态：基于 Golang 的 IM 系统，具备单聊、群聊、好友关系等基础能力。

升级目标：从“工具型 IM”转型为“智能型 IM”。不只是加个机器人，而是让 AI 介入消息的生产、分发、展示全流程。

核心理念：

AI Native：AI 是第一公民，而非外挂插件。

Privacy First：严格区分“私有数据（仅我看）”与“公有数据（群组共享）”的权限边界。

2. 核心功能需求 (Functional Requirements)
2.1 模块一：全局 AI 个人助手 (Global Copilot)
定义：用户的主动交互入口，拥有该用户的完整数据权限（L1 级实时响应）。

核心能力：

全域 RAG (Retrieval-Augmented Generation)：

场景：用户问“我上周答应李四什么事了？”，AI 检索（私聊+群聊）历史记录回答。

离线总结：用户登录时，自动推送一条“离线期间重点消息摘要”。

主动通知：作为系统的“嘴巴”，负责发送提醒、日报摘要。

Agent Action (MCP 架构)：

场景：用户指令“帮我把张三拉进 Golang 群”，AI 将自然语言转为 API 调用。

扩展性：通过 OmniLink (MCP Client) 连接外部服务（GitHub, Calendar）。

🛠 技术实现提示：

Session 管理：这是一个 Stateful（有状态）的长连接会话。

MCP 实现：后端需实现一个 ToolManager，将 IM 内部 API（如 InviteUser, SendMessage）封装为 OpenAI 兼容的 Function Calling 格式。

RAG 边界：检索范围 = PrivateChats + JoinedGroupChats。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：YES。这是系统的核心 Stateful Agent，维护长连接会话。

独立 RAG?：YES (Master Scope)。拥有该用户最高级别的向量检索权限（All Private Chats + All Joined Group Chats）。

🔗 模块联动：

联动源：它是 模块四 (智能指令) 的执行端。

逻辑：当模块四的任务（如定时提醒）触发时，调度器会回调本模块，由本模块通过“助手对话框”向用户发送 Push Notification。

2.2 模块二：自定义 AI Agent 工厂 (User Sandbox)
定义：用户创建的独立 AI 实体，用于娱乐或特定任务（L1 级独立实例）。

核心能力：

Persona 配置：自定义 Prompt（如“严厉的面试官”）。

数字替身 (Mimicry)：

功能：读取与某好友的聊天记录，微调或 In-context Learning 模仿其语气。

私有知识库 (Private KB)：

功能：上传 PDF/MD，AI 仅基于文档回答。

🛠 技术实现提示：

向量隔离：必须在向量数据库（如 Milvus/pgvector）中通过 Metadata 严格隔离不同 Agent 的 Knowledge Base，防止串号。

推理成本：对于高频对话的 Agent，考虑使用小参数模型（如 Llama-3-8B）以降低成本。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：YES。每个自定义 Agent 都是一个独立的逻辑实例（Instance），拥有独立的 System Prompt 和 Context Window。

独立 RAG?：YES (Isolated Scope)。

必须实现物理隔离或逻辑强隔离（Metadata Filtering）。

Agent A 的知识库绝对不能被 Agent B 检索到。

🔗 模块联动：

无直接联动。它们是独立的沙箱，通常不与其他模块交互，防止逻辑混乱。

2.3 模块三：AI 微服务/小工具 (UI Enhancers)
定义：嵌入在前端交互流程中的无感 AI，低延迟要求（L1 级轻量服务）。

核心能力：

智能输入辅助：

补全：根据输入预测后半句。

润色：提供“更礼貌”、“翻译”、“扩写”三个快捷按钮。

信息降噪：

功能：群消息积压 > 50 条时，自动生成浮层摘要。

🛠 技术实现提示：

延迟敏感：建议使用专用的小模型 API，不要排队等待主 Copilot 的推理资源。

交互方式：WebSocket 流式传输，前端防抖（Debounce）触发。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：NO。

这是 Stateless LLM API Calls（无状态调用）。不需要维护对话历史，请求一次返回一次。

模型建议：使用专用的低延迟小模型（如 7B/8B 量化模型）。

独立 RAG?：NO。

输入：当前输入框的文本 + 最近 10 条消息（Context）。

不需要检索向量数据库。

🔗 模块联动：

前端直接触发。不经过后端复杂路由。

2.4 模块四：智能指令系统 (Command Router)
定义：基于 / 的快速意图触发器，本质是模块一的快捷入口。

核心能力：

触发与解析：输入框输入 / 弹起菜单，支持自然语言参数（如 /todo 明早10点开会）。

任务调度：解析时间与意图，通过 MCP 调用日历/提醒服务。

🛠 技术实现提示：

路由逻辑：前端解析 / -> 后端识别 Command -> 复用模块一的 Function Calling 逻辑 -> 执行 MCP Tool。不要单独写一套 NLP 逻辑。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：NO。

这是一个 NLP Router / Parser。它负责“理解意图”，不需要维护对话状态。

独立 RAG?：NO。

🔗 模块联动 (关键)：

联动目标 -> 模块一 (全局助手)

流程：

用户输入 /todo 明早10点开会。

本模块解析出 Task: 开会, Time: 10:00 AM。

本模块调用 Scheduler Service 注册 Cron Job。

联动：时间一到，Job 触发，调用 模块一 的接口，以“全局助手”的身份给用户发一条消息：“🔔 提醒：您该去开会了”。

2.5 模块五：动态上下文画布 (Generative UI)
定义：打破气泡限制，AI 根据意图动态渲染 React/Flutter 组件。

核心能力：

生成式界面 (GenUI)：

场景：讨论“去哪吃饭” -> AI 识别 -> 下发 JSON -> 前端渲染为【投票卡片】或【地图标记】。

协作白板 (Infinite Whiteboard)：

场景：输入 /brainstorm，聊天背景变暗，消息自动转化为白板上的便利贴，AI 负责聚类。

🛠 技术实现提示：

协议约定：前后端需约定一套 RenderProtocol。

示例： json { "type": "widget", "component": "vote_card", "data": { "options": [...] } }

状态同步：白板上的操作需通过 WebSocket 广播给群内所有成员。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：NO。

这是一种 Capability (能力)，依附于 模块一 或 模块六。

独立 RAG?：NO。

🔗 模块联动：

依附于对话流。

当 模块一 (助手) 或 模块六 (群Bot) 识别到特定意图（Intent = Voting/Map）时，输出特定格式的 JSON，前端识别后渲染此画布。

2.6 模块六：群组智能协作 (Group Moderator) - 【公有/共享】
定义：服务于群组所有人的公共实体，客观中立（L3 级群组服务）。

核心能力：

群组维基 (Group Wiki / Live Pin)：

功能：AI 监听群聊，识别“结论性信息”（如会议时间、项目状态），自动更新群侧边栏的公告区域。

权限：所有群成员可见。内容更新需（可选）管理员确认。

话题分流 (Topic Threading)：

功能：检测到群内话题分裂（如一部分人聊代码，一部分人聊游戏），建议创建临时子频道。

社交润滑 (Atmosphere)：

功能：检测到全员“哈哈哈哈”时，自动发送表情包或特效。

🛠 技术实现提示：

流式缓冲：不要每条消息触发 LLM。设置 Buffer（如每 20 条消息或每 5 分钟）进行一次 Batch 处理，提取 Facts。

广播机制：Wiki 更新后，通过 WebSocket 推送 GroupStateUpdate 事件给所有群成员。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：YES (System Bot)。

每个群一个逻辑实例。

注意：为了省钱，通常不是实时在线，而是基于 Event Trigger（事件触发）或 Batch Processing（批处理）。

独立 RAG?：YES (Group Scope)。

检索边界：严格限制为当前群组的历史记录。严禁跨群检索。

🔗 模块联动：

联动 -> 模块一 (全局助手)：

场景：如果群助手在群里总结出“@张三 需要提交代码”，它不仅会更新群维基，还会触发 张三的模块一，私聊提醒他：“群助手的总结中提到了你的任务...”。

2.7 模块七：动态 AI 档案与关系管理 (Personal Analyst) - 【私有/独享】
定义：服务于单个用户的后台分析服务，通过异步分析生成私有情报（L2 级异步服务）。

核心能力：

动态关系备注 (Private AI Note)：

交互：点击某人头像（好友或群友） -> 展示 AI 生成的备注卡片（仅自己可见）。

内容：

待办/承诺：“上次他答应发接口文档 (12-05)”。

话题雷达：“他最近在 Golang 群经常讨论 Eino 框架”。

情感温度：“上次沟通有些急躁，建议轻松开场”。

离线数字分身 (Offline Avatar)：

功能：用户下线后，授权 Agent 进行有限回复（如查询文档链接），并必须标注 [AI自动回复]。

🛠 技术实现提示：

数据边界 (Critical)：生成张三的画像时，RAG 检索范围 = PrivateChat(Me, 张三) + GroupChat(Shared_Groups)。严禁越权读取张三在其他群的发言。

异步处理：此功能不应阻塞主线程。建议使用消息队列（Kafka/Redis Stream），在后台慢慢生成画像并存入缓存。

离线托管：本质上是一个监听用户 Offline 状态的 Hook，激活一个只读权限的 RAG Agent。

⚙️ 架构规格 (Architecture Specs)

独立 Agent?：

动态备注：NO。这是 Background Job (后台任务)。

离线分身：YES。这是一个特殊的“托管 Agent”，仅在 User Status = Offline 时激活。

独立 RAG?：YES (Relationship Scope)。

检索边界 (最复杂)：Shared_Context(Me, Target)。即 {私聊记录} + {共同所在的群聊记录}。

🔗 模块联动：

联动 -> 前端 UI：

本模块分析生成的数据（如 tags, summary）会存入 User Profile DB。

前端加载用户资料卡时，读取这些数据进行渲染。

现在的项目的AI功能是：用户可以创建或选择已有agent，创建agent的话可以选择是global还是private，如果是global的话就用global的rag知识库，如果是private的话支持用户自己上传文档作为知识库，当然两种agent都支持用户级prompt，然后选完agent后可以基于agent来创建会话或者选择历史会话然后聊天。但回看C:\Users\chenjun\goProject\OmniLink\internal\modules\ai\docs\ai prd_new.md感觉还是走偏了，现在我想改成下面这个样子，首先用户注册完成后后台自动创建一个全局ai助手agent，用global rag知识库，然后有一个专门的助手会话，这个助手会话唯一、不可取消且置顶，用来实现ai prd_new.md里面的比如离线总结、主动通知等功能，当然用户也可以在助手会话里跟全局ai助手进行聊天、咨询、命令等。然后用户可以基于全局ai助手再创建会话进行聊天等（满足用户不希望会话之间历史对话这种上下文干扰）。再然后就是用户自定义agent，然后基于agent再来创建会话这个样子。我打算这个阶段先实现我说的这些功能（有些是需要在已有代码中修改），至于ai prd_new.md中的后面所有模块，那是未来的事，先不考虑，先把我说的一步一步落实。当然最重要的是考虑到后续与ai prd_new.md后续模块的兼容和扩展性。
前端要做的是将AI模块和IM模块合并，取消那个AI入口和专门的AI页面，全部整合进主页面，然后agent列表整合进IM会话列表，点击agent后会话窗口中新增一列显示历史会话和创建新对话入口，以及与ai聊天的会话窗口。然后创建agent之类的入口也融入到IM主页面。
你需要做的：浏览本项目已有的AI模块代码，写出一份详细的改动技术方案（包括前后端），如何改，怎么改，具体怎么改，一步一步的，要详细一点，然后中间穿插一步一步的给ai开发用的prompt。最重要的是考虑到后续与ai prd_new.md后续模块的兼容和扩展性。比如实现已有代码的时候考虑到后续模块可能会用到所以要留接口或者注释之类的，一定要注重扩展性和后续的兼容性。并且：我非常讨厌类似于“前期怎么怎么样，后期改成怎么怎么样”的过渡方案，必须都要一步到位！
在C:\Users\chenjun\goProject\OmniLink\internal\modules\ai\docs新建一个文档，严禁改动任何代码！！！严禁改动任何代码！！！严禁改动任何代码！！！严禁改动任何代码！！！你需要做的只是新建一个文档并写入改动技术方案。严禁改动任何代码！！！严禁改动任何代码！！！严禁改动任何代码！！！



我想和你探讨一下：目前打算实现C:\Users\chenjun\goProject\OmniLink\internal\modules\ai\docs\ai prd_new.md中模块一：全局 AI 个人助手的离线总结、主动通知功能，并且兼容后续的模块四 (智能指令)等等，比如当模块四的任务（如定时提醒）触发时，调度器会回调本模块，由本模块通过“助手对话框”向用户发送 Push Notification。然后我想把这几个功能做在一个系统里，或者共用一套逻辑，做成可复用可扩展的一套业务逻辑，不要离线总结是一个单独的功能，主动通知又是另外一个功能，这样后续每次新增类似的功能都要重新写一整套代码，所以我想的是做一个业务逻辑能够涵盖这些所有功能以及未来可能的扩展功能。
我目前的想法是开发一套AI-job系统，开发MCP接口让各个agent能够发布定时任务或者其他任务（自定义触发事件），然后任务的具体内容就是一个助理job-agent驱使其他AI来进行任务，可以指定agent，自己指定prompt。然后再新增一个ai向用户push message的mcp接口。
通过这样的系统，实现离线总结：这个功能是每个用户的全局 AI 个人助手都有，所以在给用户创建全局ai助手的时候实现：触发事件：用户每次登陆的时候且距离上次登陆间距xxx hours，助理job-agent发送给那个用户的全局AI个人助手，制定的prompt让那个用户的全局AI助手调用查询离线期间消息的mcp，然后组织语言进行总结，以及打招呼。
实现主动通知：用户打出/todo 明天七点去健身，该用户的全局 AI 个人助手agent进行意图识别，然后调用创建ai-job的mcp，创建一个触发事件为定时job，然后生成prompt：“调用push message的mcp去提醒用户去健身”，这种。然后到了特定时间，助理job-agent就会把这个prompt发送给该用户的全局 AI 个人助手agent。
请你看一下这样的设想合不合理，有没有必要，有没有什么样的意义，实现难度如何，不要修改和新建任何文件。