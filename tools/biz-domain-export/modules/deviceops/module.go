package deviceops

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/hvritual/biz/internal/device/operations"
	"github.com/hvritual/biz/internal/device/transport/httpapi"
	"github.com/hvritual/biz/internal/iam/access"
	iambootstrap "github.com/hvritual/biz/internal/iam/bootstrap"
	"github.com/hvritual/biz/internal/migration"
)

const ModuleName = "deviceops"

type Module struct {
	dependencies Dependencies
	service      *operations.Service
	auth         *access.Authenticator

	mu       sync.RWMutex
	server   *http.Server
	listener net.Listener
	serveErr error
}

func NewModule(dependencies Dependencies) (*Module, error) {
	if dependencies.Logger == nil {
		return nil, errors.New("deviceops: logger is required")
	}
	if dependencies.PrimaryDatabase == nil {
		return nil, errors.New("deviceops: primary database is required")
	}
	if err := dependencies.Config.Validate(); err != nil {
		return nil, err
	}
	service, err := operations.NewService(dependencies.PrimaryDatabase)
	if err != nil {
		return nil, err
	}
	auth, err := access.NewAuthenticator(dependencies.PrimaryDatabase)
	if err != nil {
		return nil, err
	}
	return &Module{dependencies: dependencies, service: service, auth: auth}, nil
}

func (*Module) Name() string { return ModuleName }

func (module *Module) Start(ctx context.Context) error {
	if module == nil {
		return errors.New("deviceops: module is nil")
	}
	if module.dependencies.Config.AutoMigrate {
		if err := migration.Migrate(ctx, module.dependencies.PrimaryDatabase); err != nil {
			return fmt.Errorf("deviceops: migrate: %w", err)
		}
	}
	if module.dependencies.Config.Bootstrap.Token != "" {
		config := module.dependencies.Config.Bootstrap
		if err := iambootstrap.Ensure(ctx, module.dependencies.PrimaryDatabase, iambootstrap.Config{
			TenantID:   config.TenantID,
			TenantName: config.TenantName,
			UserID:     config.UserID,
			Email:      config.Email,
			Token:      config.Token,
		}); err != nil {
			return fmt.Errorf("deviceops: bootstrap: %w", err)
		}
	}
	listener, err := net.Listen("tcp", module.dependencies.Config.ListenAddress)
	if err != nil {
		return fmt.Errorf("deviceops: listen: %w", err)
	}
	server := &http.Server{
		Handler:           httpapi.NewHandler(module.service, module.auth, module.dependencies.PrimaryDatabase),
		ReadHeaderTimeout: 5 * time.Second,
	}
	module.mu.Lock()
	module.listener = listener
	module.server = server
	module.serveErr = nil
	module.mu.Unlock()
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			module.mu.Lock()
			module.serveErr = err
			module.mu.Unlock()
			module.dependencies.Logger.Errorf("deviceops HTTP server: %v", err)
		}
	}()
	return nil
}

func (module *Module) Health(ctx context.Context) error {
	if module == nil || module.dependencies.PrimaryDatabase == nil {
		return errors.New("deviceops: module database unavailable")
	}
	sqlDB, err := module.dependencies.PrimaryDatabase.DB()
	if err != nil {
		return err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return err
	}
	module.mu.RLock()
	defer module.mu.RUnlock()
	if module.server == nil || module.listener == nil {
		return errors.New("deviceops: HTTP server is not started")
	}
	return module.serveErr
}

func (module *Module) Shutdown(ctx context.Context) error {
	if module == nil {
		return nil
	}
	module.mu.RLock()
	server := module.server
	module.mu.RUnlock()
	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
}
