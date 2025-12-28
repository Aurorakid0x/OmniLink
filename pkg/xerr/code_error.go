package xerr

import "fmt"

// CodeError 自定义错误结构
type CodeError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error 实现 error 接口
func (e *CodeError) Error() string {
	return fmt.Sprintf("Code: %d, Message: %s", e.Code, e.Message)
}

// New 创建新的 CodeError
func New(code int, msg string) *CodeError {
	return &CodeError{Code: code, Message: msg}
}

// 常用通用错误码
const (
	OK                  = 200
	BadRequest          = 400
	Unauthorized        = 401
	Forbidden           = 403
	NotFound            = 404
	InternalServerError = 500
)

// 常用预定义错误
var (
	ErrSuccess     = New(OK, "Success")
	ErrServerError = New(InternalServerError, "系统错误，请联系工作人员")
	ErrParam       = New(BadRequest, "参数错误")
)