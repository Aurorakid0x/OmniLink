package pipeline

import (
	"context"
	"fmt"
	"time"

	"OmniLink/internal/modules/ai/infrastructure/plugins"
	"OmniLink/pkg/zlog"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// CacheInterface 缓存接口定义
//
// 设计原理：
// - 抽象缓存接口，便于替换实现（Redis/内存/其他）
// - 支持 Get/Set/Delete 基本操作
type CacheInterface interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) (int64, error)
}

// MicroservicePipeline 微服务统一 Pipeline
//
// 职责：
// 1. 管理所有插件
// 2. 根据 ServiceType 路由到对应插件
// 3. 处理缓存逻辑
// 4. 调用 LLM（每个服务可用不同模型）
// 5. 记录性能指标
//
// 设计原理：
// - 插件注册模式：动态添加/移除插件
// - 多模型支持：每个服务独立配置模型（input_prediction/polish/digest）
// - 缓存优先：先查缓存，未命中再调用 LLM。
// - 统一错误处理：所有插件共享相同的错误处理逻辑
type MicroservicePipeline struct {
	chatModels map[string]model.BaseChatModel        // LLM 模型映射（key: service_type）
	cache      CacheInterface                        // 缓存（Redis）
	plugins    map[string]plugins.MicroservicePlugin // 插件映射表
}

// NewMicroservicePipeline 创建微服务 Pipeline
//
// 参数：
//   - chatModels: LLM 模型映射（必须）
//     格式：map[string]model.BaseChatModel
//     示例：{
//     "input_prediction": inputModel,
//     "polish": polishModel,
//     "digest": digestModel,
//     }
//   - cache: 缓存接口（可选，传 nil 禁用缓存）
//
// 返回值：
//   - *MicroservicePipeline: Pipeline 实例
//
// 设计要点：
// - 支持每个服务使用不同的模型和配置
// - 模型映射在启动时创建，运行时只读
func NewMicroservicePipeline(
	chatModels map[string]model.BaseChatModel,
	cache CacheInterface,
) *MicroservicePipeline {
	p := &MicroservicePipeline{
		chatModels: chatModels,
		cache:      cache,
		plugins:    make(map[string]plugins.MicroservicePlugin),
	}

	// 注册默认插件
	//
	// 设计要点：
	// - 使用默认配置（nil）
	// - 后续可以通过 RegisterPlugin 覆盖
	p.RegisterPlugin(plugins.NewInputPredictionPlugin(nil))
	p.RegisterPlugin(plugins.NewPolishPlugin(nil))
	p.RegisterPlugin(plugins.NewDigestPlugin(nil))

	return p
}

// RegisterPlugin 注册插件
//
// 参数：
//   - plugin: 插件实例
//
// 使用场景：
// - 启动时注册默认插件
// - 运行时动态添加新插件
// - 覆盖现有插件（如替换配置）
func (p *MicroservicePipeline) RegisterPlugin(plugin plugins.MicroservicePlugin) {
	serviceType := plugin.GetServiceType()
	p.plugins[serviceType] = plugin

	zlog.Info("plugin registered",
		zap.String("service_type", serviceType))
}

// Execute 执行微服务调用（非流式）
//
// 完整流程：
// 1. 获取插件
// 2. 参数验证
// 3. 检查缓存
// 4. 构建 Prompt
// 5. 调用 LLM
// 6. 解析响应
// 7. 写入缓存
// 8. 记录日志
//
// 参数：
//   - ctx: 上下文
//   - req: 插件请求
//
// 返回值：
//   - *plugins.PluginResponse: 插件响应
//   - error: 错误信息
func (p *MicroservicePipeline) Execute(ctx context.Context, req *plugins.PluginRequest) (*plugins.PluginResponse, error) {
	startTime := time.Now()

	// ========== Step 1: 获取插件 ==========
	//
	// 设计要点：
	// - 根据 ServiceType 从映射表获取插件
	// - 如果插件不存在，返回错误
	plugin, ok := p.plugins[req.ServiceType]
	if !ok {
		return nil, fmt.Errorf("unknown service type: %s", req.ServiceType)
	}

	// ========== Step 2: 参数验证 ==========
	//
	// 设计要点：
	// - 在调用 LLM 之前验证，避免浪费 API 调用
	// - 每个插件自己定义验证规则
	if err := plugin.Validate(ctx, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// ========== Step 3: 检查缓存 ==========
	//
	// 设计要点：
	// - 先查缓存，命中则直接返回
	// - 缓存 Key 由插件自己生成
	// - 如果缓存不可用，跳过此步骤
	cacheKey := plugin.GetCacheKey(ctx, req)
	if cacheKey != "" && p.cache != nil {
		if cached, err := p.cache.Get(ctx, cacheKey); err == nil && cached != "" {
			zlog.Info("cache hit",
				zap.String("service_type", req.ServiceType),
				zap.String("cache_key", cacheKey))

			return &plugins.PluginResponse{
				Output:   cached,
				CacheHit: true,
			}, nil
		}
	}

	// ========== Step 4: 构建 Prompt ==========
	//
	// 设计要点：
	// - 每个插件根据自己的需求构建 Prompt
	// - 返回 Eino 标准消息格式
	promptMsgs, err := plugin.BuildPrompt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}

	// ========== Step 5: 调用 LLM ==========
	//
	// 设计要点：
	// - 根据 ServiceType 选择对应的模型
	// - 每个服务可以使用不同的模型配置
	// - 转换为 Eino 要求的指针数组格式
	// - 使用 Generate 进行非流式调用
	// - 记录调用时间和 Token 消耗

	// 获取当前服务的模型
	chatModel, ok := p.chatModels[req.ServiceType]
	if !ok {
		return nil, fmt.Errorf("no model configured for service type: %s", req.ServiceType)
	}

	promptMsgPtrs := make([]*schema.Message, len(promptMsgs))
	for i := range promptMsgs {
		promptMsgPtrs[i] = &promptMsgs[i]
	}

	llmStart := time.Now()
	llmResp, err := chatModel.Generate(ctx, promptMsgPtrs)
	llmMs := time.Since(llmStart).Milliseconds()

	if err != nil {
		zlog.Error("llm generate failed",
			zap.Error(err),
			zap.String("service_type", req.ServiceType))
		return nil, fmt.Errorf("llm generate failed: %w", err)
	}

	// ========== Step 6: 解析响应 ==========
	//
	// 设计要点：
	// - 每个插件自己解析 LLM 输出
	// - 处理 JSON 解析失败等异常
	resp, err := plugin.ParseResponse(ctx, llmResp.Content, req)
	if err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}

	// ========== Step 7: 填充 Token 统计 ==========
	//
	// 设计要点：
	// - 从 LLM 响应中提取 Token 使用量
	// - 用于成本统计和性能分析
	if llmResp.ResponseMeta != nil && llmResp.ResponseMeta.Usage != nil {
		resp.TokensUsed = llmResp.ResponseMeta.Usage.TotalTokens
	}

	// ========== Step 8: 写入缓存 ==========
	//
	// 设计要点：
	// - 只缓存成功的响应
	// - 使用插件定义的 TTL
	// - 缓存写入失败不影响主流程
	if cacheKey != "" && p.cache != nil {
		ttl := time.Duration(plugin.GetCacheTTL()) * time.Second
		if err := p.cache.Set(ctx, cacheKey, resp.Output, ttl); err != nil {
			zlog.Warn("cache set failed",
				zap.Error(err),
				zap.String("cache_key", cacheKey))
		}
	}

	// ========== Step 9: 记录日志 ==========
	//
	// 设计要点：
	// - 记录性能指标（延迟、Token）
	// - 记录缓存命中情况
	// - 用于后续分析和优化
	totalLatencyMs := time.Since(startTime).Milliseconds()
	zlog.Info("microservice execute done",
		zap.String("service_type", req.ServiceType),
		zap.Int64("total_latency_ms", totalLatencyMs),
		zap.Int64("llm_latency_ms", llmMs),
		zap.Int("tokens", resp.TokensUsed),
		zap.Bool("cache_hit", resp.CacheHit))

	return resp, nil
}

// ExecuteStream 执行微服务调用（流式）
//
// 流程：
// 1. 获取插件
// 2. 参数验证
// 3. 选择对应模型
// 4. 构建 Prompt
// 5. 调用 LLM Stream
// 6. 返回 StreamReader
//
// 注意事项：
// - 流式模式下不检查缓存（实时性优先）
// - 调用方负责读取 StreamReader 并处理结果
// - 持久化由调用方完成
func (p *MicroservicePipeline) ExecuteStream(ctx context.Context, req *plugins.PluginRequest) (*schema.StreamReader[*schema.Message], error) {
	// Step 1: 获取插件
	plugin, ok := p.plugins[req.ServiceType]
	if !ok {
		return nil, fmt.Errorf("unknown service type: %s", req.ServiceType)
	}

	// Step 2: 参数验证
	if err := plugin.Validate(ctx, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Step 3: 获取当前服务的模型
	chatModel, ok := p.chatModels[req.ServiceType]
	if !ok {
		return nil, fmt.Errorf("no model configured for service type: %s", req.ServiceType)
	}

	// Step 4: 构建 Prompt
	promptMsgs, err := plugin.BuildPrompt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}

	// Step 5: 转换为指针数组
	promptMsgPtrs := make([]*schema.Message, len(promptMsgs))
	for i := range promptMsgs {
		promptMsgPtrs[i] = &promptMsgs[i]
	}

	// Step 6: 调用 LLM Stream
	streamReader, err := chatModel.Stream(ctx, promptMsgPtrs)
	if err != nil {
		zlog.Error("llm stream failed",
			zap.Error(err),
			zap.String("service_type", req.ServiceType))
		return nil, fmt.Errorf("llm stream failed: %w", err)
	}

	zlog.Info("microservice stream started",
		zap.String("service_type", req.ServiceType))

	return streamReader, nil
}
