package initial

import (
	"OmniLink/internal/config"
	aiRag "OmniLink/internal/modules/ai/domain/rag"
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	contactEntity "OmniLink/internal/modules/contact/domain/entity"
	userEntity "OmniLink/internal/modules/user/domain/entity"

	"OmniLink/pkg/zlog"
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var GormDB *gorm.DB

func init() {
	conf := config.GetConfig()
	user := conf.MysqlConfig.User
	password := conf.MysqlConfig.Password
	host := conf.MysqlConfig.Host
	port := conf.MysqlConfig.Port
	appName := conf.AppName
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, password, host, port, appName)
	//dsn := fmt.Sprintf("%s@unix(/var/run/mysqld/mysqld.sock)/%s?charset=utf8mb4&parseTime=True&loc=Local", user, appName)
	var err error
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
	GormDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormLogger})
	if err != nil {
		zlog.Fatal(err.Error())
	}
	err = GormDB.AutoMigrate(
		&userEntity.UserInfo{},
		&contactEntity.UserContact{},
		&contactEntity.ContactApply{},
		&contactEntity.GroupInfo{},
		&chatEntity.Session{},
		&chatEntity.Message{},

		&aiRag.AIKnowledgeBase{},
		&aiRag.AIKnowledgeSource{},
		&aiRag.AIKnowledgeChunk{},
		&aiRag.AIVectorRecord{},
		&aiRag.AIIngestEvent{},
		&aiRag.AIChatSession{},
		&aiRag.AIChatMessage{},
		&aiRag.AIAgent{},
		&aiRag.AIToolRegistry{},
		&aiRag.AIAgentToolBinding{},
		&aiRag.AIUploadedFile{},
	)
	// 自动迁移，如果没有建表，会自动创建对应的表
	if err != nil {
		zlog.Fatal(err.Error())
	}
}
