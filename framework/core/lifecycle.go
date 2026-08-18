package core

import (
	"context"
	"fmt"
	"time"
)

const DefaultShutdownTimeout = 10 * time.Second

type AppState uint32

const (
	AppStateNew AppState = iota
	AppStateInitializing
	AppStateStarting
	AppStateReady
	AppStateStopping
	AppStateStopped
	AppStateFailed
)

func (s AppState) String() string {
	switch s {
	case AppStateNew:
		return "new"
	case AppStateInitializing:
		return "initializing"
	case AppStateStarting:
		return "starting"
	case AppStateReady:
		return "ready"
	case AppStateStopping:
		return "stopping"
	case AppStateStopped:
		return "stopped"
	case AppStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Startable is implemented by process-scoped resources that require explicit startup.
type Startable interface {
	Start(context.Context) error
}

// Shutdowner is implemented by process-scoped resources that support graceful shutdown.
type Shutdowner interface {
	Shutdown(context.Context) error
}

// HealthChecker is implemented by process-scoped resources that can verify dependency health.
type HealthChecker interface {
	Health(context.Context) error
}

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type HealthCheck struct {
	Name   string       `json:"name"`
	Status HealthStatus `json:"status"`
	Error  string       `json:"error,omitempty"`
}

type HealthReport struct {
	State  string        `json:"state"`
	Live   bool          `json:"live"`
	Ready  bool          `json:"ready"`
	Checks []HealthCheck `json:"checks,omitempty"`
}

func safeLifecycleCall(name string, fn func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s panicked: %v", name, recovered)
		}
	}()
	return fn()
}
