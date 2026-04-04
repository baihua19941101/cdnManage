package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
)

func ErrorHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) == 0 {
			return
		}

		if ctx.Writer.Written() {
			return
		}

		err := ctx.Errors.Last()
		status := ctx.Writer.Status()
		if status < http.StatusBadRequest {
			status = http.StatusInternalServerError
		}

		if appErr, ok := err.Err.(*httpresp.AppError); ok {
			httpresp.ErrorWithDetails(ctx, appErr.StatusCode, appErr.Code, appErr.Message, appErr.Details)
			return
		}

		code := "internal_error"
		if err.Meta != nil {
			if metaCode, ok := err.Meta.(string); ok {
				code = metaCode
			}
		}

		httpresp.ErrorWithDetails(ctx, status, code, err.Error(), nil)
	}
}
