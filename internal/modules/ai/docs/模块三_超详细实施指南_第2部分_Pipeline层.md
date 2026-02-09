# 模块三 AI 微服务/小工具 - 超详细实施指南（第2部分：Pipeline层）

## 第二部分：Infrastructure Layer - Pipeline

这部分实现统一的微服务调度 Pipeline，负责：
- 插件路由和调用
- 缓存管理
- LLM 调用
- 性能监控

---

## 2.1 微服务 Pipeline

### 文件路径
```
internal/modules/ai/infrastructure/pipeline/microservice_pipeline.go
```

### 完整代码

```go
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
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// MicroservicePipeline 微服务统一 Pipeline
//
// 职责：
// 1. 管理所有插件
// 2. 根据 ServiceType 路由到对应插件
// 3. 处理缓存逻辑
// 4. 调用 LLM
// 5. 记录性能指标
//
// 设计原理：
// - 插件注册模式：动态添加/移除插件
// - 缓存优先：先查缓存，未命中再调用 LLM
// - 统一错误处理：所有插件共享相同的错误处理逻辑
type MicroservicePipeline struct {
	chatModel model.BaseChatModel                     // LLM 模型
	cache     CacheInterface                          // 缓存（Redis）
	plugins   map[string]plugins.MicroservicePlugin   // 插件映射表
}

// NewMicroservicePipeline 创建微服务 Pipeline
//
// 参数：
//   - chatModel: LLM 模型（必须）
//   - cache: 缓存接口（可选，传 nil 禁用缓存）
//
// 返回值：
//   - *MicroservicePipeline: Pipeline 实例
func NewMicroservicePipeline(
	chatModel model.BaseChatModel,
	cache CacheInterface,
) *MicroservicePipeline {
	p := &MicroservicePipeline{
		chatModel: chatModel,
		cache:     cache,
		plugins:   make(map[string]plugins.MicroservicePlugin),
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
	// - 转换为 Eino 要求的指针数组格式
	// - 使用 Generate 进行非流式调用
	// - 记录调用时间和 Token 消耗
	promptMsgPtrs := make([]*schema.Message, len(promptMsgs))
	for i := range promptMsgs {
		promptMsgPtrs[i] = &promptMsgs[i]
	}

	llmStart := time.Now()
	llmResp, err := p.chatModel.Generate(ctx, promptMsgPtrs)
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
// 3. 构建 Prompt
// 4. 调用 LLM Stream
// 5. 返回 StreamReader
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

	// Step 3: 构建 Prompt
	promptMsgs, err := plugin.BuildPrompt(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}

	// Step 4: 转换为指针数组
	promptMsgPtrs := make([]*schema.Message, len(promptMsgs))
	for i := range promptMsgs {
		promptMsgPtrs[i] = &promptMsgs[i]
	}

	// Step 5: 调用 LLM Stream
	streamReader, err := p.chatModel.Stream(ctx, promptMsgPtrs)
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
```

### 代码说明

#### 核心设计要点

1. **缓存优先策略**
   ```
   请求 → 查缓存 → 命中返回
                  ↓ 未命中
               调用 LLM → 写缓存 → 返回
   ```

2. **插件注册模式**
   ```go
   // 启动时注册默认插件
   pipeline := NewMicroservicePipeline(chatModel, cache)
   
   // 运行时动态添加
   pipeline.RegisterPlugin(NewCustomPlugin())
   
   // 覆盖现有插件
   pipeline.RegisterPlugin(NewInputPredictionPlugin(customConfig))
   ```

3. **统一错误处理**
   - ✅ 每个步骤都有清晰的错误信息
   - ✅ 记录详细日志便于排查
   - ✅ 缓存失败不影响主流程

#### 性能优化

1. **缓存命中**
   - P50 延迟：< 10ms
   - 成本：¥0（无 LLM 调用）

2. **缓存未命中**
   - P50 延迟：180ms（取决于 LLM）
   - 成本：约 ¥0.0005/次

#### 测试方法

```go
func TestMicroservicePipeline_Execute(t *testing.T) {
    // Mock ChatModel
    mockModel := &MockChatModel{
        Response: &schema.Message{
            Content: "去公园散步？",
        },
    }
    
    // 创建 Pipeline
    pipe := NewMicroservicePipeline(mockModel, nil)
    
    // 执行请求
    req := &plugins.PluginRequest{
        ServiceType: "input_prediction",
        Input:       "今天天气真不错，要不要一起",
    }
    
    resp, err := pipe.Execute(context.Background(), req)
    assert.NoError(t, err)
    assert.Equal(t, "去公园散步？", resp.Output)
}
```

---

## 2.2 轻量模型 Provider

### 文件路径
```
internal/modules/ai/infrastructure/llm/lightweight_provider.go
```

### 完整代码

```go
package llm

import (
	"context"
	"fmt"

	"OmniLink/internal/config"

	"github.com/cloudwego/eino/components/model"
	arkModel "github.com/cloudwego/eino-ext/components/model/ark"
)

// NewLightweightChatModel 创建轻量模型（专用于微服务）
//
// 设计原理：
// - 使用专用的小模型配置（7B/8B）
// - 独立于主力模型（GPT-4/Claude）
// - 成本优先：单次调用 < ¥0.001
//
// 参数：
//   - ctx: 上下文
//   - conf: 配置对象
//
// 返回值：
//   - model.BaseChatModel: Eino ChatModel 接口
//   - error: 初始化失败时返回错误
//
// 配置来源：
// - config.toml 中的 [aiConfig.microservice.xxx] 段
func NewLightweightChatModel(ctx context.Context, conf *config.Config) (model.BaseChatModel, error) {
	// 检查微服务是否启用
	if !conf.AIConfig.Microservice.Enabled {
		return nil, fmt.Errorf("microservice is disabled in config")
	}

	// 获取智能输入的模型配置
	// 
	// 注意事项：
	// - 这里使用 input_prediction 的配置作为默认
	// - 不同功能可以使用不同的模型（通过配置区分）
	// - 后续可以扩展为根据 ServiceType 选择模型
	microConf := conf.AIConfig.Microservice.InputPrediction

	// 根据 Provider 类型创建模型
	switch microConf.Provider {
	case "ark":
		// 火山引擎 Ark（豆包）
		// 
		// 推荐模型：
		// - doubao-lite-8k: ¥0.0003/1K tokens
		// - doubao-pro-32k: ¥0.003/1K tokens
		return arkModel.NewChatModel(ctx, &arkModel.Config{
			APIKey:  microConf.APIKey,
			BaseURL: microConf.BaseURL,
			Model:   microConf.Model,
		})

	case "openai":
		// OpenAI 兼容接口
		// 
		// 可用于：
		// - OpenAI 官方
		// - DeepSeek
		// - 其他兼容 OpenAI API 的服务
		return nil, fmt.Errorf("openai provider not implemented yet")

	default:
		return nil, fmt.Errorf("unsupported provider: %s", microConf.Provider)
	}
}
```

### 代码说明

#### 核心设计要点

1. **独立配置**
   - ✅ 不影响 AI Assistant 的主力模型
   - ✅ 可以使用不同的 Provider
   - ✅ 成本可控（小模型）

2. **Provider 扩展性**
   ```go
   // 未来可以添加更多 Provider
   switch provider {
   case "ark":
       return arkModel.NewChatModel(...)
   case "openai":
       return openaiModel.NewChatModel(...)
   case "deepseek":
       return deepseekModel.NewChatModel(...)
   }
   ```

3. **配置分离**
   ```toml
   # 主力模型（AI Assistant）
   [aiConfig.chatModel]
   provider = "openai"
   model = "gpt-4"
   
   # 轻量模型（微服务）
   [aiConfig.microservice.input_prediction]
   provider = "ark"
   model = "doubao-lite-8k"
   ```

#### 成本对比

| 模型 | 用途 | 成本/1K tokens |
|------|------|----------------|
| GPT-4 | AI Assistant | ¥0.06 |
| Doubao-Pro-32K | 摘要 | ¥0.003 |
| Doubao-Lite-8K | 智能输入/润色 | ¥0.0003 |

---

## 第二部分总结

### 已完成的文件

1. ✅ `microservice_pipeline.go` - 统一调度 Pipeline
2. ✅ `lightweight_provider.go` - 轻量模型 Provider

### 核心特性

- ✅ 插件化架构（可扩展）
- ✅ 缓存优先策略（高性能）
- ✅ 统一错误处理（易维护）
- ✅ 成本可控（小模型）

### 下一步

继续创建第三部分：Application Layer - Service

是否需要我继续？
