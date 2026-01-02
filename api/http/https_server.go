package http

import (
	"OmniLink/internal/config"
	"OmniLink/internal/initial"
	jwtMiddleware "OmniLink/internal/middleware/jwt"
	chatService "OmniLink/internal/modules/chat/application/service"
	chatPersistence "OmniLink/internal/modules/chat/infrastructure/persistence"
	chatHandler "OmniLink/internal/modules/chat/interface/http"
	contactService "OmniLink/internal/modules/contact/application/service"
	contactPersistence "OmniLink/internal/modules/contact/infrastructure/persistence"
	contactHandler "OmniLink/internal/modules/contact/interface/http"
	"OmniLink/internal/modules/user/application/service"
	"OmniLink/internal/modules/user/infrastructure/persistence"
	userHandler "OmniLink/internal/modules/user/interface/http"
	"OmniLink/pkg/ssl"
	"OmniLink/pkg/ws"

	cors "github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var GE *gin.Engine

func init() {
	GE = gin.Default()
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"*"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	GE.Use(cors.New(corsConfig))
	GE.Use(ssl.TlsHandler(config.GetConfig().MainConfig.Host, config.GetConfig().MainConfig.Port))

	wsHub := ws.NewHub()

	userRepo := persistence.NewUserInfoRepository(initial.GormDB)
	contactRepo := contactPersistence.NewUserContactRepository(initial.GormDB)
	applyRepo := contactPersistence.NewContactApplyRepository(initial.GormDB)
	uow := contactPersistence.NewContactUnitOfWork(initial.GormDB)
	sessionRepo := chatPersistence.NewSessionRepository(initial.GormDB)
	messageRepo := chatPersistence.NewMessageRepository(initial.GormDB)

	userSvc := service.NewUserInfoService(userRepo)
	contactSvc := contactService.NewContactService(contactRepo, applyRepo, userRepo, uow)
	sessionSvc := chatService.NewSessionService(sessionRepo, contactRepo, userRepo)
	messageSvc := chatService.NewMessageService(messageRepo, contactRepo)
	realtimeSvc := chatService.NewRealtimeService(messageRepo, sessionRepo, contactRepo, userRepo)

	userH := userHandler.NewUserInfoHandler(userSvc)
	contactH := contactHandler.NewContactHandler(contactSvc, wsHub)
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
	authed.POST("/contact/getUserList", contactH.GetUserList)
	authed.POST("/contact/getContactInfo", contactH.GetContactInfo)
	authed.POST("/contact/applyContact", contactH.ApplyContact)
	authed.POST("/contact/getNewContactList", contactH.GetNewContactList)
	authed.POST("/contact/passContactApply", contactH.PassContactApply)
	authed.POST("/contact/refuseContactApply", contactH.RefuseContactApply)
	authed.POST("/session/checkOpenSessionAllowed", sessionH.CheckOpenSessionAllowed)
	authed.POST("/session/openSession", sessionH.OpenSession)
	authed.POST("/session/getUserSessionList", sessionH.GetUserSessionList)
	authed.POST("/message/getMessageList", messageH.GetMessageList)
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
