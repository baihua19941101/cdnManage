package app

import (
	"net/http"

	"github.com/baihua19941101/cdnManage/internal/infra/configloader"
	"github.com/baihua19941101/cdnManage/internal/transport"
)

type Application struct{ server *http.Server }

func New() (*Application, error) {
	cfg, err := configloader.Load()
	if err != nil {
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
