package types

import "fmt"

// MCPError MCP 错误类型
type MCPError struct {
	Code    int
	Message string
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("MCP Error [%d]: %s", e.Code, e.Message)
}

// 常见错误代码
const (
	ErrCodeInvalidParams  = -32602
	ErrCodeMethodNotFound = -32601
	ErrCodeInternalError  = -32603
	ErrCodeToolNotFound   = -32001
	ErrCodeToolExecFailed = -32002
	ErrCodeUnauthorized   = -32003
)

// NewMCPError 创建 MCP 错误
func NewMCPError(code int, message string) *MCPError {
	return &MCPError{
		Code:    code,
		Message: message,
	}
}

// 预定义错误
var (
	ErrToolNotFound  = NewMCPError(ErrCodeToolNotFound, "tool not found")
	ErrInvalidParams = NewMCPError(ErrCodeInvalidParams, "invalid parameters")
	ErrUnauthorized  = NewMCPError(ErrCodeUnauthorized, "unauthorized")
	ErrInternalError = NewMCPError(ErrCodeInternalError, "internal error")
)
