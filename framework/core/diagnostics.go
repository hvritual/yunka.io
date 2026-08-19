package core

import "context"

const DiagnosticsSchemaVersion = 1

// DiagnosticsReport is the transport-neutral, read-only inventory owned by the
// application core. Higher-level diagnostics packages may add contract,
// resilience, selector, or other domain-specific snapshots without making core
// depend on those packages.
type DiagnosticsReport struct {
	SchemaVersion int                `json:"schemaVersion"`
	State         string             `json:"state"`
	Health        HealthReport       `json:"health"`
	Modules       []ModuleDiagnostic `json:"modules,omitempty"`
	Routes        []string           `json:"routes,omitempty"`
	Runtime       RuntimeDiagnostic  `json:"runtime"`
}

type ModuleDiagnostic struct {
	Name          string `json:"name"`
	Startable     bool   `json:"startable,omitempty"`
	Shutdownable  bool   `json:"shutdownable,omitempty"`
	HealthChecked bool   `json:"healthChecked,omitempty"`
}

type RuntimeDiagnostic struct {
	RouteCount          int  `json:"routeCount"`
	RPCClientConfigured bool `json:"rpcClientConfigured"`
	RPCServerCount      int  `json:"rpcServerCount"`
	EventBusConfigured  bool `json:"eventBusConfigured"`
}

// Diagnostics returns a point-in-time, read-only view of application runtime
// inventory. Configuration values, credentials, request identity, and other
// secret-bearing state are intentionally excluded.
func (app *App) Diagnostics(ctx context.Context) DiagnosticsReport {
	if ctx == nil {
		ctx = context.Background()
	}
	if app == nil {
		return DiagnosticsReport{
			SchemaVersion: DiagnosticsSchemaVersion,
			State:         "unknown",
			Health:        HealthReport{State: "unknown"},
		}
	}

	routes := app.rhTree.Paths()
	modules := app.moduleSnapshot()
	moduleDiagnostics := make([]ModuleDiagnostic, 0, len(modules))
	for _, mod := range modules {
		if mod == nil {
			continue
		}
		_, startable := mod.(Startable)
		_, shutdownable := mod.(Shutdowner)
		_, healthChecked := mod.(HealthChecker)
		moduleDiagnostics = append(moduleDiagnostics, ModuleDiagnostic{
			Name:          mod.Name(),
			Startable:     startable,
			Shutdownable:  shutdownable,
			HealthChecked: healthChecked,
		})
	}

	return DiagnosticsReport{
		SchemaVersion: DiagnosticsSchemaVersion,
		State:         app.State().String(),
		Health:        app.Health(ctx),
		Modules:       moduleDiagnostics,
		Routes:        routes,
		Runtime: RuntimeDiagnostic{
			RouteCount:          len(routes),
			RPCClientConfigured: app.clt != nil,
			RPCServerCount:      len(app.srvs),
			EventBusConfigured:  app.eventBus != nil,
		},
	}
}
