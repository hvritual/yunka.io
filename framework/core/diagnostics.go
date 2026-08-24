package core

import (
	"context"

	"yunka.io/framework/core/modulecatalog"
)

const DiagnosticsSchemaVersion = 1

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
	Composition   string `json:"composition,omitempty"`
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

func (app *App) Diagnostics(ctx context.Context) DiagnosticsReport {
	if ctx == nil {
		ctx = context.Background()
	}
	if app == nil {
		return DiagnosticsReport{SchemaVersion: DiagnosticsSchemaVersion, State: "unknown", Health: HealthReport{State: "unknown"}}
	}
	var routes []string
	if app.rhTree != nil {
		routes = app.rhTree.Paths()
	}
	modules := app.composedModuleSnapshot()
	diagnostics := make([]ModuleDiagnostic, 0, len(modules))
	for _, module := range modules {
		if module != nil {
			diagnostics = append(diagnostics, diagnosticForComposedModule(module))
		}
	}
	return DiagnosticsReport{
		SchemaVersion: DiagnosticsSchemaVersion,
		State:         app.State().String(),
		Health:        app.Health(ctx),
		Modules:       diagnostics,
		Routes:        routes,
		Runtime: RuntimeDiagnostic{
			RouteCount:         len(routes),
			EventBusConfigured: app.eventBus != nil,
		},
	}
}

func diagnosticForComposedModule(module modulecatalog.Instance) ModuleDiagnostic {
	_, startable := module.(Startable)
	_, shutdownable := module.(Shutdowner)
	_, healthChecked := module.(HealthChecker)
	return ModuleDiagnostic{Name: module.Name(), Composition: "typed", Startable: startable, Shutdownable: shutdownable, HealthChecked: healthChecked}
}
