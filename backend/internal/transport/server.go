package transport

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/baihua19941101/cdnManage/internal/config"
	audithandler "github.com/baihua19941101/cdnManage/internal/handler/audits"
	authhandler "github.com/baihua19941101/cdnManage/internal/handler/auth"
	"github.com/baihua19941101/cdnManage/internal/handler/health"
	overviewhandler "github.com/baihua19941101/cdnManage/internal/handler/overview"
	projecthandler "github.com/baihua19941101/cdnManage/internal/handler/projects"
	storagehandler "github.com/baihua19941101/cdnManage/internal/handler/storage"
	userhandler "github.com/baihua19941101/cdnManage/internal/handler/users"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
)

type projectScopeMiddleware interface {
	Middleware() gin.HandlerFunc
}

func NewRouter(cfg *config.AppConfig, authHandler *authhandler.Handler, userHandler *userhandler.Handler, projectHandler *projecthandler.Handler, storageHandler *storagehandler.Handler, auditHandler *audithandler.Handler, overviewHandler *overviewhandler.Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) *gin.Engine {
	router := gin.New()

	router.Use(
		middleware.CORS(cfg.CORS),
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
		projecthandler.RegisterRoutes(router, projectHandler, authenticator, projectScope)
	}
	if storageHandler != nil && authenticator != nil {
		storagehandler.RegisterRoutes(router, storageHandler, authenticator, projectScope)
	}
	if auditHandler != nil && authenticator != nil {
		audithandler.RegisterRoutes(router, auditHandler, authenticator, projectScope)
	}
	if overviewHandler != nil && authenticator != nil {
		overviewhandler.RegisterRoutes(router, overviewHandler, authenticator)
	}

	return router
}

func NewServer(cfg *config.AppConfig, authHandler *authhandler.Handler, userHandler *userhandler.Handler, projectHandler *projecthandler.Handler, storageHandler *storagehandler.Handler, auditHandler *audithandler.Handler, overviewHandler *overviewhandler.Handler, authenticator *serviceauth.Service, projectScope projectScopeMiddleware) *http.Server {
	return &http.Server{
		Addr:    ":" + strconv.Itoa(cfg.Server.Port),
		Handler: NewRouter(cfg, authHandler, userHandler, projectHandler, storageHandler, auditHandler, overviewHandler, authenticator, projectScope),
	}
}
