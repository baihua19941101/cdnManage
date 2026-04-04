package transport

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/baihua19941101/cdnManage/internal/config"
	authhandler "github.com/baihua19941101/cdnManage/internal/handler/auth"
	"github.com/baihua19941101/cdnManage/internal/handler/health"
	"github.com/baihua19941101/cdnManage/internal/middleware"
)

func NewRouter(authHandler *authhandler.Handler) *gin.Engine {
	router := gin.New()

	router.Use(
		middleware.RequestID(),
		middleware.Recovery(),
		middleware.ErrorHandler(),
	)

	health.RegisterRoutes(router)
	if authHandler != nil {
		authhandler.RegisterRoutes(router, authHandler)
	}

	return router
}

func NewServer(cfg *config.AppConfig, authHandler *authhandler.Handler) *http.Server {
	return &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.Port),
		Handler: NewRouter(authHandler),
	}
}
