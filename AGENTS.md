# AGENTS.md - AI Coding Agent Guidelines for OmniLink

## Project Overview

OmniLink is a real-time chat/IM application with AI-powered RAG (Retrieval Augmented Generation) capabilities.

- **Backend**: Go 1.25, Gin HTTP framework, GORM ORM, Zap logging
- **Frontend**: Vue 3 + Vite + Element Plus (in `web/`)
- **Architecture**: DDD (Domain-Driven Design) with clean layer separation
- **Infrastructure**: MySQL, Milvus (vector DB), Kafka (message queue)

## Build/Lint/Test Commands

### Backend (Go)

```bash
# Build
go build -o OmniLink.exe ./cmd/OmniLink

# Run server
go run ./cmd/OmniLink/main.go

# Test all packages
go test ./...

# Test single package
go test ./internal/modules/user/application/service/...

# Test single file (pattern)
go test -v -run TestFunctionName ./path/to/package/...

# Format code
go fmt ./...

# Vet code
go vet ./...

# Get dependencies
go mod tidy
```

### Frontend (Vue 3)

```bash
cd web

# Install dependencies
npm install

# Development server
npm run dev

# Production build
npm run build

# Preview production build
npm run preview
```

## Directory Structure

```
OmniLink/
├── cmd/OmniLink/main.go     # Application entry point
├── api/http/                 # HTTP server setup, routes, middleware wiring
├── internal/
│   ├── config/              # Configuration loading (TOML)
│   ├── initial/             # DB/Milvus initialization
│   ├── middleware/          # HTTP middleware (JWT auth)
│   └── modules/
│       ├── user/            # User module
│       ├── contact/         # Contacts & groups
│       ├── chat/            # Sessions & messages
│       └── ai/              # RAG, embeddings, vector search
│           ├── domain/      # Entities, repository interfaces
│           ├── application/ # Services, DTOs
│           ├── infrastructure/  # Implementations
│           └── interface/   # HTTP handlers, workers
├── pkg/                     # Shared utilities
│   ├── back/               # HTTP response helpers
│   ├── xerr/               # Error codes & types
│   ├── zlog/               # Logging wrapper (Zap)
│   ├── util/               # ID generation, helpers
│   ├── ws/                 # WebSocket hub
│   └── ssl/                # TLS helpers
├── configs/                 # TOML config files
├── web/                     # Vue 3 frontend
└── docs/                    # Documentation
```

## Code Style Guidelines

### Import Order

```go
import (
    // 1. Standard library
    "context"
    "errors"
    "time"

    // 2. Project internal packages (aliased when needed)
    aiService "OmniLink/internal/modules/ai/application/service"
    "OmniLink/internal/modules/user/domain/entity"
    "OmniLink/pkg/xerr"
    "OmniLink/pkg/zlog"

    // 3. Third-party packages
    "github.com/gin-gonic/gin"
    "gorm.io/gorm"
)
```

### Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Entities | PascalCase, with `TableName()` method | `UserInfo`, `Message` |
| Repository Interface | `XxxRepository` in domain | `UserInfoRepository` |
| Repository Impl | `xxxRepositoryImpl` in infrastructure | `userInfoRepositoryImpl` |
| Service Interface | `XxxService` in application | `UserInfoService` |
| Service Impl | `xxxServiceImpl` private struct | `userInfoServiceImpl` |
| Handler | `XxxHandler` struct | `UserInfoHandler` |
| Constructor | `NewXxx()` function | `NewUserInfoService()` |
| DTOs | Separate `request/` and `respond/` packages | `LoginRequest`, `LoginRespond` |
| ID Generation | Prefixed IDs | `U` (user), `G` (group), `S` (session), `M` (message), `A` (apply) |

### Entity Definition Pattern

```go
type UserInfo struct {
    Id       int64  `gorm:"column:id;primaryKey;comment:自增id"`
    Uuid     string `gorm:"column:uuid;uniqueIndex;type:char(20);comment:用户唯一id"`
    Username string `gorm:"column:username;uniqueIndex;type:varchar(20);not null;comment:账号"`
    // ... more fields with GORM tags
}

func (UserInfo) TableName() string {
    return "user_info"
}
```

### Repository Pattern

```go
// Domain layer (interface)
type UserInfoRepository interface {
    CreateUserInfo(user *entity.UserInfo) error
    GetUserInfoById(id int64) (*entity.UserInfo, error)
}

// Infrastructure layer (implementation)
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

### Service Pattern

```go
// Interface
type UserInfoService interface {
    Login(req request.LoginRequest) (*respond.LoginRespond, error)
}

// Implementation
type userInfoServiceImpl struct {
    repo repository.UserInfoRepository
}

func NewUserInfoService(repo repository.UserInfoRepository) UserInfoService {
    return &userInfoServiceImpl{repo: repo}
}
```

### HTTP Handler Pattern

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

### Error Handling

Use the `xerr` package for structured errors:

```go
// Creating errors
xerr.New(xerr.BadRequest, "用户已存在")
xerr.ErrServerError  // Predefined server error

// Error codes (xerr package)
xerr.OK                  = 200
xerr.BadRequest          = 400
xerr.Unauthorized        = 401
xerr.Forbidden           = 403
xerr.NotFound            = 404
xerr.InternalServerError = 500
```

### HTTP Response Pattern

Use the `back` package:

```go
back.Result(c, data, err)  // Auto-handles success/error
back.Success(c, data)      // Success response
back.Error(c, code, msg)   // Error response
```

### Logging

Use `zlog` package (wraps Zap):

```go
zlog.Info("message")
zlog.Warn("message")
zlog.Error("message")
zlog.Fatal("message")  // Exits application
zlog.Debug("message")
```

### Configuration

TOML format in `configs/config_local.toml`:

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

Access via `config.GetConfig()` singleton.

## Key Patterns to Follow

1. **DDD Layers**: domain → application → infrastructure → interface
2. **Dependency Injection**: Constructor injection, wire in `api/http/https_server.go`
3. **Transaction Handling**: Use `UnitOfWork` pattern for cross-aggregate transactions
4. **JWT Auth**: Middleware extracts `uuid` and `username` into gin.Context
5. **ID Generation**: Use `util.GenerateXxxID()` functions

## Common Mistakes to Avoid

1. **Never** return password fields in responses
2. **Never** log sensitive data (passwords, tokens, API keys)
3. **Always** check `gorm.ErrRecordNotFound` for "not found" cases
4. **Always** use `xerr` for business errors, not raw `errors.New()`
5. **Always** validate input before processing (check empty strings, nil)
6. **Prefer** existing libraries over adding new dependencies

## Testing Guidance

Currently no test files exist. When adding tests:

```go
// File: user_info_service_test.go
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

Run tests: `go test -v ./internal/modules/user/application/service/...`

## API Authentication

Protected routes require `Authorization: Bearer <token>` header:

```bash
curl -X POST http://localhost:8000/contact/getUserList \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"ownerId": "U..."}'
```

## Module-Specific Notes

### AI Module (`internal/modules/ai/`)

- Uses Kafka for async ingest events (outbox pattern)
- Milvus for vector storage
- Embeddings via configurable providers (Ark, OpenAI)
- RAG entities: `AIKnowledgeBase`, `AIKnowledgeSource`, `AIKnowledgeChunk`, `AIVectorRecord`

### Chat Module (`internal/modules/chat/`)

- WebSocket for real-time messaging
- Sessions track conversations (1-1 and group)
- Messages have types: text(0), voice(1), file(2), call(3)

### Contact Module (`internal/modules/contact/`)

- Handles friend relationships and group memberships
- Contact types: user(0), group(1)
- Apply workflow for friend/group requests
