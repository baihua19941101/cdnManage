package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	audithandler "github.com/baihua19941101/cdnManage/internal/handler/audits"
	authhandler "github.com/baihua19941101/cdnManage/internal/handler/auth"
	projecthandler "github.com/baihua19941101/cdnManage/internal/handler/projects"
	storagehandler "github.com/baihua19941101/cdnManage/internal/handler/storage"
	userhandler "github.com/baihua19941101/cdnManage/internal/handler/users"
	infraCache "github.com/baihua19941101/cdnManage/internal/infra/cache"
	"github.com/baihua19941101/cdnManage/internal/infra/configloader"
	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/infra/secure"
	"github.com/baihua19941101/cdnManage/internal/middleware"
	"github.com/baihua19941101/cdnManage/internal/provider"
	"github.com/baihua19941101/cdnManage/internal/repository"
	auditservice "github.com/baihua19941101/cdnManage/internal/service/audit"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	"github.com/baihua19941101/cdnManage/internal/service/bootstrap"
	serviceprojects "github.com/baihua19941101/cdnManage/internal/service/projects"
	serviceusers "github.com/baihua19941101/cdnManage/internal/service/users"
	"github.com/baihua19941101/cdnManage/internal/transport"
)

type Application struct{ server *http.Server }

func New() (*Application, error) {
	cfg, err := configloader.Load()
	if err != nil {
		return nil, err
	}

	db, err := infraDB.OpenMySQL(cfg.MySQL)
	if err != nil {
		return nil, err
	}

	if err := infraDB.AutoMigrate(db); err != nil {
		return nil, err
	}

	redisClient, err := infraCache.OpenRedis(cfg.Redis)
	if err != nil {
		return nil, err
	}

	store := repository.NewGormStore(db)
	txManager := repository.NewGormTxManager(db)
	auditRecorder := auditservice.NewRecorder(store.AuditLogs())
	bootstrapService := bootstrap.NewService(
		store.Users(),
		store.AuditLogs(),
		txManager,
		cfg.SuperAdmin,
	)
	if err := bootstrapService.Run(context.Background()); err != nil {
		return nil, err
	}

	authService := serviceauth.NewService(
		store.Users(),
		txManager,
		serviceauth.NewTokenManager(cfg.JWT),
	)
	authHandler := authhandler.NewHandler(authService)
	userService := serviceusers.NewService(
		store.Users(),
		store.Projects(),
		txManager,
	)
	userHandler := userhandler.NewHandler(userService, store.AuditLogs())
	projectService := serviceprojects.NewService(
		store.Projects(),
		txManager,
		secure.NewCredentialCipher(cfg.Encryption.Key),
	)
	if err := projectService.RegisterObjectStorageProvider(provider.NewTencentCOSProvider()); err != nil {
		return nil, fmt.Errorf("register tencent cos provider: %w", err)
	}
	if err := projectService.RegisterCDNProvider(provider.NewTencentCDNProvider()); err != nil {
		return nil, fmt.Errorf("register tencent cdn provider: %w", err)
	}
	projectService.ConfigureSyncTaskStatusCache(serviceprojects.NewRedisSyncTaskStatusCache(newRedisAdapter(redisClient)), 10*time.Minute)
	projectHandler := projecthandler.NewHandler(projectService, store.AuditLogs())
	storageHandler := storagehandler.NewHandler(projectService, store.AuditLogs(), cfg.Upload.MaxFileSizeMB)
	auditHandler := audithandler.NewHandler(store.AuditLogs())
	accessDeniedAuditor := middleware.NewAccessDeniedAuditor(auditRecorder)
	middleware.SetDefaultAccessDeniedAuditor(accessDeniedAuditor)
	projectScopeResolver := middleware.NewProjectScopeResolver(
		store.UserProjectRoles(),
		middleware.NewRedisUserProjectRoleCache(newRedisAdapter(redisClient)),
		5*time.Minute,
	).WithAuditor(accessDeniedAuditor)

	return &Application{
		server: transport.NewServer(cfg, authHandler, userHandler, projectHandler, storageHandler, auditHandler, authService, projectScopeResolver),
	}, nil
}

func (a *Application) Run() error {
	return a.server.ListenAndServe()
}

func Run() error {
	application, err := New()
	if err != nil {
		return err
	}

	if err := application.Run(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

type redisAdapter struct {
	client *redis.Client
}

func newRedisAdapter(client *redis.Client) *redisAdapter {
	return &redisAdapter{client: client}
}

func (a *redisAdapter) Get(ctx context.Context, key string) (string, error) {
	if value, err := a.client.Get(ctx, key).Result(); err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", err
	} else {
		return value, nil
	}
}

func (a *redisAdapter) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return a.client.Set(ctx, key, value, expiration).Err()
}
