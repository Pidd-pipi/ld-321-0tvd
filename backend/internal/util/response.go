package util

import (
	"errors"
	"net/http"

	"github.com/agridispatch/agridispatch/internal/constants"
	apperrors "github.com/agridispatch/agridispatch/internal/errors"
	"github.com/agridispatch/agridispatch/internal/repository"
	"github.com/gin-gonic/gin"
)

// Response 统一响应体。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK 成功响应。
func OK(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: constants.CodeOK, Message: "ok", Data: data})
}

// Created 创建成功响应。
func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{Code: constants.CodeOK, Message: "ok", Data: data})
}

// Fail 业务失败响应。
func Fail(c *gin.Context, status, code int, message string) {
	c.JSON(status, Response{Code: code, Message: message})
}

// FailError 根据错误类型转换为统一响应。
func FailError(c *gin.Context, err error) {
	var bizErr *apperrors.BusinessError
	switch {
	case errors.As(err, &bizErr):
		Fail(c, businessHTTPStatus(bizErr.Code), bizErr.Code, bizErr.Message)
	case errors.Is(err, repository.ErrNotFound):
		Fail(c, http.StatusNotFound, constants.CodeNotFound, err.Error())
	default:
		Fail(c, http.StatusBadRequest, constants.CodeBadRequest, err.Error())
	}
}

// businessHTTPStatus 业务错误码到 HTTP 状态码的映射。
func businessHTTPStatus(code int) int {
	switch code {
	case apperrors.CodeTaskNotFound, apperrors.CodeMachineMissing:
		return http.StatusNotFound
	case apperrors.CodeNoIdleMachine, apperrors.CodeTaskState, apperrors.CodeMachineBusy:
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
