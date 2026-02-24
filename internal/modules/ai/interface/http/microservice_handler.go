package http

import (
	"strings"

	"OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/back"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// MicroserviceHandler 微服务 HTTP Handler
//
// 职责：
// 1. 接收 HTTP 请求
// 2. 参数绑定（JSON → DTO）
// 3. JWT 认证
// 4. 调用 Service
// 5. 返回响应
//
// 设计原则：
// - Handler 只做接口适配，不涉及业务逻辑
// - 使用项目统一的响应格式（back.Result）
// - 统一的错误处理
type MicroserviceHandler struct {
	svc service.AIMicroserviceService
}

// NewMicroserviceHandler 创建 Handler
//
// 参数：
//   - svc: AIMicroserviceService 实例
//
// 返回值：
//   - *MicroserviceHandler: Handler 实例
func NewMicroserviceHandler(svc service.AIMicroserviceService) *MicroserviceHandler {
	return &MicroserviceHandler{svc: svc}
}

// ========== 4.1.1 Predict Handler ==========

// Predict 智能输入预测（非流式）
//
// HTTP API:
//
//	POST /ai/microservice/predict
//	Authorization: Bearer <JWT>
//	Content-Type: application/json
//
// Request Body:
//
//	{
//	  "input": "今天天气真不错，要不要一起",
//	  "context": {
//	    "messages": [...]
//	  }
//	}
//
// Response:
//
//	{
//	  "code": 200,
//	  "msg": "success",
//	  "data": {
//	    "prediction": "去公园散步？",
//	    "cache_hit": false,
//	    "tokens_used": 50,
//	    "latency_ms": 230
//	  }
//	}
func (h *MicroserviceHandler) Predict(c *gin.Context) {
	// ========== Step 1: 参数绑定 ==========
	//
	// 设计要点：
	// - 使用 Gin 的 ShouldBindJSON 自动绑定
	// - 绑定失败返回 400 错误
	var req request.PredictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zlog.Warn("predict bind json failed",
			zap.Error(err))
		back.Error(c, xerr.BadRequest, "参数格式错误")
		return
	}

	// ========== Step 2: JWT 认证 ==========
	//
	// 设计要点：
	// - 从 Gin Context 获取 uuid（由 JWT 中间件设置）
	// - 如果不存在，说明未登录
	uuid, exists := c.Get("uuid")
	if !exists {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}
	tenantUserID := uuid.(string)

	// ========== Step 3: 调用 Service ==========
	resp, err := h.svc.Predict(c.Request.Context(), req, tenantUserID)
	if err != nil {
		zlog.Error("predict service failed",
			zap.Error(err),
			zap.String("tenant_user_id", tenantUserID))

		// 根据错误类型返回不同的 HTTP 状态码
		if strings.Contains(err.Error(), "required") {
			back.Error(c, xerr.BadRequest, err.Error())
		} else {
			back.Error(c, xerr.InternalServerError, "预测失败")
		}
		return
	}

	// ========== Step 4: 返回成功响应 ==========
	//
	// 使用项目统一的响应格式
	back.Result(c, resp, nil)
}

// ========== 4.1.2 Polish Handler ==========

// Polish 文本润色
//
// HTTP API:
//
//	POST /ai/microservice/polish
//	Authorization: Bearer <JWT>
//	Content-Type: application/json
//
// Request Body:
//
//	{
//	  "text": "给我发一下那个文件",
//	  "context": {
//	    "messages": [...]
//	  }
//	}
//
// Response:
//
//	{
//	  "code": 200,
//	  "msg": "success",
//	  "data": {
//	    "polishes": [
//	      {"label": "更礼貌", "text": "麻烦您发一下那个文件，谢谢！"},
//	      {"label": "更简洁", "text": "请发文件"}
//	    ],
//	    "cache_hit": true,
//	    "tokens_used": 0,
//	    "latency_ms": 8
//	  }
//	}
func (h *MicroserviceHandler) Polish(c *gin.Context) {
	// Step 1: 参数绑定
	var req request.PolishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zlog.Warn("polish bind json failed",
			zap.Error(err))
		back.Error(c, xerr.BadRequest, "参数格式错误")
		return
	}

	// Step 2: JWT 认证
	uuid, exists := c.Get("uuid")
	if !exists {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}
	tenantUserID := uuid.(string)

	// Step 3: 调用 Service
	resp, err := h.svc.Polish(c.Request.Context(), req, tenantUserID)
	if err != nil {
		zlog.Error("polish service failed",
			zap.Error(err),
			zap.String("tenant_user_id", tenantUserID))

		if strings.Contains(err.Error(), "required") {
			back.Error(c, xerr.BadRequest, err.Error())
		} else {
			back.Error(c, xerr.InternalServerError, "润色失败")
		}
		return
	}

	// Step 4: 返回响应
	back.Result(c, resp, nil)
}

// ========== 4.1.3 Digest Handler ==========

// Digest 消息摘要
//
// HTTP API:
//
//	POST /ai/microservice/digest
//	Authorization: Bearer <JWT>
//	Content-Type: application/json
//
// Request Body:
//
//	{
//	  "group_id": "G12345",
//	  "message_count": 50,
//	  "time_range": {
//	    "start": "2026-02-09T10:00:00Z",
//	    "end": "2026-02-09T12:00:00Z"
//	  }
//	}
//
// Response:
//
//	{
//	  "code": 200,
//	  "msg": "success",
//	  "data": {
//	    "summary": "### 主要话题\n1. ...",
//	    "topics": ["项目进度", "团建"],
//	    "mentions": ["@张三", "@李四"],
//	    "latency_ms": 450,
//	    "cache_hit": false,
//	    "tokens_used": 200
//	  }
//	}
func (h *MicroserviceHandler) Digest(c *gin.Context) {
	// Step 1: 参数绑定
	var req request.DigestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		zlog.Warn("digest bind json failed",
			zap.Error(err))
		back.Error(c, xerr.BadRequest, "参数格式错误")
		return
	}

	// Step 2: JWT 认证
	uuid, exists := c.Get("uuid")
	if !exists {
		back.Error(c, xerr.Unauthorized, "未登录")
		return
	}
	tenantUserID := uuid.(string)

	// Step 3: 调用 Service
	resp, err := h.svc.Digest(c.Request.Context(), req, tenantUserID)
	if err != nil {
		zlog.Error("digest service failed",
			zap.Error(err),
			zap.String("tenant_user_id", tenantUserID),
			zap.String("group_id", req.GroupId))

		if strings.Contains(err.Error(), "required") {
			back.Error(c, xerr.BadRequest, err.Error())
		} else {
			back.Error(c, xerr.InternalServerError, "摘要生成失败")
		}
		return
	}

	// Step 4: 返回响应
	back.Result(c, resp, nil)
}
