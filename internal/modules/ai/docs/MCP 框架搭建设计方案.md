🎯 OmniLink MCP 框架搭建设计方案

一、整体架构设计

1.1 架构分层

┌─────────────────────────────────────────────────────────────┐
│                    AI Application Layer                     │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  AssistantService (已有)                              │  │
│  │  + MCPDispatcher (新增)                               │  │
│  └──────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                            ↓ 调用
┌─────────────────────────────────────────────────────────────┐
│              AI Infrastructure - MCP Layer                  │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ MCP Client   │  │ Server       │  │ Tool Router     │  │
│  │              │  │ Registry     │  │                 │  │
│  └──────────────┘  └──────────────┘  └─────────────────┘  │
└─────────────────────────────────────────────────────────────┘
       ↓                    ↓                      ↓
┌──────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ 内置 MCP     │  │ 外部 MCP Server  │  │ OmniLink Core    │
│ Server       │  │ (GitHub, 日历等) │  │ Services         │
│ (封装内部能力)│  │                  │  │ (好友/群/消息)    │
└──────────────┘  └──────────────────┘  └──────────────────┘
---
二、目录结构设计
2.1 新增目录结构
internal/modules/ai/
├── application/
│   └── service/
│       ├── assistant_service.go        (已有)
│       └── mcp_dispatcher.go           (新增) - MCP 调度器
│
├── infrastructure/
│   └── mcp/                            (新增整个目录)
│       ├── client/
│       │   ├── mcp_client.go           - MCP Client 核心实现
│       │   ├── session_manager.go      - Session 生命周期管理
│       │   ├── transport_stdio.go      - Stdio 传输层实现
│       │   └── transport_http.go       - HTTP/SSE 传输层实现
│       │
│       ├── server/
│       │   ├── builtin_server.go       - 内置 MCP Server 实现
│       │   ├── tool_registry.go        - 工具注册表
│       │   └── handlers/
│       │       ├── contact_handler.go  - 好友/联系人工具
│       │       ├── group_handler.go    - 群组工具
│       │       └── message_handler.go  - 消息工具
│       │
│       ├── registry/
│       │   ├── server_registry.go      - Server 注册与管理
│       │   └── server_config.go        - Server 配置加载
│       │
│       ├── router/
│       │   ├── tool_router.go          - 工具路由器
│       │   ├── tool_matcher.go         - 工具匹配策略
│       │   └── result_transformer.go   - 结果转换器
│       │
│       └── types/
│           ├── protocol.go             - MCP 协议类型定义
│           ├── tool.go                 - Tool 定义
│           └── errors.go               - MCP 错误定义
---

三、MCP 配置结构设计（TOML）
3.1 配置文件：configs/config.toml
[mcpConfig]
  # MCP 全局开关
  enabled = true
  
  # 调度策略
  dispatchStrategy = "priority"  # priority | round_robin | random
  
  # 超时配置
  toolCallTimeoutSeconds = 30
  serverInitTimeoutSeconds = 10
  
  # 内置 Server 配置
  [mcpConfig.builtinServer]
    enabled = true
    name = "omnilink-internal"
    version = "1.0.0"
    description = "OmniLink 内置能力封装 (好友、群、消息查询)"
    
    # 暴露的工具类别 (可按需开关)
    enableContactTools = true
    enableGroupTools = true
    enableMessageTools = true
    enableSessionTools = true
  
  # 外部 MCP Servers 配置
  [[mcpConfig.externalServers]]
    name = "github-mcp"
    enabled = true
    priority = 100
    transport = "stdio"
    
    # Stdio Transport 配置
    [mcpConfig.externalServers.command]
      executable = "npx"
      args = ["-y", "@modelcontextprotocol/server-github"]
      env = [
        "GITHUB_PERSONAL_ACCESS_TOKEN=your_token_here"
      ]
      workdir = ""
  
  [[mcpConfig.externalServers]]
    name = "filesystem-mcp"
    enabled = true
    priority = 90
    transport = "stdio"
    
    [mcpConfig.externalServers.command]
      executable = "npx"
      args = ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed/dir"]
      env = []
  
  [[mcpConfig.externalServers]]
    name = "google-calendar-mcp"
    enabled = false
    priority = 80
    transport = "http"
    
    # HTTP Transport 配置
    [mcpConfig.externalServers.http]
      endpoint = "https://api.example.com/mcp"
      authType = "bearer"  # none | bearer | api_key
      authToken = "your_api_key"
      headers = [
        "X-Custom-Header: value"
      ]


3.2 Go Config 结构体定义
// internal/config/config.go
type MCPCommandConfig struct {
    Executable string   `toml:"executable"`
    Args       []string `toml:"args"`
    Env        []string `toml:"env"`
    Workdir    string   `toml:"workdir"`
}
type MCPHTTPConfig struct {
    Endpoint   string            `toml:"endpoint"`
    AuthType   string            `toml:"authType"`   // none | bearer | api_key
    AuthToken  string            `toml:"authToken"`
    Headers    []string          `toml:"headers"`
}
type MCPExternalServerConfig struct {
    Name      string             `toml:"name"`
    Enabled   bool               `toml:"enabled"`
    Priority  int                `toml:"priority"`  // 数字越大优先级越高
    Transport string             `toml:"transport"` // stdio | http
    Command   *MCPCommandConfig  `toml:"command"`   // 用于 stdio
    HTTP      *MCPHTTPConfig     `toml:"http"`      // 用于 http
}
type MCPBuiltinServerConfig struct {
    Enabled            bool   `toml:"enabled"`
    Name               string `toml:"name"`
    Version            string `toml:"version"`
    Description        string `toml:"description"`
    EnableContactTools bool   `toml:"enableContactTools"`
    EnableGroupTools   bool   `toml:"enableGroupTools"`
    EnableMessageTools bool   `toml:"enableMessageTools"`
    EnableSessionTools bool   `toml:"enableSessionTools"`
}
type MCPConfig struct {
    Enabled                  bool                       `toml:"enabled"`
    DispatchStrategy         string                     `toml:"dispatchStrategy"`
    ToolCallTimeoutSeconds   int                        `toml:"toolCallTimeoutSeconds"`
    ServerInitTimeoutSeconds int                        `toml:"serverInitTimeoutSeconds"`
    BuiltinServer            MCPBuiltinServerConfig     `toml:"builtinServer"`
    ExternalServers          []MCPExternalServerConfig  `toml:"externalServers"`
}
// 添加到总 Config 结构体
type Config struct {
    MainConfig   `toml:"mainConfig"`
    MysqlConfig  `toml:"mysqlConfig"`
    JwtConfig    `toml:"jwtConfig"`
    MilvusConfig `toml:"milvusConfig"`
    KafkaConfig  `toml:"kafkaConfig"`
    AIConfig     `toml:"aiConfig"`
    LogConfig    `toml:"logConfig"`
    MCPConfig    `toml:"mcpConfig"`  // 新增
}
---
四、Tool 列表与接口契约规范
4.1 内置 MCP Server 工具规范
分类 1: 好友/联系人管理工具 (Contact Tools)
---
🔹 contact_list_friends  
获取用户的好友列表
- 输入参数:
    {
    tenant_user_id: string  // 租户用户ID（必填）
  }
  
- 输出结果:
    {
    friends: [
      {
        user_id: string,
        username: string,
        avatar: string,
        status: int
      }
    ],
    total: int
  }
  
---
🔹 contact_get_info  
获取联系人详细信息
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    contact_id: string       // 联系人ID（必填）
  }
  
- 输出结果:
    {
    contact_id: string,
    name: string,
    avatar: string,
    signature: string,
    gender: int,
    birthday: string
  }
  
---
🔹 contact_apply  
发送好友申请
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    contact_id: string,      // 目标联系人ID（必填）
    message: string          // 申请消息（选填）
  }
  
- 输出结果:
    {
    apply_id: string,
    status: string
  }
  
---
🔹 contact_list_applications  
获取待处理的好友申请
- 输入参数:
    {
    tenant_user_id: string  // 租户用户ID（必填）
  }
  
- 输出结果:
    {
    applications: [
      {
        uuid: string,
        user_id: string,
        username: string,
        message: string,
        last_apply_at: string
      }
    ],
    total: int
  }
  
---
🔹 contact_accept_application  
通过好友申请
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    apply_id: string         // 申请ID（必填）
  }
  
- 输出结果:
    {
    success: bool,
    message: string
  }
  
---
🔹 contact_reject_application  
拒绝好友申请
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    apply_id: string         // 申请ID（必填）
  }
  
- 输出结果:
    {
    success: bool,
    message: string
  }
  
---
分类 2: 群组管理工具 (Group Tools)
---
🔹 group_list_joined  
获取用户已加入的群列表
- 输入参数:
    {
    tenant_user_id: string  // 租户用户ID（必填）
  }
  
- 输出结果:
    {
    groups: [
      {
        group_id: string,
        group_name: string,
        avatar: string
      }
    ],
    total: int
  }
  
---
🔹 group_get_info  
获取群组详细信息
- 输入参数:
    {
    group_id: string  // 群组ID（必填）
  }
  
- 输出结果:
    {
    group_id: string,
    name: string,
    notice: string,
    owner_id: string,
    member_count: int,
    avatar: string,
    status: int,
    created_at: string
  }
  
---
🔹 group_list_members  
获取群成员列表
- 输入参数:
    {
    group_id: string  // 群组ID（必填）
  }
  
- 输出结果:
    {
    members: [
      {
        user_id: string,
        username: string,
        nickname: string,
        avatar: string,
        role: int
      }
    ],
    total: int
  }
  
---
🔹 group_create  
创建新群组
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    name: string,            // 群组名称（必填）
    notice: string,          // 群公告（选填）
    member_ids: [string]     // 初始成员ID列表（必填）
  }
  
- 输出结果:
    {
    group_id: string,
    name: string,
    owner_id: string,
    member_count: int
  }
  
---
🔹 group_invite_members  
邀请成员入群
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    group_id: string,        // 群组ID（必填）
    member_ids: [string]     // 待邀请成员ID列表（必填）
  }
  
- 输出结果:
    {
    success: bool,
    added_count: int
  }
  
---
分类 3: 消息查询工具 (Message Tools)
---
🔹 message_list_private  
获取私聊消息记录
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    contact_id: string,      // 联系人ID（必填）
    page: int,               // 页码（必填，>= 1）
    page_size: int           // 每页条数（必填，范围 1-200）
  }
  
- 输出结果:
    {
    messages: [
      {
        uuid: string,
        send_id: string,
        content: string,
        type: int,
        created_at: string
      }
    ],
    total: int
  }
  
---
🔹 message_list_group  
获取群聊消息记录
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    group_id: string,        // 群组ID（必填）
    page: int,               // 页码（必填，>= 1）
    page_size: int           // 每页条数（必填，范围 1-200）
  }
  
- 输出结果:
    {
    messages: [
      {
        uuid: string,
        send_id: string,
        send_name: string,
        content: string,
        type: int,
        created_at: string
      }
    ],
    total: int
  }
  
---
🔹 message_search  
搜索消息内容
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    keyword: string,         // 搜索关键词（必填）
    scope: string,           // 搜索范围（选填，如 "private" | "group" | "all"）
    limit: int               // 结果数量限制（必填，范围 1-100）
  }
  
- 输出结果:
    {
    messages: [
      {
        uuid: string,
        session_id: string,
        content: string,
        created_at: string
      }
    ],
    total: int
  }
  
---
分类 4: 会话管理工具 (Session Tools)
---
🔹 session_list_private  
获取私聊会话列表
- 输入参数:
    {
    tenant_user_id: string  // 租户用户ID（必填）
  }
  
- 输出结果:
    {
    sessions: [
      {
        session_id: string,
        contact_id: string,
        last_message: string,
        updated_at: string
      }
    ],
    total: int
  }
  
---
🔹 session_list_group  
获取群聊会话列表
- 输入参数:
    {
    tenant_user_id: string  // 租户用户ID（必填）
  }
  
- 输出结果:
    {
    sessions: [
      {
        session_id: string,
        group_id: string,
        last_message: string,
        updated_at: string
      }
    ],
    total: int
  }
  
---
🔹 session_open  
打开/创建会话
- 输入参数:
    {
    tenant_user_id: string,  // 租户用户ID（必填）
    contact_id: string,      // 联系人ID（contact_id 和 group_id 二选一）
    group_id: string         // 群组ID（contact_id 和 group_id 二选一）
  }
  
- 输出结果:
    {
    session_id: string,
    can_chat: bool
  }
  
---
工具数量统计:
- 好友/联系人工具: 6 个
- 群组管理工具: 5 个
- 消息查询工具: 3 个
- 会话管理工具: 3 个
- 总计: 17 个工具

4.2 工具接口契约规范 (JSON Schema)
所有工具必须遵循以下契约：
工具描述规范 (Tool Descriptor)
{
  name: contact_list_friends,
  description: 获取用户的好友列表，返回好友基本信息（用户名、头像、状态）,
  inputSchema: {
    type: object,
    properties: {
      tenant_user_id: {
        type: string,
        description: 租户用户ID（必填，从上下文获取）
      }
    },
    required: [tenant_user_id]
  }
}
工具调用请求 (Tool Call Request)
{
  jsonrpc: 2.0,
  id: call-123,
  method: tools/call,
  params: {
    name: contact_list_friends,
    arguments: {
      tenant_user_id: U_20250125_ABC123
    }
  }
}
工具调用响应 (Tool Call Response)
成功响应：
{
  jsonrpc: 2.0,
  id: call-123,
  result: {
    content: [
      {
        type: text,
        text: 找到 3 位好友：\n1. 张三 (@zhangsan) - 在线\n2. 李四 (@lisi) - 离线\n3. 王五 (@wangwu) - 在线
      }
    ],
    metadata: {
      friends: [
        { user_id: U_001, username: 张三, avatar: https://..., status: 0 },
        { user_id: U_002, username: 李四, avatar: https://..., status: 1 },
        { user_id: U_003, username: 王五, avatar: https://..., status: 0 }
      ],
      total: 3
    }
  }
}
错误响应：
{
  jsonrpc: 2.0,
  id: call-123,
  result: {
    content: [
      {
        type: text,
        text: 查询失败：用户未登录或 token 无效
      }
    ],
    isError: true
  }
}

4.3 工具命名规范
- 格式: {category}_{action}_{object}
  - category: contact | group | message | session
  - action: list | get | create | update | delete | search | send | accept | reject
  - object: friends | info | members | applications | private | group
- 示例:
  - ✅ contact_list_friends (清晰)
  - ✅ group_invite_members (动宾结构)
  - ❌ getFriends (不符合蛇形命名)
  - ❌ contact-list (缺少对象)
4.4 输入验证规范
所有工具实现必须：
1. 必填字段校验: tenant_user_id 必须存在且非空
2. 权限校验: 验证 tenant_user_id 有权访问请求的资源（好友/群/消息）
3. 参数类型校验: 严格按照 JSON Schema 验证参数类型
4. 边界值校验: 
   - page >= 1
   - page_size 范围 1, 200
   - limit 范围 1, 100
4.5 输出格式规范
所有工具输出必须包含：
1. content (必填): 数组，包含至少一个 TextContent
   - type: "text"
   - text: 人类可读的结果描述
2. metadata (可选): 结构化数据，供后续处理使用
3. isError (可选): 布尔值，标识是否为错误结果
---


五、MCP 调度器设计
5.1 MCPDispatcher 核心职责
┌─────────────────────────────────────────────────────────┐
│                      MCPDispatcher                      │
├─────────────────────────────────────────────────────────┤
│  职责：                                                  │
│  1. 接收 LLM 的工具调用请求 (tool_call)                  │
│  2. 路由到正确的 MCP Server (内置 or 外部)               │
│  3. 执行工具调用                                         │
│  4. 转换结果格式                                         │
│  5. 返回给 AI Pipeline                                   │
└─────────────────────────────────────────────────────────┘

完整架构对比
┌─────────────────────────────────────────────────────────────┐
│                      MCPDispatcher                          │
├─────────────────────────────────────────────────────────────┤
│  统一调度所有工具（内置 + 外部）                              │
└─────────────────────────────────────────────────────────────┘
          ↓                    ↓                    ↓
    
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ 内置 MCP Server  │  │ 外部 Server      │  │ 外部 Server      │
│ (无传输层)       │  │ (Stdio)          │  │ (HTTP)           │
├──────────────────┤  ├──────────────────┤  ├──────────────────┤
│ 直接函数调用 ✓   │  │ 子进程通信       │  │ HTTP 请求        │
│ 零延迟          │  │ 低延迟 (~1ms)    │  │ 网络延迟 (~50ms) │
│ 类型安全        │  │ JSON 序列化      │  │ JSON 序列化      │
│ 共享数据库      │  │ 隔离进程         │  │ 远程服务         │
└──────────────────┘  └──────────────────┘  └──────────────────┘
        ↓                     ↓                     ↓
ContactService       GitHub CLI           Google Calendar API
GroupService         Filesystem           Notion API
MessageService       ...                  ...
---


5.2 接口定义
// internal/modules/ai/application/service/mcp_dispatcher.go
package service
import (
    "context"
    "OmniLink/internal/modules/ai/infrastructure/mcp/types"
)
// MCPDispatcher MCP 调度器接口
type MCPDispatcher interface {
    // ListAvailableTools 列出所有可用工具
    ListAvailableTools(ctx context.Context, tenantUserID string) ([]types.ToolDescriptor, error)
    
    // CallTool 调用指定工具
    CallTool(ctx context.Context, req *ToolCallRequest) (*ToolCallResponse, error)
    
    // GetToolByName 根据名称查找工具
    GetToolByName(ctx context.Context, name string) (*types.ToolDescriptor, error)
    
    // RefreshServers 重新加载 MCP Server 配置
    RefreshServers(ctx context.Context) error
}
// ToolCallRequest 工具调用请求
type ToolCallRequest struct {
    TenantUserID string                 // 租户用户 ID (必填)
    ToolName     string                 // 工具名称 (必填)
    Arguments    map[string]interface{} // 工具参数
    Timeout      int                    // 超时时间 (秒)，0 表示使用默认值
}
// ToolCallResponse 工具调用响应
type ToolCallResponse struct {
    Success  bool                   // 是否成功
    Content  string                 // 文本内容 (给 LLM 看的)
    Metadata map[string]interface{} // 结构化数据 (可选)
    Error    string                 // 错误信息 (如果失败)
}
5.3 工作流程
sequenceDiagram
    participant LLM as LLM (ChatModel)
    participant Pipe as AssistantPipeline
    participant Disp as MCPDispatcher
    participant Router as ToolRouter
    participant Builtin as BuiltinMCPServer
    participant External as ExternalMCPClient
    LLM->>Pipe: 返回 tool_call (name=contact_list_friends)
    Pipe->>Disp: CallTool(name, args)
    Disp->>Router: RouteToolCall(name)
    Router-->>Disp: 返回 target=BuiltinServer
    Disp->>Builtin: ExecuteTool(name, args)
    Builtin->>Builtin: 调用 ContactService.GetUserList()
    Builtin-->>Disp: 返回结果
    Disp->>Disp: 转换为 ToolCallResponse
    Disp-->>Pipe: 返回结果
    Pipe->>LLM: 将结果作为 tool_result 回填
    LLM-->>Pipe: 生成最终回答
---

六、internal/modules/ai/infrastructure/mcp/ 目录设计
6.1 核心组件说明
A. MCP Client (client/mcp_client.go)
职责:
- 管理与外部 MCP Server 的连接
- 处理 JSON-RPC 2.0 消息序列化/反序列化
- 执行 initialize → initialized 握手
- 调用 tools/list、tools/call 等方法
核心接口:
type MCPClient interface {
    // Initialize 初始化连接（握手）
    Initialize(ctx context.Context, clientInfo ClientInfo) (*InitializeResult, error)
    
    // ListTools 获取 Server 提供的工具列表
    ListTools(ctx context.Context) ([]ToolDescriptor, error)
    
    // CallTool 调用指定工具
    CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error)
    
    // Close 关闭连接
    Close(ctx context.Context) error
}
B. Server Registry (registry/server_registry.go)
职责:
- 注册内置 MCP Server
- 注册外部 MCP Server (从配置加载)
- 管理 Server 生命周期 (启动/停止/重启)
- 提供 Server 查询接口
核心接口:
type ServerRegistry interface {
    // RegisterBuiltinServer 注册内置 Server
    RegisterBuiltinServer(server MCPServer) error
    
    // RegisterExternalServer 注册外部 Server
    RegisterExternalServer(config MCPExternalServerConfig) error
    
    // GetServerByName 根据名称获取 Server
    GetServerByName(name string) (RegisteredServer, error)
    
    // ListServers 列出所有已注册的 Server
    ListServers() []RegisteredServer
    
    // StartAll 启动所有 Server
    StartAll(ctx context.Context) error
    
    // StopAll 停止所有 Server
    StopAll(ctx context.Context) error
}
type RegisteredServer struct {
    Name      string
    Type      string  // builtin | external
    Priority  int
    Status    string  // running | stopped | error
    Client    MCPClient  // 外部 Server 的客户端连接
    Server    MCPServer  // 内置 Server 实例
}
C. Tool Router (router/tool_router.go)
职责:
- 根据工具名称路由到正确的 MCP Server
- 实现路由策略 (优先级、轮询、随机)
- 缓存工具列表 (减少重复查询)
核心接口:
type ToolRouter interface {
    // RouteToolCall 路由工具调用
    RouteToolCall(ctx context.Context, toolName string) (*RouteTarget, error)
    
    // ListAllTools 列出所有 Server 的工具
    ListAllTools(ctx context.Context) ([]ToolDescriptor, error)
    
    // RefreshToolCache 刷新工具缓存
    RefreshToolCache(ctx context.Context) error
}
type RouteTarget struct {
    ServerName string
    ServerType string  // builtin | external
    Tool       ToolDescriptor
}
D. Builtin MCP Server (server/builtin_server.go)
职责:
- 实现 MCP Server 协议
- 封装 OmniLink 内部能力（Contact/Group/Message/Session Services）
- 注册工具到 ToolRegistry
- 处理 tools/list 和 tools/call 请求
核心接口:
type MCPServer interface {
    // GetServerInfo 获取 Server 基本信息
    GetServerInfo() ServerInfo
    
    // ListTools 列出所有工具
    ListTools(ctx context.Context) ([]ToolDescriptor, error)
    
    // CallTool 执行工具调用
    CallTool(ctx context.Context, name string, args map[string]interface{}) (*CallToolResult, error)
}
type ServerInfo struct {
    Name        string
    Version     string
    Description string
}
---
七、集成到 AssistantPipeline 的方案
7.1 修改 AssistantPipeline 以支持 Tool Calling
方案 A: 在 chatModelNode 后增加 ToolCallNode (推荐)
当前流程:
  LoadMemory → Retrieve → BuildPrompt → ChatModel → Persist
修改后流程:
  LoadMemory → Retrieve → BuildPrompt → ChatModel → [ToolCall] → [ChatModel Again] → Persist
                                           ↓ (如果 LLM 返回 tool_call)
                                      MCPDispatcher
                                           ↓
                                      执行工具，回填结果
                                           ↓
                                      返回 ChatModel 继续生成
增强的 chatModelNode 逻辑:
// 伪代码
func (p *AssistantPipeline) chatModelNode(ctx context.Context, st *assistantState) (*assistantState, error) {
    // 第一次调用 LLM
    response := chatModel.Generate(st.PromptMsgs)
    
    // 检查是否有 tool_call
    if response.HasToolCalls() {
        // 调用 MCPDispatcher
        for _, toolCall := range response.ToolCalls {
            result := mcpDispatcher.CallTool(ctx, &ToolCallRequest{
                TenantUserID: st.Req.TenantUserID,
                ToolName:     toolCall.Name,
                Arguments:    toolCall.Arguments,
            })
            
            // 将工具结果添加到消息历史
            st.PromptMsgs = append(st.PromptMsgs, schema.Message{
                Role: "tool",
                Name: toolCall.Name,
                Content: result.Content,
            })
        }
        
        // 第二次调用 LLM (生成最终回答)
        finalResponse := chatModel.Generate(st.PromptMsgs)
        st.Answer = finalResponse.Content
    } else {
        st.Answer = response.Content
    }
    
    return st, nil
}
7.2 配置 LLM 使用 Tool
在 buildPromptNode 中，需要将可用工具列表传递给 LLM：
func (p *AssistantPipeline) buildPromptNode(ctx context.Context, st *assistantState) (*assistantState, error) {
    // ... 现有逻辑 ...
    
    // 新增：获取可用工具列表
    if p.mcpDispatcher != nil {
        tools, err := p.mcpDispatcher.ListAvailableTools(ctx, st.Req.TenantUserID)
        if err == nil {
            st.AvailableTools = tools  // 保存到 state
        }
    }
    
    return st, nil
}
// 然后在 chatModelNode 调用 LLM 时：
func (p *AssistantPipeline) chatModelNode(ctx context.Context, st *assistantState) (*assistantState, error) {
    // 构造 LLM 请求，包含 tools 参数
    llmRequest := &LLMRequest{
        Messages: st.PromptMsgs,
        Tools:    convertToolsToLLMFormat(st.AvailableTools),  // 转换为 OpenAI/Claude 格式
    }
    
    response := p.chatModel.Generate(ctx, llmRequest)
    // ... 后续处理 ...
}
---
八、工具实现示例（仅展示结构，不写完整代码）
8.1 内置 Server 工具 Handler 结构
// internal/modules/ai/infrastructure/mcp/server/handlers/contact_handler.go
package handlers
import (
    "context"
    contactService "OmniLink/internal/modules/contact/application/service"
    "OmniLink/internal/modules/ai/infrastructure/mcp/types"
)
type ContactToolHandler struct {
    contactSvc contactService.ContactService
}
func NewContactToolHandler(svc contactService.ContactService) *ContactToolHandler {
    return &ContactToolHandler{contactSvc: svc}
}
// RegisterTools 注册所有好友相关工具
func (h *ContactToolHandler) RegisterTools() []types.ToolDescriptor {
    return []types.ToolDescriptor{
        {
            Name:        "contact_list_friends",
            Description: "获取用户的好友列表",
            InputSchema: map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "tenant_user_id": map[string]string{"type": "string"},
                },
                "required": []string{"tenant_user_id"},
            },
            Handler: h.handleListFriends,  // 绑定处理函数
        },
        // ... 其他工具
    }
}
// handleListFriends 实现具体逻辑
func (h *ContactToolHandler) handleListFriends(ctx context.Context, args map[string]interface{}) (*types.CallToolResult, error) {
    // 1. 参数校验
    tenantUserID := args["tenant_user_id"].(string)
    
    // 2. 调用 ContactService
    friends, err := h.contactSvc.GetUserList(contactRequest.GetUserListRequest{
        OwnerId: tenantUserID,
    })
    
    // 3. 转换为 MCP 结果格式
    return &types.CallToolResult{
        Content: []types.Content{
            {
                Type: "text",
                Text: formatFriendsAsText(friends),  // 转换为人类可读文本
            },
        },
        Metadata: map[string]interface{}{
            "friends": friends,
            "total":   len(friends),
        },
    }, nil
}
---
九、关键技术决策
9.1 为什么选择 JSON-RPC 2.0？
- 官方标准: MCP 官方规范要求使用 JSON-RPC 2.0
- 简单可靠: 成熟的 RPC 协议，易于调试
- 双向通信: 支持请求-响应 + 通知模式
9.2 为什么支持 Stdio 和 HTTP 两种传输？
- Stdio: 本地进程通信，安全性高，适合本地工具 (GitHub CLI, 文件系统)
- HTTP: 远程服务调用，适合云端 API (Google Calendar, Notion)
9.3 为什么需要 ToolRouter？
- 多 Server 场景: 可能有 10+ 个 MCP Server，每个提供不同工具
- 冲突解决: 两个 Server 可能提供同名工具，需要优先级策略
- 性能优化: 缓存工具列表，避免重复调用 tools/list
9.4 为什么内置 Server 也要实现 MCP 协议？
- 统一接口: 内外部 Server 使用相同接口，简化 Dispatcher 逻辑
- 可插拔: 未来可以将内置 Server 拆分为独立进程
- 可测试: 内置 Server 可以独立测试，无需启动整个应用
---
十、最小可行实现（MVP）路线图
Phase 1: 核心框架 (1-2 天)
1. ✅ 定义 MCP 配置结构 (TOML + Go struct)
2. ✅ 实现 types/protocol.go (JSON-RPC 类型定义)
3. ✅ 实现 registry/server_registry.go (Server 注册表)
4. ✅ 实现 MCPDispatcher 接口
Phase 2: 内置 Server (2-3 天)
1. ✅ 实现 server/builtin_server.go
2. ✅ 实现 handlers/contact_handler.go (5 个工具)
3. ✅ 实现 handlers/group_handler.go (3 个工具)
4. ✅ 集成测试：调用内置工具
Phase 3: 外部 Server 支持 (2-3 天)
1. ✅ 实现 client/transport_stdio.go (Stdio 传输)
2. ✅ 实现 client/mcp_client.go (JSON-RPC 客户端)
3. ✅ 实现 router/tool_router.go (工具路由)
4. ✅ 集成测试：连接外部 MCP Server (如 @modelcontextprotocol/server-filesystem)
Phase 4: Pipeline 集成 (1-2 天)
1. ✅ 修改 AssistantPipeline 支持 tool calling
2. ✅ 修改 chatModelNode 实现工具调用循环
3. ✅ 端到端测试：用户问"我有哪些好友" → LLM 调用 contact_list_friends → 返回结果
---
十一、最终交付物
11.1 配置文件
- configs/config.toml - 新增 [mcpConfig] 配置段
- internal/config/config.go - 新增 MCPConfig 结构体
11.2 核心代码文件（仅结构，不含实现）
internal/modules/ai/
├── application/service/mcp_dispatcher.go        (接口定义)
├── infrastructure/mcp/
│   ├── types/protocol.go                        (MCP 协议类型)
│   ├── types/tool.go                            (Tool 定义)
│   ├── types/errors.go                          (错误定义)
│   ├── client/mcp_client.go                     (MCP Client 实现)
│   ├── client/session_manager.go                (Session 管理)
│   ├── client/transport_stdio.go                (Stdio 传输)
│   ├── server/builtin_server.go                 (内置 Server)
│   ├── server/tool_registry.go                  (工具注册表)
│   ├── server/handlers/contact_handler.go       (好友工具)
│   ├── server/handlers/group_handler.go         (群组工具)
│   ├── server/handlers/message_handler.go       (消息工具)
│   ├── registry/server_registry.go              (Server 注册表)
│   └── router/tool_router.go                    (工具路由)
11.3 文档
- 工具规范文档: 所有 16 个内置工具的 JSON Schema 定义
- 配置指南: 如何添加外部 MCP Server
- 开发指南: 如何添加新的内置工具
---
十二、后续扩展方向
1. 工具权限控制: 基于用户角色限制可用工具
2. 工具使用审计: 记录所有工具调用日志
3. 工具缓存: 缓存频繁调用工具的结果 (如 list_friends)
4. 工具组合: 支持一次调用多个工具并聚合结果
5. HTTP Transport: 实现远程 MCP Server 支持
6. Resource 支持: 扩展到 MCP Resources (如"我的聊天记录")
7. Prompt 支持: 扩展到 MCP Prompts (如"帮我写一封邮件")
---
以上就是完整的 MCP 框架搭建设计方案！ 🎉
这个方案：
- ✅ 保持了 DDD 分层架构不动
- ✅ 新增 internal/modules/ai/infrastructure/mcp/ 完整结构
- ✅ 定义了 TOML 配置规范
- ✅ 规范了 16 个内置工具的接口契约
- ✅ 提供了 MCPDispatcher 的详细设计
- ✅ 给出了集成到 AssistantPipeline 的具体方案
- ✅ 包含最小可行实现（MVP）路线图
所有设计都遵循了 OmniLink 现有的架构风格和技术栈！