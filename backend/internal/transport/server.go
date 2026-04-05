package transport

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/baihua19941101/cdnManage/internal/config"
	authhandler "github.com/baihua19941101/cdnManage/internal/handler/auth"
	"github.com/baihua19941101/cdnManage/internal/handler/health"
	projecthandler "github.com/baihua19941101/cdnManage/internal/handler/projects"
	userhandler "github.com/baihua19941101/cdnManage/internal/handler/users"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
)

type projectScopeMiddleware interface {
	Middleware() gin.HandlerFunc
}

func NewRouter(authHandler *authhandler.Handler, userHandler *userhandler.Handler, projectHandler *projecthandler.Handler, authenticator *serviceauth.Service, _ projectScopeMiddleware) *gin.Engine {
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
	if userHandler != nil && authenticator != nil {
		userhandler.RegisterRoutes(router, userHandler, authenticator)
	}
	if projectHandler != nil && authenticator != nil {
		projecthandler.RegisterRoutes(router, projectHandler, authenticator)
	}

	return router
}

func NewServer(cfg *config.AppConfig, authHandler *authhandler.Handler, userHandler *userhandler.Handler, projectHandler *projecthandler.Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) *http.Server {
	return &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.Port),
		Handler: NewRouter(authHandler, userHandler, projectHandler, authenticator, projectScope),
	}
}
