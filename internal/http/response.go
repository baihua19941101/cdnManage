package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	RequestIDKey    = "request_id"
	RequestIDHeader = "X-Request-ID"
)

type APIResponse struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	RequestID string      `json:"requestId,omitempty"`
	Details   interface{} `json:"details,omitempty"`
	Data      interface{} `json:"data,omitempty"`
}

type AppError struct {
	StatusCode int
	Code       string
	Message    string
	Details    interface{}
}

func (e *AppError) Error() string {
	return e.Message
}

func NewAppError(status int, code, message string, details interface{}) *AppError {
	return &AppError{
		StatusCode: status,
		Code:       code,
		Message:    message,
		Details:    details,
	}
}

func Success(ctx *gin.Context, data interface{}) {
	JSON(ctx, http.StatusOK, "success", "ok", data)
}

func JSON(ctx *gin.Context, status int, code, message string, data interface{}) {
	response := APIResponse{
		Code:    code,
		Message: message,
		Data:    data,
	}

	if requestID := GetRequestID(ctx); requestID != "" {
		response.RequestID = requestID
	}

	ctx.JSON(status, response)
}

func Error(ctx *gin.Context, status int, code, message string) {
	JSON(ctx, status, code, message, nil)
}

func ErrorWithDetails(ctx *gin.Context, status int, code, message string, details interface{}) {
	response := APIResponse{
		Code:    code,
		Message: message,
		Details: details,
	}

	if requestID := GetRequestID(ctx); requestID != "" {
		response.RequestID = requestID
	}

	ctx.JSON(status, response)
}

func GetRequestID(ctx *gin.Context) string {
	if value, exists := ctx.Get(RequestIDKey); exists {
		if id, ok := value.(string); ok {
			return id
		}
	}

	if header := ctx.GetHeader(RequestIDHeader); header != "" {
		return header
	}

	return ""
}
