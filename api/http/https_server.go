package http

import (
	"context"
	"time"

	"OmniLink/internal/config"
	"OmniLink/internal/initial"
	jwtMiddleware "OmniLink/internal/middleware/jwt"
	aiService "OmniLink/internal/modules/ai/application/service"
	aiChunking "OmniLink/internal/modules/ai/infrastructure/chunking"
	aiEmbedding "OmniLink/internal/modules/ai/infrastructure/embedding"
	aiLLM "OmniLink/internal/modules/ai/infrastructure/llm"
	aiKafka "OmniLink/internal/modules/ai/infrastructure/mq/kafka"
	aiPersistence "OmniLink/internal/modules/ai/infrastructure/persistence"
	aiPipeline "OmniLink/internal/modules/ai/infrastructure/pipeline"
	aiQueue "OmniLink/internal/modules/ai/infrastructure/queue"
	aiReader "OmniLink/internal/modules/ai/infrastructure/reader"
	aiTransform "OmniLink/internal/modules/ai/infrastructure/transform"
	aiVectordb "OmniLink/internal/modules/ai/infrastructure/vectordb"
	aiHTTP "OmniLink/internal/modules/ai/interface/http"

	// MCP Imports
	einoMCP "github.com/cloudwego/eino-ext/components/tool/mcp"
	"github.com/cloudwego/eino/components/tool"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	omniMcpServer "OmniLink/internal/modules/ai/infrastructure/mcp/server"

	chatService "OmniLink/internal/modules/chat/application/service"
	chatPersistence "OmniLink/internal/modules/chat/infrastructure/persistence"
	chatHandler "OmniLink/internal/modules/chat/interface/http"
	contactService "OmniLink/internal/modules/contact/application/service"
	contactPersistence "OmniLink/internal/modules/contact/infrastructure/persistence"
	contactHandler "OmniLink/internal/modules/contact/interface/http"
	"OmniLink/internal/modules/user/application/service"
	"OmniLink/internal/modules/user/infrastructure/persistence"
	userHandler "OmniLink/internal/modules/user/interface/http"
	"OmniLink/pkg/ws"
	"OmniLink/pkg/zlog"
	"fmt"
	"strings"

	cors "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

var GE *gin.Engine

func init() {
	GE = gin.Default()
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	GE.Use(cors.New(corsConfig))
	// GE.Use(ssl.TlsHandler(config.GetConfig().MainConfig.Host, config.GetConfig().MainConfig.Port))
	wsHub := ws.NewHub()
	userRepo := persistence.NewUserInfoRepository(initial.GormDB)
	contactRepo := contactPersistence.NewUserContactRepository(initial.GormDB)
	applyRepo := contactPersistence.NewContactApplyRepository(initial.GormDB)
	groupRepo := contactPersistence.NewGroupInfoRepository(initial.GormDB)
	uow := contactPersistence.NewContactUnitOfWork(initial.GormDB)
	sessionRepo := chatPersistence.NewSessionRepository(initial.GormDB)
	messageRepo := chatPersistence.NewMessageRepository(initial.GormDB)
	conf := config.GetConfig()
	var aiAdminH *aiHTTP.AdminHandler
	var aiQueryH *aiHTTP.QueryHandler
	var aiAssistantH *aiHTTP.AssistantHandler
	var assistantPipeline *aiPipeline.AssistantPipeline
	var aiAsyncIngest aiService.AsyncIngestService
	if initial.MilvusClient != nil {
		metric := entity.COSINE
		switch strings.ToUpper(strings.TrimSpace(conf.MilvusConfig.MetricType)) {
		case "L2":
			metric = entity.L2
		case "IP":
			metric = entity.IP
		case "COSINE", "":
			metric = entity.COSINE
		}
		store, err := aiVectordb.NewMilvusStore(initial.MilvusClient, strings.TrimSpace(conf.MilvusConfig.CollectionName), "vector", conf.MilvusConfig.VectorDim, metric)
		if err != nil {
			zlog.Warn("ai milvus store init failed: " + err.Error())
		} else {
			vs, err := aiVectordb.NewMilvusVectorStore(store)
			if err != nil {
				zlog.Warn("ai milvus vector store init failed: " + err.Error())
			} else {
				ragRepo := aiPersistence.NewRAGRepository(initial.GormDB)
				eventRepo := aiPersistence.NewIngestEventRepository(initial.GormDB)
				jobRepo := aiPersistence.NewBackfillJobRepository(initial.GormDB)
				pub, err := aiKafka.NewPublisher(conf.KafkaConfig.Brokers)
				if err != nil {
					zlog.Warn("ai kafka publisher init failed: " + err.Error())
				} else {
					_ = aiKafka.EnsureTopic(aiKafka.TopicAdminConfig{
						Brokers:  conf.KafkaConfig.Brokers,
						ClientID: conf.KafkaConfig.ClientID,
					}, strings.TrimSpace(conf.KafkaConfig.IngestTopic), conf.KafkaConfig.Partitions, conf.KafkaConfig.Replication)
					relay := aiQueue.NewOutboxRelay(eventRepo, jobRepo, pub, strings.TrimSpace(conf.KafkaConfig.IngestTopic), 200, 500*time.Millisecond)
					go func() {
						if err := relay.Run(context.Background()); err != nil {
							zlog.Warn("ai outbox relay stopped: " + err.Error())
						}
					}()
				}
				chatReader := aiReader.NewChatSessionReader(sessionRepo, messageRepo)
				selfReader := aiReader.NewSelfProfileReader(userRepo)
				contactReader := aiReader.NewContactProfileReader(contactRepo, userRepo)
				groupReader := aiReader.NewGroupProfileReader(groupRepo, contactRepo, userRepo)
				chunker := aiChunking.NewRecursiveChunker(800, 120)
				merger := aiTransform.NewChatTurnMerger()
				embedder, embMeta, err := aiEmbedding.NewEmbedderFromConfig(context.Background(), conf)
				if err != nil {
					zlog.Warn("ai embedder init failed: " + err.Error() + "; fallback to mock")
					embedder = aiEmbedding.NewMockEmbedder(conf.MilvusConfig.VectorDim)
					embMeta = aiEmbedding.EmbedderMeta{Provider: "mock", Model: "mock"}
				}
				p, err := aiPipeline.NewIngestPipeline(ragRepo, vs, embedder, embMeta.Provider, embMeta.Model, merger, chunker, strings.TrimSpace(conf.MilvusConfig.CollectionName), conf.MilvusConfig.VectorDim)
				if err != nil {
					zlog.Warn("ai ingest pipeline init failed: " + err.Error())
				} else {
					ingestSvc := aiService.NewIngestService(chatReader, selfReader, contactReader, groupReader, jobRepo, eventRepo)
					aiAdminH = aiHTTP.NewAdminHandler(ingestSvc)
					aiAsyncIngest = aiService.NewAsyncIngestService(eventRepo)
					// RAG 召回 Pipeline & Service & Handler
					retrievePipeline, err := aiPipeline.NewRetrievePipeline(ragRepo, vs, embedder, conf.MilvusConfig.VectorDim)
					if err != nil {
						zlog.Warn("ai retrieve pipeline init failed: " + err.Error())
					} else {
						retrieveSvc := aiService.NewRetrieveService(retrievePipeline)
						aiQueryH = aiHTTP.NewQueryHandler(retrieveSvc)

						// AI Assistant Pipeline & Service & Handler
						chatModel, chatMeta, err := aiLLM.NewChatModelFromConfig(context.Background(), conf)
						if err != nil {
							zlog.Warn("ai chat model init failed: " + err.Error())
						} else {
							sessionRepo := aiPersistence.NewAssistantSessionRepository(initial.GormDB)
							messageRepo := aiPersistence.NewAssistantMessageRepository(initial.GormDB)
							agentRepo := aiPersistence.NewAgentRepository(initial.GormDB)

							assistantPipeline, err = aiPipeline.NewAssistantPipeline(
								sessionRepo,
								messageRepo,
								agentRepo,
								ragRepo,
								retrievePipeline,
								chatModel,
								aiPipeline.ChatModelMeta{
									Provider: chatMeta.Provider,
									Model:    chatMeta.Model,
								},
								nil,
							)
							if err != nil {
								zlog.Warn("ai assistant pipeline init failed: " + err.Error())
							} else {
								assistantSvc := aiService.NewAssistantService(sessionRepo, messageRepo, agentRepo, assistantPipeline)
								aiAssistantH = aiHTTP.NewAssistantHandler(assistantSvc)
							}
						}
					}
					consumer, err := aiKafka.NewConsumer(aiKafka.ConsumerConfig{
						Brokers:  conf.KafkaConfig.Brokers,
						GroupID:  strings.TrimSpace(conf.KafkaConfig.ConsumerGroupID),
						Topics:   []string{strings.TrimSpace(conf.KafkaConfig.IngestTopic)},
						ClientID: conf.KafkaConfig.ClientID,
					})
					if err != nil {
						zlog.Warn("ai kafka consumer init failed: " + err.Error())
					} else {
						worker := aiQueue.NewIngestConsumerWorker(consumer, eventRepo, jobRepo, chatReader, selfReader, contactReader, groupReader, p)
						go func() {
							if err := worker.Run(context.Background()); err != nil {
								zlog.Warn("ai ingest consumer stopped: " + err.Error())
							}
						}()
					}
				}
			}
		}
	} else {
		zlog.Warn("ai milvus client is nil; ai routes disabled")
	}
	userSvc := service.NewUserInfoService(userRepo)
	contactSvc := contactService.NewContactService(contactRepo, applyRepo, userRepo, uow, aiAsyncIngest)
	groupSvc := contactService.NewGroupService(contactRepo, groupRepo, userRepo, uow, aiAsyncIngest)
	sessionSvc := chatService.NewSessionService(sessionRepo, contactRepo, userRepo, groupRepo)
	messageSvc := chatService.NewMessageService(messageRepo, contactRepo)
	realtimeSvc := chatService.NewRealtimeService(messageRepo, sessionRepo, contactRepo, userRepo, groupRepo, aiAsyncIngest)

	// MCP Initialization
	if conf.MCPConfig.Enabled {
		zlog.Info("Initializing MCP components...")

		var allTools []tool.BaseTool

		// 1. 创建并注册内置 Server (使用 mcp-go)
		if conf.MCPConfig.BuiltinServer.Enabled {
			// 使用工厂函数创建 Server
			s := omniMcpServer.NewBuiltinMCPServer(
				omniMcpServer.BuiltinServerConfig{
					Name:               conf.MCPConfig.BuiltinServer.Name,
					Version:            conf.MCPConfig.BuiltinServer.Version,
					EnableContactTools: conf.MCPConfig.BuiltinServer.EnableContactTools,
					EnableGroupTools:   conf.MCPConfig.BuiltinServer.EnableGroupTools,
					EnableMessageTools: conf.MCPConfig.BuiltinServer.EnableMessageTools,
					EnableSessionTools: conf.MCPConfig.BuiltinServer.EnableSessionTools,
				},
				omniMcpServer.BuiltinServerDependencies{
					ContactSvc: contactSvc,
					GroupSvc:   groupSvc,
					MessageSvc: messageSvc,
					SessionSvc: sessionSvc,
				},
			)

			// 创建 In-Process Client 并连接
			inProcCli, err := client.NewInProcessClient(s)
			if err != nil {
				zlog.Error("Failed to create in-process client: " + err.Error())
			} else {
				initReq := mcp.InitializeRequest{}
				initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
				initReq.Params.ClientInfo = mcp.Implementation{
					Name:    "omnilink-internal-client",
					Version: "1.0.0",
				}

				if _, err := inProcCli.Initialize(context.Background(), initReq); err != nil {
					zlog.Error("Failed to initialize builtin MCP client: " + err.Error())
				} else {
					// 转换为 Eino Tools
					builtinTools, err := einoMCP.GetTools(context.Background(), &einoMCP.Config{
						Cli: inProcCli,
					})
					if err != nil {
						zlog.Error("Failed to get builtin tools: " + err.Error())
					} else {
						allTools = append(allTools, builtinTools...)
						zlog.Info(fmt.Sprintf("Registered %d builtin tools", len(builtinTools)))
					}
				}
			}
		}

		// 2. 注入 Pipeline
		if assistantPipeline != nil {
			assistantPipeline.SetTools(allTools)
		}

		zlog.Info("MCP initialization completed")
	}

	userH := userHandler.NewUserInfoHandler(userSvc)
	contactH := contactHandler.NewContactHandler(contactSvc, wsHub)
	groupH := contactHandler.NewGroupHandler(groupSvc)
	sessionH := chatHandler.NewSessionHandler(sessionSvc)
	messageH := chatHandler.NewMessageHandler(messageSvc)
	wsH := chatHandler.NewWsHandler(wsHub, realtimeSvc, userRepo)
	GE.POST("/login", userH.Login)
	GE.POST("/register", userH.Register)
	GE.GET("/wss", wsH.Connect)
	authed := GE.Group("/")
	authed.Use(jwtMiddleware.Auth())
	authed.GET("/auth/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"uuid":     c.GetString("uuid"),
			"username": c.GetString("username"),
		})
	})
	if aiAdminH != nil {
		authed.POST("/ai/internal/rag/backfill", aiAdminH.Backfill)
	}
	if aiQueryH != nil {
		authed.POST("/ai/rag/query", aiQueryH.Query)
	}
	if aiAssistantH != nil {
		authed.POST("/ai/assistant/chat", aiAssistantH.Chat)
		authed.POST("/ai/assistant/chat/stream", aiAssistantH.ChatStream)
		authed.GET("/ai/assistant/sessions", aiAssistantH.ListSessions)
		authed.GET("/ai/assistant/sessions/:session_id/messages", aiAssistantH.GetSessionMessages)
		authed.GET("/ai/assistant/agents", aiAssistantH.ListAgents)
	}
	authed.POST("/contact/getUserList", contactH.GetUserList)
	authed.POST("/contact/loadMyJoinedGroup", contactH.LoadMyJoinedGroup)
	authed.POST("/contact/getContactInfo", contactH.GetContactInfo)
	authed.POST("/contact/applyContact", contactH.ApplyContact)
	authed.POST("/contact/getNewContactList", contactH.GetNewContactList)
	authed.POST("/contact/passContactApply", contactH.PassContactApply)
	authed.POST("/contact/refuseContactApply", contactH.RefuseContactApply)
	authed.POST("/session/checkOpenSessionAllowed", sessionH.CheckOpenSessionAllowed)
	authed.POST("/session/openSession", sessionH.OpenSession)
	authed.POST("/session/getUserSessionList", sessionH.GetUserSessionList)
	authed.POST("/session/getGroupSessionList", sessionH.GetGroupSessionList)
	authed.POST("/message/getMessageList", messageH.GetMessageList)
	authed.POST("/message/getGroupMessageList", messageH.GetGroupMessageList)
	authed.POST("/group/createGroup", groupH.CreateGroup)
	authed.POST("/group/getGroupInfo", groupH.GetGroupInfo)
	authed.POST("/group/getGroupMemberList", groupH.GetGroupMemberList)
	authed.POST("/group/inviteGroupMembers", groupH.InviteGroupMembers)
	authed.POST("/group/leaveGroup", groupH.LeaveGroup)
	authed.POST("/group/dismissGroup", groupH.DismissGroup)
	//GE.POST("/user/updateUserInfo", v1.UpdateUserInfo)
	// GE.POST("/user/getUserInfoList", v1.GetUserInfoList)
	// GE.POST("/user/ableUsers", v1.AbleUsers)
	// GE.POST("/user/getUserInfo", v1.GetUserInfo)
	// GE.POST("/user/disableUsers", v1.DisableUsers)
	// GE.POST("/user/deleteUsers", v1.DeleteUsers)
	// GE.POST("/user/setAdmin", v1.SetAdmin)
	// GE.POST("/user/sendSmsCode", v1.SendSmsCode)
	// GE.POST("/user/smsLogin", v1.SmsLogin)
	// GE.POST("/user/wsLogout", v1.WsLogout)
	// GE.POST("/group/createGroup", v1.CreateGroup)
	// GE.POST("/group/loadMyGroup", v1.LoadMyGroup)
	// GE.POST("/group/checkGroupAddMode", v1.CheckGroupAddMode)
	// GE.POST("/group/enterGroupDirectly", v1.EnterGroupDirectly)
	// GE.POST("/group/leaveGroup", v1.LeaveGroup)
	// GE.POST("/group/dismissGroup", v1.DismissGroup)
	// GE.POST("/group/getGroupInfo", v1.GetGroupInfo)
	// GE.POST("/group/getGroupInfoList", v1.GetGroupInfoList)
	// GE.POST("/group/deleteGroups", v1.DeleteGroups)
	// GE.POST("/group/setGroupsStatus", v1.SetGroupsStatus)
	// GE.POST("/group/updateGroupInfo", v1.UpdateGroupInfo)
	// GE.POST("/group/getGroupMemberList", v1.GetGroupMemberList)
	// GE.POST("/group/removeGroupMembers", v1.RemoveGroupMembers)
	// GE.POST("/session/openSession", v1.OpenSession)
	// GE.POST("/session/getUserSessionList", v1.GetUserSessionList)
	// GE.POST("/session/getGroupSessionList", v1.GetGroupSessionList)
	// GE.POST("/session/deleteSession", v1.DeleteSession)
	// GE.POST("/session/checkOpenSessionAllowed", v1.CheckOpenSessionAllowed)
	// GE.POST("/contact/getUserList", v1.GetUserList)
	// GE.POST("/contact/loadMyJoinedGroup", v1.LoadMyJoinedGroup)
	// GE.POST("/contact/getContactInfo", v1.GetContactInfo)
	// GE.POST("/contact/deleteContact", v1.DeleteContact)
	// GE.POST("/contact/applyContact", v1.ApplyContact)
	// GE.POST("/contact/getNewContactList", v1.GetNewContactList)
	// GE.POST("/contact/passContactApply", v1.PassContactApply)
	// GE.POST("/contact/blackContact", v1.BlackContact)
	// GE.POST("/contact/cancelBlackContact", v1.CancelBlackContact)
	// GE.POST("/contact/getAddGroupList", v1.GetAddGroupList)
	// GE.POST("/contact/refuseContactApply", v1.RefuseContactApply)
	// GE.POST("/contact/blackApply", v1.BlackApply)
	// GE.POST("/message/getMessageList", v1.GetMessageList)
	// GE.POST("/message/getGroupMessageList", v1.GetGroupMessageList)
	// GE.POST("/message/uploadAvatar", v1.UploadAvatar)
	// GE.POST("/message/uploadFile", v1.UploadFile)
	// GE.POST("/chatroom/getCurContactListInChatRoom", v1.GetCurContactListInChatRoom)
	// GE.GET("/wss", v1.WsLogin)

}
