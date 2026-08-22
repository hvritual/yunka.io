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
	routes := app.rhTree.Paths()
	moduleDiagnostics := make([]ModuleDiagnostic, 0, len(app.moduleSnapshot())+len(app.composedModuleSnapshot()))
	for _, module := range app.moduleSnapshot() {
		if module == nil {
			continue
		}
		moduleDiagnostics = append(moduleDiagnostics, diagnosticForModule(module, "legacy"))
	}
	for _, module := range app.composedModuleSnapshot() {
		if module == nil {
			continue
		}
		moduleDiagnostics = append(moduleDiagnostics, diagnosticForComposedModule(module))
	}
	rpcClientConfigured, rpcServerCount := app.rpcInventory()
	return DiagnosticsReport{
		SchemaVersion: DiagnosticsSchemaVersion,
		State:         app.State().String(),
		Health:        app.Health(ctx),
		Modules:       moduleDiagnostics,
		Routes:        routes,
		Runtime: RuntimeDiagnostic{
			RouteCount: len(routes), RPCClientConfigured: rpcClientConfigured,
			RPCServerCount: rpcServerCount, EventBusConfigured: app.eventBus != nil,
		},
	}
}

func diagnosticForModule(module Module, composition string) ModuleDiagnostic {
	_, startable := module.(Startable)
	_, shutdownable := module.(Shutdowner)
	_, healthChecked := module.(HealthChecker)
	return ModuleDiagnostic{Name: module.Name(), Composition: composition, Startable: startable, Shutdownable: shutdownable, HealthChecked: healthChecked}
}

func diagnosticForComposedModule(module modulecatalog.Instance) ModuleDiagnostic {
	_, startable := module.(Startable)
	_, shutdownable := module.(Shutdowner)
	_, healthChecked := module.(HealthChecker)
	return ModuleDiagnostic{Name: module.Name(), Composition: "typed", Startable: startable, Shutdownable: shutdownable, HealthChecked: healthChecked}
}
