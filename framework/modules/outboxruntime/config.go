package outboxruntime

import (
	"fmt"
	"strings"
	"time"
)

type Config struct {
	Table                  string        `json:"table"`
	AutoMigrate            bool          `json:"autoMigrate"`
	SkipLocked             bool          `json:"skipLocked"`
	WorkerID               string        `json:"workerID"`
	PollInterval           time.Duration `json:"pollInterval"`
	BatchSize              int           `json:"batchSize"`
	Concurrency            int           `json:"concurrency"`
	LeaseDuration          time.Duration `json:"leaseDuration"`
	PublishTimeout         time.Duration `json:"publishTimeout"`
	MaxAttempts            int           `json:"maxAttempts"`
	RetryBase              time.Duration `json:"retryBase"`
	RetryMax               time.Duration `json:"retryMax"`
	RetryJitter            float64       `json:"retryJitter"`
	HealthFailureThreshold int           `json:"healthFailureThreshold"`
}

func DefaultConfig() Config {
	return Config{
		Table:                  "yunka_outbox",
		WorkerID:               "",
		PollInterval:           500 * time.Millisecond,
		BatchSize:              50,
		Concurrency:            4,
		LeaseDuration:          3 * time.Minute,
		PublishTimeout:         10 * time.Second,
		MaxAttempts:            8,
		RetryBase:              time.Second,
		RetryMax:               5 * time.Minute,
		RetryJitter:            0.2,
		HealthFailureThreshold: 5,
	}
}

func (config Config) normalized() Config {
	defaults := DefaultConfig()
	if strings.TrimSpace(config.Table) == "" {
		config.Table = defaults.Table
	}
	if strings.TrimSpace(config.WorkerID) == "" {
		config.WorkerID = defaults.WorkerID
	}
	if config.PollInterval <= 0 {
		config.PollInterval = defaults.PollInterval
	}
	if config.BatchSize <= 0 {
		config.BatchSize = defaults.BatchSize
	}
	if config.Concurrency <= 0 {
		config.Concurrency = defaults.Concurrency
	}
	if config.LeaseDuration <= 0 {
		config.LeaseDuration = defaults.LeaseDuration
	}
	if config.PublishTimeout <= 0 {
		config.PublishTimeout = defaults.PublishTimeout
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = defaults.MaxAttempts
	}
	if config.RetryBase <= 0 {
		config.RetryBase = defaults.RetryBase
	}
	if config.RetryMax <= 0 {
		config.RetryMax = defaults.RetryMax
	}
	if config.HealthFailureThreshold <= 0 {
		config.HealthFailureThreshold = defaults.HealthFailureThreshold
	}
	return config
}

func (config Config) Validate() error {
	config = config.normalized()
	if config.RetryJitter < 0 || config.RetryJitter > 1 {
		return fmt.Errorf("outboxruntime: retry jitter must be within [0,1]")
	}
	if config.RetryMax < config.RetryBase {
		return fmt.Errorf("outboxruntime: retry max %s is shorter than retry base %s", config.RetryMax, config.RetryBase)
	}
	waves := (config.BatchSize + config.Concurrency - 1) / config.Concurrency
	minimumLease := time.Duration(waves) * config.PublishTimeout
	if config.LeaseDuration < minimumLease {
		return fmt.Errorf("outboxruntime: lease duration %s is shorter than worst-case publish window %s", config.LeaseDuration, minimumLease)
	}
	return nil
}
