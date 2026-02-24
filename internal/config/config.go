package config

import (
	"log"

	"github.com/BurntSushi/toml"
	//"time"
)

type MainConfig struct {
	AppName string `toml:"appName"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
}

type MysqlConfig struct {
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	User         string `toml:"user"`
	Password     string `toml:"password"`
	DatabaseName string `toml:"databaseName"`
}

type LogConfig struct {
	LogPath string `toml:"logPath"`
}

type JwtConfig struct {
	Key         string `toml:"key"`
	ExpireHours int    `toml:"expireHours"`
	Issuer      string `toml:"issuer"`
}

type MilvusConfig struct {
	Address        string `toml:"address"`
	Username       string `toml:"username"`
	Password       string `toml:"password"`
	DBName         string `toml:"dbName"`
	CollectionName string `toml:"collectionName"`
	VectorDim      int    `toml:"vectorDim"`
	MetricType     string `toml:"metricType"`
}

type KafkaConfig struct {
	Brokers         []string `toml:"brokers"`
	ClientID        string   `toml:"clientID"`
	IngestTopic     string   `toml:"ingestTopic"`
	ConsumerGroupID string   `toml:"consumerGroupID"`
	Partitions      int32    `toml:"partitions"`
	Replication     int16    `toml:"replication"`
}

type AIEmbeddingConfig struct {
	Provider        string `toml:"provider"`
	APIKey          string `toml:"apiKey"`
	AccessKey       string `toml:"accessKey"`
	SecretKey       string `toml:"secretKey"`
	BaseURL         string `toml:"baseURL"`
	Region          string `toml:"region"`
	Model           string `toml:"model"`
	Dimensions      int    `toml:"dimensions"`
	TimeoutSeconds  int    `toml:"timeoutSeconds"`
	RetryTimes      int    `toml:"retryTimes"`
	User            string `toml:"user"`
	ByAzure         bool   `toml:"byAzure"`
	AzureAPIVersion string `toml:"azureApiVersion"`
}

type AIChatModelConfig struct {
	Provider        string `toml:"provider"`
	APIKey          string `toml:"apiKey"`
	AccessKey       string `toml:"accessKey"`
	SecretKey       string `toml:"secretKey"`
	BaseURL         string `toml:"baseURL"`
	Region          string `toml:"region"`
	Model           string `toml:"model"`
	TimeoutSeconds  int    `toml:"timeoutSeconds"`
	RetryTimes      int    `toml:"retryTimes"`
	ByAzure         bool   `toml:"byAzure"`
	AzureAPIVersion string `toml:"azureApiVersion"`
}

type AIConfig struct {
	Embedding AIEmbeddingConfig `toml:"embedding"`
	ChatModel AIChatModelConfig `toml:"chatModel"`

	// ========== 新增：微服务配置 ==========
	Microservice MicroserviceConfig `toml:"microservice"`
}

// MCPBuiltinServerConfig 内置 MCP Server 配置
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

// MCPConfig MCP 配置
type MCPConfig struct {
	Enabled                  bool                   `toml:"enabled"`
	ToolCallTimeoutSeconds   int                    `toml:"toolCallTimeoutSeconds"`
	ServerInitTimeoutSeconds int                    `toml:"serverInitTimeoutSeconds"`
	BuiltinServer            MCPBuiltinServerConfig `toml:"builtinServer"`
}

type RedisConfig struct {
	Host         string `toml:"host"`
	Port         int    `toml:"port"`
	Password     string `toml:"password"`
	DB           int    `toml:"db"`
	PoolSize     int    `toml:"poolSize"`
	MinIdleConns int    `toml:"minIdleConns"`
}

type Config struct {
	MainConfig   `toml:"mainConfig"`
	MysqlConfig  `toml:"mysqlConfig"`
	JwtConfig    `toml:"jwtConfig"`
	MilvusConfig `toml:"milvusConfig"`
	KafkaConfig  `toml:"kafkaConfig"`
	AIConfig     `toml:"aiConfig"`
	LogConfig    `toml:"logConfig"`
	MCPConfig    `toml:"mcpConfig"`
	RedisConfig  `toml:"redisConfig"`
}

var config *Config

func LoadConfig() error {

	configPath := "configs/config_local.toml"
	// 本地部署
	// if _, err := toml.DecodeFile("C:\\Users\\chenjun\\goProject\\OmniLink\\configs\\config_local.toml", config); err != nil {
	// 	log.Fatal(err.Error())
	// 	return err
	// }
	if _, err := toml.DecodeFile(configPath, config); err != nil {
		log.Printf("加载配置文件失败: %v, 尝试使用默认设置", err)
		return err
	}
	// Ubuntu22.04云服务器部署
	//if _, err := toml.DecodeFile("/root/project/KamaChat/configs/config_local.toml", config); err != nil {
	//	log.Fatal(err.Error())
	//	return err
	//}
	return nil
}

func GetConfig() *Config {
	if config == nil {
		config = new(Config)
		_ = LoadConfig()
	}
	return config
}

// ========== 微服务配置结构 ==========

// MicroserviceConfig 微服务总配置
//
// 设计原理：
// 1. 每个功能独立配置（input_prediction、polish、digest）
// 2. 支持全局开关（Enabled）
// 3. 支持调用日志开关（LogCalls）
type MicroserviceConfig struct {
	Enabled  bool `toml:"enabled"`   // 是否启用微服务（总开关）
	LogCalls bool `toml:"log_calls"` // 是否记录调用日志（开发环境true，生产环境可选）

	// 各功能独立配置
	InputPrediction ServiceModelConfig `toml:"input_prediction"` // 智能输入预测
	Polish          ServiceModelConfig `toml:"polish"`           // 文本润色
	Digest          ServiceModelConfig `toml:"digest"`           // 消息摘要
}

// ServiceModelConfig 单个微服务的模型配置
//
// 设计原理：
// 1. 每个功能可以使用不同的模型
// 2. 支持不同的 Provider（ark、openai、deepseek）
// 3. 支持功能特定的参数（如 ContextMessages）
//
// 配置示例：
//
//	[aiConfig.microservice.input_prediction]
//	provider = "ark"
//	model = "doubao-lite-8k"
//	api_key = "${DOUBAO_API_KEY}"
//	temperature = 0.7
//	max_tokens = 100
type ServiceModelConfig struct {
	// ========== LLM 基础配置 ==========
	Provider       string  `toml:"provider"`        // 提供商：ark/openai/deepseek
	Model          string  `toml:"model"`           // 模型名称
	APIKey         string  `toml:"api_key"`         // API Key（支持环境变量）
	BaseURL        string  `toml:"base_url"`        // API Base URL
	Temperature    float64 `toml:"temperature"`     // 温度（0-1）
	MaxTokens      int     `toml:"max_tokens"`      // 最大 Token 数
	TimeoutSeconds int     `toml:"timeout_seconds"` // 超时时间（秒）

	// ========== 功能特定配置 ==========
	// 以下参数为可选，不同功能使用不同参数

	// 智能输入预测专用
	ContextMessages int `toml:"context_messages"`  // 上下文消息数（默认10）
	DebounceMsint   int `toml:"debounce_ms"`       // 防抖延迟（毫秒，默认500）
	MaxInputChars   int `toml:"max_input_chars"`   // 最大输入字符（默认500）
	CacheTTLSeconds int `toml:"cache_ttl_seconds"` // 缓存TTL（秒）

	// 润色专用
	MaxOptions int `toml:"max_options"` // 最多返回选项数（默认3）

	// 摘要专用
	MaxMessages int `toml:"max_messages"` // 最多处理消息数（默认200）
}
