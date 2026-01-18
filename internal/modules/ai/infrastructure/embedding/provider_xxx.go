package embedding

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"OmniLink/internal/config"

	arkEmbed "github.com/cloudwego/eino-ext/components/embedding/ark"
	openaIEmbed "github.com/cloudwego/eino-ext/components/embedding/openai"
	"github.com/cloudwego/eino/components/embedding"
)

type EmbedderMeta struct {
	Provider string
	Model    string
	Dim      int
}

func NewEmbedderFromConfig(ctx context.Context, conf *config.Config) (embedding.Embedder, EmbedderMeta, error) {
	if conf == nil {
		return nil, EmbedderMeta{}, fmt.Errorf("nil config")
	}

	dim := conf.MilvusConfig.VectorDim
	provider := strings.ToLower(strings.TrimSpace(conf.AIConfig.Embedding.Provider))
	model := strings.TrimSpace(conf.AIConfig.Embedding.Model)
	if conf.AIConfig.Embedding.Dimensions > 0 {
		dim = conf.AIConfig.Embedding.Dimensions
	}

	switch provider {
	case "", "mock":
		if model == "" {
			model = "mock"
		}
		return NewMockEmbedder(dim), EmbedderMeta{Provider: "mock", Model: model, Dim: dim}, nil
	case "openai":
		apiKey := strings.TrimSpace(conf.AIConfig.Embedding.APIKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
		}
		if model == "" {
			model = strings.TrimSpace(os.Getenv("OPENAI_EMBED_MODEL"))
		}
		baseURL := strings.TrimSpace(conf.AIConfig.Embedding.BaseURL)
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
		}
		if apiKey == "" || model == "" {
			return nil, EmbedderMeta{}, fmt.Errorf("openai embedding missing apiKey/model")
		}

		timeout := 30 * time.Second
		if conf.AIConfig.Embedding.TimeoutSeconds > 0 {
			timeout = time.Duration(conf.AIConfig.Embedding.TimeoutSeconds) * time.Second
		}

		localDim := dim
		cfg := &openaIEmbed.EmbeddingConfig{
			APIKey:     apiKey,
			Model:      model,
			BaseURL:    baseURL,
			Timeout:    timeout,
			Dimensions: &localDim,
		}
		em, err := openaIEmbed.NewEmbedder(ctx, cfg)
		if err != nil {
			return nil, EmbedderMeta{}, err
		}
		return em, EmbedderMeta{Provider: "openai", Model: model, Dim: dim}, nil
	case "ark":
		apiKey := strings.TrimSpace(conf.AIConfig.Embedding.APIKey)
		accessKey := strings.TrimSpace(conf.AIConfig.Embedding.AccessKey)
		secretKey := strings.TrimSpace(conf.AIConfig.Embedding.SecretKey)
		if apiKey == "" {
			apiKey = strings.TrimSpace(os.Getenv("ARK_API_KEY"))
		}
		if accessKey == "" {
			accessKey = strings.TrimSpace(os.Getenv("ARK_ACCESS_KEY"))
		}
		if secretKey == "" {
			secretKey = strings.TrimSpace(os.Getenv("ARK_SECRET_KEY"))
		}
		if model == "" {
			model = strings.TrimSpace(os.Getenv("ARK_EMBED_MODEL"))
		}
		baseURL := strings.TrimSpace(conf.AIConfig.Embedding.BaseURL)
		region := strings.TrimSpace(conf.AIConfig.Embedding.Region)
		if baseURL == "" {
			baseURL = strings.TrimSpace(os.Getenv("ARK_BASE_URL"))
		}
		if region == "" {
			region = strings.TrimSpace(os.Getenv("ARK_REGION"))
		}
		if apiKey == "" && (accessKey == "" || secretKey == "") {
			return nil, EmbedderMeta{}, fmt.Errorf("ark embedding missing apiKey or accessKey/secretKey")
		}
		if model == "" {
			return nil, EmbedderMeta{}, fmt.Errorf("ark embedding missing model")
		}

		timeout := 30 * time.Second
		if conf.AIConfig.Embedding.TimeoutSeconds > 0 {
			timeout = time.Duration(conf.AIConfig.Embedding.TimeoutSeconds) * time.Second
		}
		retryTimes := 2
		if conf.AIConfig.Embedding.RetryTimes > 0 {
			retryTimes = conf.AIConfig.Embedding.RetryTimes
		}
		cfg := &arkEmbed.EmbeddingConfig{
			APIKey:     apiKey,
			AccessKey:  accessKey,
			SecretKey:  secretKey,
			Model:      model,
			BaseURL:    baseURL,
			Region:     region,
			Timeout:    &timeout,
			RetryTimes: &retryTimes,
		}

		em, err := arkEmbed.NewEmbedder(ctx, cfg)
		if err != nil {
			return nil, EmbedderMeta{}, err
		}
		return em, EmbedderMeta{Provider: "ark", Model: model, Dim: dim}, nil
	default:
		return nil, EmbedderMeta{}, fmt.Errorf("unknown embedding provider: %s", provider)
	}
}

var _ embedding.Embedder = (*MockEmbedder)(nil)
