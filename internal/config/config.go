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

type Config struct {
	MainConfig  `toml:"mainConfig"`
	MysqlConfig `toml:"mysqlConfig"`
	//RedisConfig     `toml:"redisConfig"`
	//AuthCodeConfig  `toml:"authCodeConfig"`
	LogConfig `toml:"logConfig"`
	//KafkaConfig     `toml:"kafkaConfig"`
	//StaticSrcConfig `toml:"staticSrcConfig"`
}

var config *Config

func LoadConfig() error {

	configPath := "configs/config.toml"
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
