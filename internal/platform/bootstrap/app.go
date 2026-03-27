package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/config"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/httpserver"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/observability"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/persistence"
)

type RuntimeOptions struct {
	Handler http.Handler
	Close   func(context.Context) error
}

type App struct {
	Config config.Config
	Logger *slog.Logger
	Server *httpserver.Server
	DB     *sql.DB
	close  func(context.Context) error
}

func New(_ context.Context, serviceName string) (*App, error) {
	cfg := config.Load(serviceName)
	logger := observability.NewBootstrapLogger(serviceName)
	db, err := persistence.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	return &App{
		Config: cfg,
		Logger: logger,
		DB:     db,
	}, nil
}

func (a *App) RegisterRuntime(options RuntimeOptions) error {
	if options.Handler == nil {
		return errors.New("runtime handler is nil")
	}

	a.Server = httpserver.New(a.Config.HTTPAddress, options.Handler)
	a.close = options.Close
	return nil
}

func (a *App) Run(ctx context.Context) error {
	if a.Server == nil {
		return errors.New("runtime is not registered")
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- a.Server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run http server: %w", err)
		}
		return nil
	case <-ctx.Done():
		return a.Close(context.Background())
	}
}

func (a *App) Close(ctx context.Context) error {
	var errs []error

	if a.Server != nil {
		if err := a.Server.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if a.close != nil {
		if err := a.close(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if a.Server == nil {
		if a.DB != nil {
			errs = append(errs, a.DB.Close())
		}
		return errors.Join(errs...)
	}
	if a.DB != nil {
		errs = append(errs, a.DB.Close())
	}
	return errors.Join(errs...)
}
