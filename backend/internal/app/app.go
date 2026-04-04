package app

import (
	"context"
	"net/http"

	"github.com/baihua19941101/cdnManage/internal/infra/configloader"
	infraDB "github.com/baihua19941101/cdnManage/internal/infra/db"
	"github.com/baihua19941101/cdnManage/internal/repository"
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

	return &Application{
		server: transport.NewServer(cfg),
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
