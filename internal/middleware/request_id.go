package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
)

func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		requestID := ctx.GetHeader(httpresp.RequestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}

		ctx.Writer.Header().Set(httpresp.RequestIDHeader, requestID)
		ctx.Set(httpresp.RequestIDKey, requestID)
		ctx.Next()
	}
}

func generateRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}
