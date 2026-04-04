package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	httpresp "github.com/baihua19941101/cdnManage/internal/http"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(ctx *gin.Context, _ interface{}) {
		httpresp.ErrorWithDetails(ctx, http.StatusInternalServerError, "internal_error", "internal server error", nil)
	})
}
