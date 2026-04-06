package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/baihua19941101/cdnManage/internal/config"
)

// CORS applies cross-origin policy for browser requests.
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowMethods := strings.Join(cfg.AllowMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ", ")

	allowedOrigins := make(map[string]struct{}, len(cfg.AllowOrigins))
	for _, origin := range cfg.AllowOrigins {
		normalized := strings.TrimSpace(origin)
		if normalized == "" {
			continue
		}
		allowedOrigins[normalized] = struct{}{}
	}

	allowAllOrigins := false
	if _, ok := allowedOrigins["*"]; ok {
		allowAllOrigins = true
	}

	return func(ctx *gin.Context) {
		if !cfg.Enabled {
			ctx.Next()
			return
		}

		origin := strings.TrimSpace(ctx.GetHeader("Origin"))
		if origin != "" && (allowAllOrigins || originAllowed(origin, allowedOrigins)) {
			if allowAllOrigins {
				ctx.Header("Access-Control-Allow-Origin", "*")
			} else {
				ctx.Header("Access-Control-Allow-Origin", origin)
				ctx.Header("Vary", "Origin")
			}

			ctx.Header("Access-Control-Allow-Methods", allowMethods)
			ctx.Header("Access-Control-Allow-Headers", allowHeaders)
			if exposeHeaders != "" {
				ctx.Header("Access-Control-Expose-Headers", exposeHeaders)
			}
			if cfg.AllowCredentials {
				ctx.Header("Access-Control-Allow-Credentials", "true")
			}
			if cfg.MaxAgeSeconds > 0 {
				ctx.Header("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAgeSeconds))
			}
		}

		if ctx.Request.Method == http.MethodOptions {
			ctx.AbortWithStatus(http.StatusNoContent)
			return
		}

		ctx.Next()
	}
}

func originAllowed(origin string, allowedOrigins map[string]struct{}) bool {
	_, ok := allowedOrigins[origin]
	return ok
}
