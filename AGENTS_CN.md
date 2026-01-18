# AGENTS.md - AI 编码智能体开发指南 (OmniLink)

## 项目概述

OmniLink 是一个支持 RAG（检索增强生成）AI 功能的实时聊天/即时通讯应用。

- **后端**：Go 1.25、Gin HTTP 框架、GORM ORM、Zap 日志
- **前端**：Vue 3 + Vite + Element Plus（位于 `web/` 目录）
- **架构**：DDD（领域驱动设计），层级清晰分离
- **基础设施**：MySQL、Milvus（向量数据库）、Kafka（消息队列）

## 构建/测试/检查命令

### 后端 (Go)

```bash
# 构建
go build -o OmniLink.exe ./cmd/OmniLink

# 运行服务
go run ./cmd/OmniLink/main.go

# 测试所有包
go test ./...

# 测试单个包
go test ./internal/modules/user/application/service/...

# 测试单个文件（按模式）
go test -v -run TestFunctionName ./path/to/package/...

# 格式化代码
go fmt ./...

# 代码检查
go vet ./...

# 获取依赖
go mod tidy
```

### 前端 (Vue 3)

```bash
cd web

# 安装依赖
npm install

# 开发服务器
npm run dev

# 生产构建
npm run build

# 预览生产构建
npm run preview
```

## 目录结构

```
OmniLink/
├── cmd/OmniLink/main.go     # 应用入口
├── api/http/                 # HTTP 服务设置、路由、中间件配置
├── internal/
│   ├── config/              # 配置加载 (TOML)
│   ├── initial/             # 数据库/Milvus 初始化
│   ├── middleware/          # HTTP 中间件 (JWT 认证)
│   └── modules/
│       ├── user/            # 用户模块
│       ├── contact/         # 联系人 & 群组
│       ├── chat/            # 会话 & 消息
│       └── ai/              # RAG、向量嵌入、向量搜索
│           ├── domain/      # 实体、仓储接口
│           ├── application/ # 服务、DTO
│           ├── infrastructure/  # 实现
│           └── interface/   # HTTP 处理器、工作线程
├── pkg/                     # 共享工具
│   ├── back/               # HTTP 响应辅助
│   ├── xerr/               # 错误码 & 类型
│   ├── zlog/               # 日志封装 (Zap)
│   ├── util/               # ID 生成、辅助函数
│   ├── ws/                 # WebSocket 中心
│   └── ssl/                # TLS 辅助
├── configs/                 # TOML 配置文件
├── web/                     # Vue 3 前端
└── docs/                    # 文档
```

## 代码风格规范

### 导入顺序

```go
import (
    // 1. 标准库
    "context"
    "errors"
    "time"

    // 2. 项目内部包（需要时使用别名）
    aiService "OmniLink/internal/modules/ai/application/service"
    "OmniLink/internal/modules/user/domain/entity"
    "OmniLink/pkg/xerr"
    "OmniLink/pkg/zlog"

    // 3. 第三方包
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)
```

### 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 实体 | PascalCase，含 `TableName()` 方法 | `UserInfo`, `Message` |
| 仓储接口 | 位于 domain 层，`XxxRepository` | `UserInfoRepository` |
| 仓储实现 | 位于 infrastructure 层，`xxxRepositoryImpl` | `userInfoRepositoryImpl` |
| 服务接口 | 位于 application 层，`XxxService` | `UserInfoService` |
| 服务实现 | 私有结构体，`xxxServiceImpl` | `userInfoServiceImpl` |
| 处理器 | 结构体，`XxxHandler` | `UserInfoHandler` |
| 构造函数 | `NewXxx()` 函数 | `NewUserInfoService()` |
| DTO | 独立的 `request/` 和 `respond/` 包 | `LoginRequest`, `LoginRespond` |
| ID 生成 | 带前缀 | `U`（用户）、`G`（群组）、`S`（会话）、`M`（消息）、`A`（申请） |

### 实体定义规范

```go
type UserInfo struct {
    Id       int64  `gorm:"column:id;primaryKey;comment:自增id"`
    Uuid     string `gorm:"column:uuid;uniqueIndex;type:char(20);comment:用户唯一id"`
    Username string `gorm:"column:username;uniqueIndex;type:varchar(20);not null;comment:账号"`
    // ... 更多字段，使用 GORM 标签
}

func (UserInfo) TableName() string {
    return "user_info"
}
```

### 仓储模式

```go
// Domain 层（接口）
type UserInfoRepository interface {
    CreateUserInfo(user *entity.UserInfo) error
    GetUserInfoById(id int64) (*entity.UserInfo, error)
}

// Infrastructure 层（实现）
type userInfoRepositoryImpl struct {
    db *gorm.DB
}

func NewUserInfoRepository(db *gorm.DB) repository.UserInfoRepository {
    return &userInfoRepositoryImpl{db: db}
}

func (r *userInfoRepositoryImpl) CreateUserInfo(user *entity.UserInfo) error {
    return r.db.Create(user).Error
}
```

### 服务模式

```go
// 接口
type UserInfoService interface {
    Login(req request.LoginRequest) (*respond.LoginRespond, error)
}

// 实现
type userInfoServiceImpl struct {
    repo repository.UserInfoRepository
}

func NewUserInfoService(repo repository.UserInfoRepository) UserInfoService {
    return &userInfoServiceImpl{repo: repo}
}
```

### HTTP 处理器模式

```go
type UserInfoHandler struct {
    svc service.UserInfoService
}

func NewUserInfoHandler(svc service.UserInfoService) *UserInfoHandler {
    return &UserInfoHandler{svc: svc}
}

func (h *UserInfoHandler) Login(c *gin.Context) {
    var req request.LoginRequest
    if err := c.BindJSON(&req); err != nil {
        zlog.Error(err.Error())
        back.Error(c, xerr.BadRequest, xerr.ErrParam.Message)
        return
    }
    data, err := h.svc.Login(req)
    back.Result(c, data, err)
}
```

### 错误处理

使用 `xerr` 包处理结构化错误：

```go
// 创建错误
xerr.New(xerr.BadRequest, "用户已存在")
xerr.ErrServerError  // 预定义服务器错误

// 错误码 (xerr 包)
xerr.OK                  = 200
xerr.BadRequest          = 400
xerr.Unauthorized        = 401
xerr.Forbidden           = 403
xerr.NotFound            = 404
xerr.InternalServerError = 500
```

### HTTP 响应模式

使用 `back` 包：

```go
back.Result(c, data, err)  // 自动处理成功/错误
back.Success(c, data)      // 成功响应
back.Error(c, code, msg)   // 错误响应
```

### 日志

使用 `zlog` 包（封装 Zap）：

```go
zlog.Info("message")      // 信息
zlog.Warn("message")      // 警告
zlog.Error("message")     // 错误
zlog.Fatal("message")     // 致命错误（退出程序）
zlog.Debug("message")     // 调试
```

### 配置

TOML 格式，位于 `configs/config_local.toml`：

```toml
[mainConfig]
appName = "OmniLink"
host = "0.0.0.0"
port = 8000

[mysqlConfig]
host = "127.0.0.1"
port = 3306
# ...
```

通过 `config.GetConfig()` 单例访问。

## 需要遵循的关键模式

1. **DDD 分层**：domain → application → infrastructure → interface
2. **依赖注入**：构造函数注入，在 `api/http/https_server.go` 中组装
3. **事务处理**：使用 `UnitOfWork` 模式处理跨聚合事务
4. **JWT 认证**：中间件将 `uuid` 和 `username` 提取到 gin.Context
5. **ID 生成**：使用 `util.GenerateXxxID()` 函数

## 常见错误避免

1. **禁止**在响应中返回密码字段
2. **禁止**记录敏感数据（密码、令牌、API 密钥）
3. **必须**检查 `gorm.ErrRecordNotFound` 处理"未找到"情况
4. **必须**对业务错误使用 `xerr`，而非原始 `errors.New()`
5. **必须**在处理前验证输入（检查空字符串、nil）
6. **优先**使用现有库而非引入新依赖

## 测试指南

当前不存在测试文件。添加测试时：

```go
// 文件名：user_info_service_test.go
package service_test

import (
    "testing"
    // ...
)

func TestUserInfoService_Login(t *testing.T) {
    // Arrange
    // Act
    // Assert
}
```

运行测试：`go test -v ./internal/modules/user/application/service/...`

## API 认证

受保护的路由需要 `Authorization: Bearer <token>` 请求头：

```bash
curl -X POST http://localhost:8000/contact/getUserList \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"ownerId": "U..."}'
```

## 模块特定说明

### AI 模块 (`internal/modules/ai/`)

- 使用 Kafka 进行异步摄取事件（发件箱模式）
- Milvus 用于向量存储
- 通过可配置提供商提供向量嵌入（Ark、OpenAI）
- RAG 实体：`AIKnowledgeBase`、`AIKnowledgeSource`、`AIKnowledgeChunk`、`AIVectorRecord`

### 聊天模块 (`internal/modules/chat/`)

- WebSocket 实现实时消息
- 会话跟踪对话（1对1 和 群组）
- 消息类型：text(0)、voice(1)、file(2)、call(3)

### 联系人模块 (`internal/modules/contact/`)

- 处理好友关系和群组成员
- 联系人类型：user(0)、group(1)
- 好友/群组申请的审批工作流
