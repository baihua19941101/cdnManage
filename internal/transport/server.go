package transport

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/baihua19941101/cdnManage/internal/config"
	"github.com/baihua19941101/cdnManage/internal/handler/health"
	"github.com/baihua19941101/cdnManage/internal/middleware"
)

func NewRouter() *gin.Engine {
	router := gin.New()

	router.Use(
		middleware.RequestID(),
		middleware.Recovery(),
		middleware.ErrorHandler(),
	)

	health.RegisterRoutes(router)

	return router
}

func NewServer(cfg *config.AppConfig) *http.Server {
	return &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.Port),
		Handler: NewRouter(),
	}
}
