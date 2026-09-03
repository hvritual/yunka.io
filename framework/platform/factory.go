package platform

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/observability"
)

// DatabaseFactory creates one named database resource during Provider.Prepare.
type DatabaseFactory interface {
	Open(context.Context, string) (DatabaseResource, error)
}

type DatabaseFactoryFunc func(context.Context, string) (DatabaseResource, error)

func (factory DatabaseFactoryFunc) Open(ctx context.Context, name string) (DatabaseResource, error) {
	if factory == nil {
		return DatabaseResource{}, errors.New("platform: database factory is nil")
	}
	return factory(ctx, name)
}

// RPCFactory creates one named typed gRPC resource during Provider.Prepare.
type RPCFactory interface {
	Open(context.Context, string) (RPCResource, error)
}

type RPCFactoryFunc func(context.Context, string) (RPCResource, error)

func (factory RPCFactoryFunc) Open(ctx context.Context, name string) (RPCResource, error) {
	if factory == nil {
		return RPCResource{}, errors.New("platform: RPC factory is nil")
	}
	return factory(ctx, name)
}

// MySQLConfig is process-level infrastructure configuration. It is consumed by
// the platform provider and never exposed through module BuildContext.
type MySQLConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	GORM            *gorm.Config
}

func (config MySQLConfig) validate(name string) error {
	if strings.TrimSpace(config.DSN) == "" {
		return fmt.Errorf("platform: MySQL database %q DSN is required", name)
	}
	if config.MaxOpenConns < 0 || config.MaxIdleConns < 0 || config.ConnMaxLifetime < 0 || config.ConnMaxIdleTime < 0 {
		return fmt.Errorf("platform: MySQL database %q has negative pool configuration", name)
	}
	return nil
}

// MySQLFactory opens configured GORM databases and transfers sql.DB ownership
// to the platform provider.
type MySQLFactory struct {
	Configurations map[string]MySQLConfig
}

func (factory MySQLFactory) Open(_ context.Context, name string) (DatabaseResource, error) {
	name = strings.TrimSpace(name)
	config, ok := factory.Configurations[name]
	if !ok {
		return DatabaseResource{}, fmt.Errorf("platform: MySQL database %q is not configured", name)
	}
	if err := config.validate(name); err != nil {
		return DatabaseResource{}, err
	}
	gormConfig := config.GORM
	if gormConfig == nil {
		gormConfig = &gorm.Config{}
	}
	database, err := gorm.Open(mysql.Open(config.DSN), gormConfig)
	if err != nil {
		return DatabaseResource{}, fmt.Errorf("platform: open MySQL database %q: %w", name, err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		return DatabaseResource{}, fmt.Errorf("platform: access MySQL database %q pool: %w", name, err)
	}
	if config.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	}
	if config.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	}
	if config.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(config.ConnMaxIdleTime)
	}
	return ManagedDatabase(database), nil
}

// GRPCTLSConfig keeps transport trust explicit. Insecure transport is not a
// default; tests may deliberately supply insecure.NewCredentials through
// GRPCConfig.Credentials.
type GRPCTLSConfig struct {
	ServerName string
	CAFile     string
	CertFile   string
	KeyFile    string
	MinVersion uint16
}

func (config GRPCTLSConfig) Credentials() (credentials.TransportCredentials, error) {
	minimum := config.MinVersion
	if minimum == 0 {
		minimum = tls.VersionTLS12
	}
	tlsConfig := &tls.Config{MinVersion: minimum, ServerName: strings.TrimSpace(config.ServerName)}
	if path := strings.TrimSpace(config.CAFile); path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("platform: read RPC CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(contents) {
			return nil, errors.New("platform: RPC CA file contains no certificates")
		}
		tlsConfig.RootCAs = pool
	}
	certFile := strings.TrimSpace(config.CertFile)
	keyFile := strings.TrimSpace(config.KeyFile)
	if (certFile == "") != (keyFile == "") {
		return nil, errors.New("platform: RPC client certificate and key must be configured together")
	}
	if certFile != "" {
		certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("platform: load RPC client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{certificate}
	}
	return credentials.NewTLS(tlsConfig), nil
}

// GRPCConfig describes one named typed gRPC target.
type GRPCConfig struct {
	Target      string
	Credentials credentials.TransportCredentials
	DialOptions []grpc.DialOption
}

func (config GRPCConfig) validate(name string) error {
	if strings.TrimSpace(config.Target) == "" {
		return fmt.Errorf("platform: RPC target %q address is required", name)
	}
	if config.Credentials == nil {
		return fmt.Errorf("platform: RPC target %q transport credentials are required", name)
	}
	return nil
}

// GRPCFactory creates App-owned grpc.ClientConn instances with explicit
// transport credentials. Canonical W3C propagation is installed after
// application-provided dial options so callers cannot silently omit it while
// retaining the ability to compose their own observability/resilience clients.
type GRPCFactory struct {
	Configurations map[string]GRPCConfig
}

func (factory GRPCFactory) Open(_ context.Context, name string) (RPCResource, error) {
	name = strings.TrimSpace(name)
	config, ok := factory.Configurations[name]
	if !ok {
		return RPCResource{}, fmt.Errorf("platform: RPC target %q is not configured", name)
	}
	if err := config.validate(name); err != nil {
		return RPCResource{}, err
	}
	options := make([]grpc.DialOption, 0, len(config.DialOptions)+3)
	options = append(options, grpc.WithTransportCredentials(config.Credentials))
	options = append(options, config.DialOptions...)
	options = append(options,
		grpc.WithChainUnaryInterceptor(observability.UnaryClientPropagationInterceptor()),
		grpc.WithChainStreamInterceptor(observability.StreamClientPropagationInterceptor()),
	)
	connection, err := grpc.NewClient(strings.TrimSpace(config.Target), options...)
	if err != nil {
		return RPCResource{}, fmt.Errorf("platform: create RPC target %q: %w", name, err)
	}
	return ManagedClientConn(connection), nil
}
