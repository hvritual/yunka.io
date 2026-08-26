package deviceops

import (
	"errors"
	"strings"
)

type BootstrapConfig struct {
	TenantID   string
	TenantName string
	UserID     string
	Email      string
	Token      string
}

type Config struct {
	ListenAddress string
	AutoMigrate   bool
	Bootstrap     BootstrapConfig
}

func DefaultConfig() Config {
	return Config{ListenAddress: "127.0.0.1:8080"}
}

func (config Config) Validate() error {
	if strings.TrimSpace(config.ListenAddress) == "" {
		return errors.New("deviceops: listen address is required")
	}
	if config.Bootstrap.Token == "" {
		return nil
	}
	if strings.TrimSpace(config.Bootstrap.TenantID) == "" || strings.TrimSpace(config.Bootstrap.TenantName) == "" || strings.TrimSpace(config.Bootstrap.UserID) == "" || strings.TrimSpace(config.Bootstrap.Email) == "" {
		return errors.New("deviceops: complete bootstrap tenant/user identity is required when bootstrap token is configured")
	}
	return nil
}
