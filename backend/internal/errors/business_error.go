package errors

import "fmt"

// BusinessError 业务错误。
type BusinessError struct {
	Code    int
	Message string
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("code=%d message=%s", e.Code, e.Message)
}

// New 构造业务错误。
func New(code int, message string) *BusinessError {
	return &BusinessError{Code: code, Message: message}
}

// ConflictError 状态冲突错误（如任务状态不允许该操作、没有空闲农机）。
type ConflictError struct {
	Message string
}

func (e *ConflictError) Error() string {
	return e.Message
}
