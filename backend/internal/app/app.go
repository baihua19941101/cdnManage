package app

import (
	"context"
	"net/http"

	authhandler "github.com/baihua19941101/cdnManage/internal/handler/auth"
	"github.com/baihua19941101/cdnManage/internal/infra/configloader"
	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/repository"
	serviceauth "github.com/baihua19941101/cdnManage/internal/service/auth"
	"github.com/baihua19941101/cdnManage/internal/service/bootstrap"
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

	store := repository.NewGormStore(db)
	txManager := repository.NewGormTxManager(db)
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

	return &Application{
		server: transport.NewServer(cfg, authHandler),
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
