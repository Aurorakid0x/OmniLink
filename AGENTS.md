# OmniLink 项目 AI 代理指南

> 本文档为在 OmniLink 项目上工作的 AI 编码代理提供必要的信息。
> OmniLink 是一个集成了 AI 功能的即时通讯（IM）应用程序。

## 项目概述

**OmniLink** 是一个全栈即时通讯平台，结合了传统的 IM 功能（私聊、群聊、联系人）和现代 AI 功能（基于 RAG 的知识检索、AI 助手和自动化任务执行）。

### 核心功能
- **即时通讯**：通过 WebSocket 实现私聊、群聊、联系人管理
- **AI 助手**：具备工具使用能力的对话式 AI（MCP 协议）
- **RAG 系统**：使用向量数据库进行知识库摄取和检索
- **任务调度**：基于 Cron 的自动化 AI 任务执行
- **用户管理**：基于 JWT 的身份认证和用户生命周期管理

## 技术栈

### 后端（Go 1.25）
| 组件 | 技术 |
|------|------|
| Web 框架 | [Gin](https://github.com/gin-gonic/gin) |
| 数据库 ORM | [GORM](https://gorm.io) + MySQL |
| 向量数据库 | [Milvus](https://milvus.io) |
| 消息队列 | [Kafka](https://kafka.apache.org)（通过 Sarama） |
| WebSocket | [Gorilla WebSocket](https://github.com/gorilla/websocket) |
| 身份认证 | JWT（golang-jwt/jwt/v5） |
| AI 框架 | [CloudWeGo Eino](https://github.com/cloudwego/eino) |
| MCP 协议 | [mcp-go](https://github.com/mark3labs/mcp-go) |
| 日志 | Zap + Lumberjack |

### 前端（Vue 3）
| 组件 | 技术 |
|------|------|
| 框架 | Vue 3（Composition API） |
| 构建工具 | Vite |
| UI 库 | Element Plus |
| 状态管理 | Vuex |
| 路由 | Vue Router |
| HTTP 客户端 | Axios |

### 基础设施依赖
- **MySQL**：主数据存储
- **Milvus**：RAG 的向量存储
- **Kafka**：AI 摄取的异步消息处理

## 项目结构

```
OmniLink/
├── cmd/OmniLink/           # 应用程序入口
│   └── main.go
├── api/http/               # HTTP 服务器设置和路由
│   └── https_server.go     # Gin 引擎初始化、依赖注入
├── internal/               # 私有应用程序代码
│   ├── config/             # 配置管理（TOML）
│   ├── initial/            # 数据库初始化（GORM、Milvus）
│   ├── middleware/jwt/     # JWT 认证中间件
│   └── modules/            # 领域模块（整洁架构）
│       ├── user/           # 用户管理
│       ├── contact/        # 联系人和群组
│       ├── chat/           # 会话和消息
│       └── ai/             # AI 功能（RAG、助手、任务）
│           ├── domain/     # 实体、仓库接口
│           ├── application/# DTO、服务接口
│           ├── infrastructure/ # 实现（DB、MQ、LLM、向量 DB）
│           └── interface/  # HTTP 处理器、事件处理器、调度器
├── pkg/                    # 公共共享包
│   ├── ws/                 # WebSocket 中心实现
│   ├── zlog/               # 基于 Zap 的结构化日志
│   ├── util/               # 工具函数
│   ├── constants/          # 常量
│   └── xerr/               # 错误处理
├── eino-main/              # 嵌入式 CloudWeGo Eino 框架
├── web/                    # 前端（Vue 3）
│   ├── src/
│   │   ├── views/          # 页面视图
│   │   ├── components/     # Vue 组件
│   │   ├── store/          # Vuex store
│   │   └── api/            # API 客户端
│   └── package.json
├── configs/                # 配置文件（TOML）
├── docs/                   # 文档（中文）
└── scripts/                # 构建/工具脚本
```

## 架构模式

项目遵循**整洁架构** / **领域驱动设计（DDD）** 原则：

```
┌─────────────────────────────────────────────────────────────┐
│                    接口层（Interface Layer）                   │
│  （HTTP 处理器、WebSocket 处理器、任务工作者、调度器）           │
├─────────────────────────────────────────────────────────────┤
│                   应用层（Application Layer）                 │
│  （DTO、服务 - 编排领域操作）                                  │
├─────────────────────────────────────────────────────────────┤
│                     领域层（Domain Layer）                    │
│  （实体、仓库接口 - 业务逻辑）                                 │
├─────────────────────────────────────────────────────────────┤
│                  基础设施层（Infrastructure Layer）            │
│  （仓库实现、MQ、LLM 客户端、向量 DB、缓存）                    │
└─────────────────────────────────────────────────────────────┘
```

### 模块组织（internal/modules/）
每个模块遵循相同的结构：
```
module/
├── domain/
│   ├── entity/         # 领域实体
│   └── repository/     # 仓库接口
├── application/
│   ├── dto/request/    # 请求 DTO
│   ├── dto/respond/    # 响应 DTO
│   └── service/        # 应用服务
├── infrastructure/
│   └── persistence/    # 仓库实现
└── interface/
    └── http/           # HTTP 处理器
```

## 配置

配置以 TOML 格式存储在 `configs/config.toml`（模板）和 `configs/config_local.toml`（本地开发）中。

关键配置部分：
- `mainConfig`：应用名称、主机、端口
- `mysqlConfig`：数据库连接
- `jwtConfig`：JWT 签名密钥和过期时间
- `milvusConfig`：向量数据库设置
- `kafkaConfig`：消息队列配置
- `aiConfig`：LLM 和嵌入提供程序设置（OpenAI、Ark 等）
- `mcpConfig`：MCP（模型上下文协议）服务器设置

**注意**：`*_local.toml` 文件被 gitignore 忽略，应包含你的本地凭证。

## 构建和运行

### 前提条件
- Go 1.25+
- MySQL 8.0+
- Milvus 2.4+
- Kafka 3.0+（可选，没有它 AI 功能也能在降级模式下工作）
- Node.js 18+（用于前端）

### 后端

```bash
# 安装依赖
go mod download

# 运行应用程序
go run cmd/OmniLink/main.go

# 或构建可执行文件
go build -o OmniLink.exe cmd/OmniLink/main.go
./OmniLink.exe
```

服务器默认在 8000 端口启动（在 `configs/config_local.toml` 中配置）。

### 前端

```bash
cd web

# 安装依赖
npm install

# 开发服务器
npm run dev

# 生产构建
npm run build
```

前端开发服务器通常在 5173 端口运行，并将 API 请求代理到后端。

## 核心组件

### WebSocket 中心（pkg/ws/hub.go）
中央 WebSocket 连接管理器：
- 按用户 ID 管理客户端连接
- 处理消息广播
- 支持每个用户多个连接

### 身份认证
- 基于 JWT 的身份认证，中间件位于 `internal/middleware/jwt/auth.go`
- Token 包含 `uuid` 和 `username` 声明
- 受保护的路由使用 `authed.Use(jwtMiddleware.Auth())`

### AI 管道架构
AI 模块使用 Eino 的管道模式：

1. **摄取管道**：聊天消息 → 分块 → 嵌入 → 存储到 Milvus
2. **检索管道**：查询 → 嵌入 → 向量搜索 → 返回上下文
3. **助手管道**：聊天历史 + 上下文 → LLM → 响应（带工具使用）
4. **任务执行管道**：带 Cron 调度的自动化任务执行

### MCP（模型上下文协议）
OmniLink 实现 MCP 用于工具使用：
- 内置 MCP 服务器公开联系人/群组/消息/会话工具
- 工具动态注入到 AI 管道中
- 通过 TOML 中的 `mcpConfig` 进行配置

## 开发指南

### 代码风格
- 遵循标准 Go 约定（`gofmt`）
- 使用有意义的驼峰命名变量
- 为导出的函数和类型添加注释
- 错误消息使用中文（现有约定）

### 日志
使用自定义的 `zlog` 包：
```go
import "OmniLink/pkg/zlog"

zlog.Info("message", zap.String("key", "value"))
zlog.Error("error message", zap.Error(err))
```

### 错误处理
- 仓库错误应带有上下文包装
- HTTP 处理器返回标准化的错误响应
- 使用 `xerr.CodeError` 处理带代码的业务错误

### 数据库
- 使用 GORM 进行数据库操作
- 自动迁移发生在 `internal/initial/gorm.go` 中
- 在每个模块的 `domain/entity/` 中定义实体

### 添加新 API 端点
1. 在 `application/dto/request/` 和 `application/dto/respond/` 中定义 DTO
2. 在 `application/service/` 中实现服务逻辑
3. 在 `interface/http/` 中创建处理器
4. 在 `api/http/https_server.go` 中注册路由

### 前端开发
- 使用 Composition API 配合 `<script setup>`
- 通过 `web/src/store/` 中的 Vuex store 进行状态管理
- API 调用集中在 `web/src/api/`
- 使用 Element Plus 组件进行 UI 构建

## 测试

目前，项目的测试覆盖率有限。添加测试时：

```bash
# 运行所有测试
go test ./...

# 带覆盖率运行
go test -cover ./...
```

测试文件遵循 Go 约定：在同个包中使用 `*_test.go`。

## 重要说明

### 数据库自动迁移
应用程序在启动时自动迁移所有实体表。参见 `internal/initial/gorm.go` 中的迁移实体列表。

### Milvus 模式
Milvus 集合模式在启动时自动创建（如果不存在）。参见 `internal/initial/milvus.go`。

### Kafka 主题
应用程序在启动时自动创建所需的 Kafka 主题。参见 `internal/modules/ai/infrastructure/mq/kafka/admin_sarama.go`。

### 优雅关闭
主函数处理 SIGINT/SIGTERM 信号以实现优雅关闭。如果你需要清理其他资源，请在 `cmd/OmniLink/main.go` 中扩展退出处理器。

### 安全注意事项
- JWT 密钥应该是强密钥并定期轮换
- HTTPS 受支持但默认被注释掉（参见 main.go）
- CORS 配置为在开发中允许所有来源
- 应验证文件上传路径以防止目录遍历

## 文档

其他文档（中文）可在 `docs/` 目录中找到：
- `IM-消息收发.md`：IM 消息流程文档
- `开发日志.md`：开发日志和设计决策
- `bugfix.md`：Bug 修复记录

## TODO 和已知问题

参见 `AI_MODULE_TODO.md` 了解 AI 模块的改进计划，包括：
- 工具动态加载改进
- 调度器的分布式锁定
- 带堆栈跟踪的增强错误处理

---

**最后更新**：2026-02-21
**项目语言**：中文（简体）用于 UI 和文档
