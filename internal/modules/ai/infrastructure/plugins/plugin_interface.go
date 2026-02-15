package plugins

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// MicroservicePlugin 微服务插件接口
//
// 设计原理：
// 1. 插件化架构：每个 AI 微服务功能都是一个独立插件
// 2. 统一接口：所有插件实现相同接口，方便扩展和管理
// 3. 职责分离：每个插件只负责自己的业务逻辑
//
// 使用场景：
// - 新增功能时，只需实现此接口即可
// - Pipeline 通过此接口调用插件，无需关心具体实现
type MicroservicePlugin interface {
	// GetServiceType 获取服务类型
	//
	// 返回值：input_prediction / polish / digest 等
	// 用途：Pipeline 根据此值路由到对应插件
	GetServiceType() string

	// BuildPrompt 构建 LLM Prompt
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 插件请求（包含用户输入和上下文）
	//
	// 返回值：
	//   - []schema.Message: Eino 标准消息格式（System/User/Assistant）
	//   - error: 构建失败时返回错误
	//
	// 设计要点：
	// - 每个插件根据自己的需求构建不同的 Prompt
	// - 返回 Eino 标准格式，方便 Pipeline 统一处理
	BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error)

	// ParseResponse 解析 LLM 响应
	//
	// 参数：
	//   - ctx: 上下文
	//   - llmOutput: LLM 原始输出（纯文本或 JSON）
	//   - req: 原始请求（用于上下文信息）
	//
	// 返回值：
	//   - *PluginResponse: 标准化的插件响应
	//   - error: 解析失败时返回错误
	//
	// 设计要点：
	// - 将 LLM 的原始输出转换为结构化数据
	// - 处理 JSON 解析失败等异常情况
	ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error)

	// Validate 验证请求参数
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 插件请求
	//
	// 返回值：
	//   - error: 验证失败时返回错误，成功返回 nil
	//
	// 设计要点：
	// - 在调用 LLM 之前验证参数，避免浪费 API 调用
	// - 每个插件可以定义自己的验证规则
	Validate(ctx context.Context, req *PluginRequest) error

	// GetCacheKey 获取缓存 Key
	//
	// 参数：
	//   - ctx: 上下文
	//   - req: 插件请求
	//
	// 返回值：
	//   - string: 缓存 Key（空字符串表示不缓存）
	//
	// 设计要点：
	// - 相同输入生成相同 Key，实现缓存复用
	// - 通常使用 MD5(input + context) 作为 Key
	// - 返回空字符串可以禁用缓存
	GetCacheKey(ctx context.Context, req *PluginRequest) string

	// GetCacheTTL 获取缓存 TTL（秒）
	//
	// 返回值：
	//   - int: 缓存存活时间（秒）
	//
	// 设计要点：
	// - 不同功能的缓存时间不同
	// - 智能输入：300s（5分钟）
	// - 润色：1800s（30分钟）
	// - 摘要：600s（10分钟）
	GetCacheTTL() int
}

// PluginRequest 插件请求
//
// 设计原理：
// - 统一的请求结构，所有插件共享
// - 使用 map 存储上下文，保证灵活性
type PluginRequest struct {
	TenantUserID string                 `json:"tenant_user_id"` // 用户 ID（租户隔离）
	ServiceType  string                 `json:"service_type"`   // 服务类型
	Input        string                 `json:"input"`          // 输入文本
	Context      map[string]interface{} `json:"context"`        // 上下文信息（如历史消息）
	CustomConfig map[string]interface{} `json:"custom_config"`  // 自定义配置（可选）
}

// PluginResponse 插件响应
//
// 设计原理：
// - 统一的响应结构
// - 包含性能指标（缓存命中、Token 消耗）
type PluginResponse struct {
	Output     string                 `json:"output"`      // 输出文本或 JSON
	Metadata   map[string]interface{} `json:"metadata"`    // 元数据（如选项数量）
	CacheHit   bool                   `json:"cache_hit"`   // 是否命中缓存
	TokensUsed int                    `json:"tokens_used"` // 消耗 Token 数
}
