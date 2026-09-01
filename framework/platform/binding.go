package platform

import (
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"yunka.io/pkg/providerplan"
)

// BindManifest converts a validated provider declaration into the existing
// typed platform.Options factories. Host-owned capabilities remain explicit
// inputs; the manifest never constructs or hides them behind a service locator.
func BindManifest(manifest providerplan.Manifest, base Options) (Options, error) {
	manifest = providerplan.Canonicalize(manifest)
	if err := providerplan.Validate(manifest); err != nil {
		return Options{}, err
	}
	if manifest.Host.Config && base.Config == nil {
		return Options{}, fmt.Errorf("platform: provider manifest declares host config but no ConfigProvider was supplied")
	}
	if manifest.Host.Logger && base.Logger == nil {
		return Options{}, fmt.Errorf("platform: provider manifest declares host logger but no logger was supplied")
	}
	if manifest.Host.EventBus && base.EventBus == nil {
		return Options{}, fmt.Errorf("platform: provider manifest declares host eventBus but no event bus was supplied")
	}

	databases := NewRegistry[DatabaseFactory]()
	for name, factory := range base.Databases {
		if err := databases.Register(name, factory); err != nil {
			return Options{}, err
		}
	}
	for _, binding := range manifest.Databases {
		dsn, err := requiredEnvironment(binding.DSNEnv)
		if err != nil {
			return Options{}, fmt.Errorf("platform: database %q: %w", binding.Name, err)
		}
		maxLifetime, err := parseOptionalDuration(binding.ConnMaxLifetime)
		if err != nil {
			return Options{}, fmt.Errorf("platform: database %q connMaxLifetime: %w", binding.Name, err)
		}
		maxIdleTime, err := parseOptionalDuration(binding.ConnMaxIdleTime)
		if err != nil {
			return Options{}, fmt.Errorf("platform: database %q connMaxIdleTime: %w", binding.Name, err)
		}
		factory := MySQLFactory{Configurations: map[string]MySQLConfig{
			binding.Name: {
				DSN:             dsn,
				MaxOpenConns:    binding.MaxOpenConns,
				MaxIdleConns:    binding.MaxIdleConns,
				ConnMaxLifetime: maxLifetime,
				ConnMaxIdleTime: maxIdleTime,
			},
		}}
		if err := databases.Register(binding.Name, factory); err != nil {
			return Options{}, err
		}
	}
	databases.Seal()

	rpc := NewRegistry[RPCFactory]()
	for name, factory := range base.RPC {
		if err := rpc.Register(name, factory); err != nil {
			return Options{}, err
		}
	}
	for _, binding := range manifest.RPC {
		target, err := requiredEnvironment(binding.TargetEnv)
		if err != nil {
			return Options{}, fmt.Errorf("platform: RPC %q: %w", binding.Name, err)
		}
		transport, err := transportCredentials(binding)
		if err != nil {
			return Options{}, err
		}
		factory := GRPCFactory{Configurations: map[string]GRPCConfig{
			binding.Name: {Target: target, Credentials: transport},
		}}
		if err := rpc.Register(binding.Name, factory); err != nil {
			return Options{}, err
		}
	}
	rpc.Seal()

	base.Databases = databases.Snapshot()
	base.RPC = rpc.Snapshot()
	return base, nil
}

func transportCredentials(binding providerplan.RPCBinding) (credentials.TransportCredentials, error) {
	if binding.Insecure {
		return insecure.NewCredentials(), nil
	}
	if binding.TLS == nil {
		return nil, fmt.Errorf("platform: RPC %q TLS configuration is required", binding.Name)
	}
	serverName, err := optionalEnvironment(binding.TLS.ServerNameEnv)
	if err != nil {
		return nil, fmt.Errorf("platform: RPC %q server name: %w", binding.Name, err)
	}
	caFile, err := optionalEnvironment(binding.TLS.CAFileEnv)
	if err != nil {
		return nil, fmt.Errorf("platform: RPC %q CA file: %w", binding.Name, err)
	}
	certFile, err := optionalEnvironment(binding.TLS.CertFileEnv)
	if err != nil {
		return nil, fmt.Errorf("platform: RPC %q certificate file: %w", binding.Name, err)
	}
	keyFile, err := optionalEnvironment(binding.TLS.KeyFileEnv)
	if err != nil {
		return nil, fmt.Errorf("platform: RPC %q key file: %w", binding.Name, err)
	}
	return (GRPCTLSConfig{ServerName: serverName, CAFile: caFile, CertFile: certFile, KeyFile: keyFile}).Credentials()
}

func requiredEnvironment(name string) (string, error) {
	name = strings.TrimSpace(name)
	value, ok := os.LookupEnv(name)
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return "", fmt.Errorf("environment variable %s is required", name)
	}
	return value, nil
}

func optionalEnvironment(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", nil
	}
	return requiredEnvironment(name)
}

func parseOptionalDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return time.ParseDuration(value)
}
