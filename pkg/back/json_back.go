package back

import (
	"OmniLink/pkg/xerr"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 统一响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Result 统一返回入口
func Result(c *gin.Context, data interface{}, err error) {
	if err == nil {
		Success(c, data)
		return
	}

	// 判断是否为自定义错误
	if e, ok := err.(*xerr.CodeError); ok {
		Error(c, e.Code, e.Message)
		return
	}

	// 默认为系统错误
	Error(c, xerr.ErrServerError.Code, xerr.ErrServerError.Message)
}

// Success 成功返回
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    xerr.OK,
		Message: "Success",
		Data:    data,
	})
}

// Error 错误返回
func Error(c *gin.Context, code int, message string) {
	c.JSON(http.StatusOK, Response{
		Code:    code,
		Message: message,
	})
}

// Deprecated: JsonBack 是旧的返回方法，建议使用 Result
func JsonBack(c *gin.Context, message string, ret int, data interface{}) {
	switch ret {
	case 0:
		c.JSON(http.StatusOK, Response{
			Code:    xerr.OK,
			Message: message,
			Data:    data,
		})
	case -2:
		Error(c, xerr.BadRequest, message)
	case -1:
		Error(c, xerr.InternalServerError, message)
	}
}
