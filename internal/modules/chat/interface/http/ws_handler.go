package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	chatRequest "OmniLink/internal/modules/chat/application/dto/request"
	chatService "OmniLink/internal/modules/chat/application/service"
	aiEvent "OmniLink/internal/modules/ai/interface/event"
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/util/myjwt"
	"OmniLink/pkg/ws"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WsHandler struct {
	hub          *ws.Hub
	svc          chatService.RealtimeService
	userRepo     userRepository.UserInfoRepository
	aiEventH     *aiEvent.AIEventHandler
}

func NewWsHandler(hub *ws.Hub, svc chatService.RealtimeService, userRepo userRepository.UserInfoRepository, aiEventH *aiEvent.AIEventHandler) *WsHandler {
	return &WsHandler{
		hub:      hub,
		svc:      svc,
		userRepo: userRepo,
		aiEventH: aiEventH,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func (h *WsHandler) Connect(c *gin.Context) {
	clientID := c.Query("client_id")
	token := c.Query("token")

	if clientID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if token != "" {
		claims, err := myjwt.ParseToken(token)
		if err != nil || claims == nil || claims.Uuid != clientID {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
	}
	// 	- 事实 ： GE.GET("/wss", ...) 这一行代码是写在 authed := GE.Group("/") 外面 的（或者说它没有使用 Use(jwtMiddleware.Auth()) ）。
	// - 原因 ：WebSocket 的握手请求有时候没法像普通 API 那样把 Token 放在 Header 里（特别是浏览器原生 WebSocket API 不支持自定义 Header）。
	// - 解决 ：所以我们通常把 Token 放在 URL 参数里传过来（ ?token=xxx ）。
	// - 结论 ：因为没走中间件，所以 Gin 框架不会自动帮我们校验 Token。因此，我们在 ws_handler.go 的第 49-55 行 手动写了一段校验 Token 的代码 。这就解释了为什么那里要去拿 Token。

	briefs, err := h.userRepo.GetUserBriefByUUIDs([]string{clientID})
	if err != nil || len(briefs) == 0 || briefs[0].Status != 0 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Error(err.Error())
		return
	}

	client := ws.NewClient(clientID, conn)
	h.hub.Register(client)
	if h.aiEventH != nil {
		go h.aiEventH.OnUserLogin(context.Background(), clientID)
	}
	// 上线：更新 LastOnlineAt
	go func() {
		if err := h.userRepo.UpdateLastOnlineAt(c.Request.Context(), clientID, time.Now()); err != nil {
			zlog.Error("Failed to update last online time: " + err.Error())
		}
	}()

	defer func() {
		h.hub.Unregister(client)
		// 离线：更新 LastOfflineAt
		go func() {
			// 这里不能用 c.Request.Context() 因为请求可能已经结束，用 Background
			if err := h.userRepo.UpdateLastOfflineAt(context.Background(), clientID, time.Now()); err != nil {
				zlog.Error("Failed to update last offline time: " + err.Error())
			}
		}()
	}()

	conn.SetReadLimit(1 << 20)

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	go client.WritePump()

	for {
		var req chatRequest.SendMessageRequest
		if err := conn.ReadJSON(&req); err != nil {
			// 84行：最关键的一步！这里会阻塞（停住），等待前端发消息过来。
			// 一旦前端发了数据，conn.ReadJSON 就会读出来，解析到 req 变量里。
			// 如果出错（比如前端断网了），就 return 退出循环，连接结束。
			return
		}

		if strings.HasPrefix(req.ReceiveId, "G") {
			memberIDs, item, err := h.svc.SendGroupMessage(clientID, req)
			if err != nil {
				_ = h.hub.SendJSON(clientID, map[string]interface{}{
					"type":    "error",
					"message": err.Error(),
				})
				continue
			}
			for _, mid := range memberIDs {
				_ = h.hub.SendJSON(mid, item)
			}
			continue
		}

		senderItem, receiverItem, err := h.svc.SendPrivateMessage(clientID, req)
		if err != nil {
			_ = h.hub.SendJSON(clientID, map[string]interface{}{
				"type":    "error",
				"message": err.Error(),
			})
			continue
		}

		_ = h.hub.SendJSON(clientID, senderItem)
		_ = h.hub.SendJSON(req.ReceiveId, receiverItem)
	}
}
