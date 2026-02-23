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
