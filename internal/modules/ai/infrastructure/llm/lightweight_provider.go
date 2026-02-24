package llm

import (
	"context"
	"fmt"

	"OmniLink/internal/config"
	"OmniLink/pkg/zlog"

	//arkModel "github.com/cloudwego/eino-ext/components/model/ark"
	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"
)

// NewMicroserviceChatModels 创建微服务专用的多模型映射
//
// 设计原理：
// - 每个服务（input_prediction/polish/digest）使用独立的模型实例
// - 支持不同的 Provider 和模型配置
// - 独立于主力模型（GPT-4/Claude），成本可控
//
// 参数：
//   - ctx: 上下文
//   - conf: 配置对象
//
// 返回值：
//   - map[string]model.BaseChatModel: 服务类型到模型的映射
//     格式：{
//     "input_prediction": inputModel,
//     "polish": polishModel,
//     "digest": digestModel,
//     }
//   - error: 初始化失败时返回错误
//
// 配置来源：
// - config.toml 中的 [aiConfig.microservice.xxx] 段
func NewMicroserviceChatModels(ctx context.Context, conf *config.Config) (map[string]model.BaseChatModel, error) {
	// 检查微服务是否启用
	if !conf.AIConfig.Microservice.Enabled {
		return nil, fmt.Errorf("microservice is disabled in config")
	}

	models := make(map[string]model.BaseChatModel)

	// ========== 1. 创建智能输入预测模型 ==========
	inputModel, err := createModelFromConfig(ctx, "input_prediction", conf.AIConfig.Microservice.InputPrediction)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_prediction model: %w", err)
	}
	models["input_prediction"] = inputModel
	zlog.Info("microservice model created",
		zap.String("service_type", "input_prediction"),
		zap.String("provider", conf.AIConfig.Microservice.InputPrediction.Provider),
		zap.String("model", conf.AIConfig.Microservice.InputPrediction.Model))

	// ========== 2. 创建文本润色模型 ==========
	polishModel, err := createModelFromConfig(ctx, "polish", conf.AIConfig.Microservice.Polish)
	if err != nil {
		return nil, fmt.Errorf("failed to create polish model: %w", err)
	}
	models["polish"] = polishModel
	zlog.Info("microservice model created",
		zap.String("service_type", "polish"),
		zap.String("provider", conf.AIConfig.Microservice.Polish.Provider),
		zap.String("model", conf.AIConfig.Microservice.Polish.Model))

	// ========== 3. 创建消息摘要模型 ==========
	digestModel, err := createModelFromConfig(ctx, "digest", conf.AIConfig.Microservice.Digest)
	if err != nil {
		return nil, fmt.Errorf("failed to create digest model: %w", err)
	}
	models["digest"] = digestModel
	zlog.Info("microservice model created",
		zap.String("service_type", "digest"),
		zap.String("provider", conf.AIConfig.Microservice.Digest.Provider),
		zap.String("model", conf.AIConfig.Microservice.Digest.Model))

	return models, nil
}

// createModelFromConfig 根据配置创建模型实例
//
// 参数：
//   - ctx: 上下文
//   - serviceType: 服务类型（用于日志）
//   - conf: 服务模型配置
//
// 返回值：
//   - model.BaseChatModel: Eino ChatModel 接口
//   - error: 初始化失败时返回错误
func createModelFromConfig(ctx context.Context, serviceType string, conf config.ServiceModelConfig) (model.BaseChatModel, error) {
	// 根据 Provider 类型创建模型
	switch conf.Provider {
	case "ark":
		// 火山引擎 Ark（豆包）
		//
		// 推荐模型：
		// - doubao-lite-8k: ¥0.0003/1K tokens
		// - doubao-pro-32k: ¥0.003/1K tokens
		// return arkModel.NewChatModel(ctx, &arkModel.Config{
		// 	APIKey:  conf.APIKey,
		// 	BaseURL: conf.BaseURL,
		// 	Model:   conf.Model,
		// })
		return nil, fmt.Errorf("ark provider not implemented yet")
	case "openai":
		// OpenAI 兼容接口
		//
		// 可用于：
		// - OpenAI 官方
		// - DeepSeek
		// - 其他兼容 OpenAI API 的服务
		return nil, fmt.Errorf("openai provider not implemented yet")

	default:
		return nil, fmt.Errorf("unsupported provider: %s", conf.Provider)
	}
}
