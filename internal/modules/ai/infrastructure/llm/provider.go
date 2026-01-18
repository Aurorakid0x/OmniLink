package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"OmniLink/internal/config"

	arkModel "github.com/cloudwego/eino-ext/components/model/ark"
	openaiModel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

type ChatModelMeta struct {
	Provider string
	Model    string
}

func NewChatModelFromConfig(ctx context.Context, conf *config.Config) (model.BaseChatModel, ChatModelMeta, error) {
	if conf == nil {
		return nil, ChatModelMeta{}, fmt.Errorf("nil config")
	}

	provider := strings.ToLower(strings.TrimSpace(conf.AIConfig.ChatModel.Provider))
	modelName := strings.TrimSpace(conf.AIConfig.ChatModel.Model)

	switch provider {
	case "", "disabled", "none":
		return nil, ChatModelMeta{}, fmt.Errorf("chat model provider not configured")

	case "openai":
		apiKey := strings.TrimSpace(conf.AIConfig.ChatModel.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		}
		if modelName == "" {
			modelName = strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
		}
		baseURL := strings.TrimSpace(conf.AIConfig.ChatModel.BaseURL)
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
		}

		if apiKey == "" || modelName == "" {
			return nil, ChatModelMeta{}, fmt.Errorf("openai chat model missing apiKey/model")
		}

		timeout := 2 * time.Minute
		if conf.AIConfig.ChatModel.TimeoutSeconds > 0 {
			timeout = time.Duration(conf.AIConfig.ChatModel.TimeoutSeconds) * time.Second
		}

		cm, err := openaiModel.NewChatModel(ctx, &openaiModel.ChatModelConfig{
			APIKey:     apiKey,
			Model:      modelName,
			BaseURL:    baseURL,
			ByAzure:    conf.AIConfig.ChatModel.ByAzure,
			APIVersion: strings.TrimSpace(conf.AIConfig.ChatModel.AzureAPIVersion),
			Timeout:    timeout,
		})
		if err != nil {
			return nil, ChatModelMeta{}, err
		}
		return cm, ChatModelMeta{Provider: "openai", Model: modelName}, nil

	case "ark":
		apiKey := strings.TrimSpace(conf.AIConfig.ChatModel.APIKey)
		accessKey := strings.TrimSpace(conf.AIConfig.ChatModel.AccessKey)
		secretKey := strings.TrimSpace(conf.AIConfig.ChatModel.SecretKey)

		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("ARK_API_KEY"))
		}
		if accessKey == "" {
			accessKey = strings.TrimSpace(os.Getenv("ARK_ACCESS_KEY"))
		}
		if secretKey == "" {
			secretKey = strings.TrimSpace(os.Getenv("ARK_SECRET_KEY"))
		}
		if modelName == "" {
			modelName = strings.TrimSpace(os.Getenv("ARK_MODEL_ID"))
		}

		baseURL := strings.TrimSpace(conf.AIConfig.ChatModel.BaseURL)
		region := strings.TrimSpace(conf.AIConfig.ChatModel.Region)
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("ARK_BASE_URL"))
		}
		if region == "" {
			region = strings.TrimSpace(os.Getenv("ARK_REGION"))
		}

		if apiKey == "" && (accessKey == "" || secretKey == "") {
			return nil, ChatModelMeta{}, fmt.Errorf("ark chat model missing apiKey or accessKey/secretKey")
		}
		if modelName == "" {
			return nil, ChatModelMeta{}, fmt.Errorf("ark chat model missing model")
		}

		timeout := 2 * time.Minute
		if conf.AIConfig.ChatModel.TimeoutSeconds > 0 {
			timeout = time.Duration(conf.AIConfig.ChatModel.TimeoutSeconds) * time.Second
		}
		retryTimes := 2
		if conf.AIConfig.ChatModel.RetryTimes > 0 {
			retryTimes = conf.AIConfig.ChatModel.RetryTimes
		}

		cm, err := arkModel.NewChatModel(ctx, &arkModel.ChatModelConfig{
			APIKey:     apiKey,
			AccessKey:  accessKey,
			SecretKey:  secretKey,
			Model:      modelName,
			BaseURL:    baseURL,
			Region:     region,
			Timeout:    &timeout,
			RetryTimes: &retryTimes,
		})
		if err != nil {
			return nil, ChatModelMeta{}, err
		}
		return cm, ChatModelMeta{Provider: "ark", Model: modelName}, nil

	default:
		return nil, ChatModelMeta{}, fmt.Errorf("unknown chat model provider: %s", provider)
	}
}
