package websocket

import (
	"context"
	"encoding/json"
	"net/http"

	"OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// upgrader WebSocket 升级器
//
// 配置说明：
// - ReadBufferSize: 1024 字节（足够处理短文本）
// - WriteBufferSize: 1024 字节
// - CheckOrigin: 允许所有来源（生产环境需要严格校验）
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: 生产环境需要验证 Origin
		// 示例：return r.Header.Get("Origin") == "https://omnilink.com"
		return true
	},
}

// MicroserviceWSHandler 微服务 WebSocket Handler
//
// 职责：
// 1. 升级 HTTP 连接为 WebSocket
// 2. JWT 认证
// 3. 接收客户端消息
// 4. 调用 Service 流式接口
// 5. 发送流式响应
type MicroserviceWSHandler struct {
	svc service.AIMicroserviceService
}

// NewMicroserviceWSHandler 创建 WebSocket Handler
func NewMicroserviceWSHandler(svc service.AIMicroserviceService) *MicroserviceWSHandler {
	return &MicroserviceWSHandler{svc: svc}
}

// InputPrediction 智能输入 WebSocket 接口
//
// WebSocket URL:
//
//	ws://localhost:8080/ai/microservice/input/ws?token=<JWT>
//	或
//	ws://localhost:8080/ai/microservice/input/ws
//	Header: Authorization: Bearer <JWT>
//
// 客户端发送消息格式：
//
//	{
//	  "action": "predict",
//	  "data": {
//	    "input": "今天天气真不错",
//	    "context": {...}
//	  }
//	}
//
// 服务端响应格式：
//
//	// 流式 Token
//	{"event": "delta", "data": {"token": "去"}}
//	{"event": "delta", "data": {"token": "公园"}}
//
//	// 完成
//	{"event": "done", "data": {"prediction": "去公园散步？", "latency_ms": 230}}
//
//	// 错误
//	{"event": "error", "data": {"error": "..."}}
func (h *MicroserviceWSHandler) InputPrediction(c *gin.Context) {
	// ========== Step 1: 升级为 WebSocket ==========
	//
	// 设计要点：
	// - 升级失败会自动返回 HTTP 错误
	// - 升级成功后不能再使用 c.JSON 等方法
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		zlog.Error("websocket upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close() // 确保连接关闭

	// ========== Step 2: JWT 认证 ==========
	//
	// WebSocket 认证方式：
	// 1. URL Query 参数：?token=xxx
	// 2. HTTP Header：Authorization: Bearer xxx
	// 3. Gin Context（由中间件设置）
	//
	// 优先级：Context > Query > Header
	uuid, exists := c.Get("uuid")
	if !exists {
		// 尝试从 Query 获取
		token := c.Query("token")
		if token == "" {
			// 发送错误消息并关闭连接
			conn.WriteJSON(map[string]string{
				"event": "error",
				"error": "未登录：缺少 token",
			})
			return
		}

		// TODO: 验证 token 并提取 uuid
		// claims, err := jwt.ParseToken(token)
		// uuid = claims.UUID

		// 简化处理（示例）
		uuid = "U123"
	}
	tenantUserID := uuid.(string)

	zlog.Info("websocket connected",
		zap.String("tenant_user_id", tenantUserID),
		zap.String("remote_addr", c.Request.RemoteAddr))

	// ========== Step 3: 循环接收消息 ==========
	//
	// 设计要点：
	// - 使用无限循环持续监听
	// - ReadJSON 会阻塞，直到收到消息或连接关闭
	// - 连接关闭时 ReadJSON 返回错误，退出循环
	for {
		// Step 3.1: 读取客户端消息
		var wsMsg struct {
			Action string                 `json:"action"`
			Data   request.PredictRequest `json:"data"`
		}

		if err := conn.ReadJSON(&wsMsg); err != nil {
			// 连接关闭或读取错误
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				zlog.Warn("websocket read error",
					zap.Error(err),
					zap.String("tenant_user_id", tenantUserID))
			}
			break
		}

		// Step 3.2: 验证 action
		if wsMsg.Action != "predict" {
			conn.WriteJSON(map[string]string{
				"event": "error",
				"error": "unsupported action: " + wsMsg.Action,
			})
			continue
		}

		zlog.Info("websocket predict request",
			zap.String("tenant_user_id", tenantUserID),
			zap.Int("input_len", len(wsMsg.Data.Input)))

		// Step 3.3: 调用 Service 流式接口
		eventChan, err := h.svc.PredictStream(context.Background(), wsMsg.Data, tenantUserID)
		if err != nil {
			zlog.Error("predict stream failed",
				zap.Error(err),
				zap.String("tenant_user_id", tenantUserID))
			conn.WriteJSON(map[string]string{
				"event": "error",
				"error": err.Error(),
			})
			continue
		}

		// Step 3.4: 发送流式响应
		//
		// 设计要点：
		// - 循环读取 eventChan
		// - 每个事件立即发送给客户端
		// - 通道关闭时自动退出循环
		for event := range eventChan {
			// 序列化事件
			eventJSON, _ := json.Marshal(event)

			// 发送 JSON 消息
			if err := conn.WriteMessage(websocket.TextMessage, eventJSON); err != nil {
				zlog.Warn("websocket write failed",
					zap.Error(err),
					zap.String("tenant_user_id", tenantUserID))
				break
			}
		}

		zlog.Info("websocket predict complete",
			zap.String("tenant_user_id", tenantUserID))
	}

	zlog.Info("websocket disconnected",
		zap.String("tenant_user_id", tenantUserID))
}
