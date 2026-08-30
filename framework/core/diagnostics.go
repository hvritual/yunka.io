package core

import (
	"context"

	"yunka.io/framework/core/modulecatalog"
)

const DiagnosticsSchemaVersion = 1

type DiagnosticsReport struct {
	SchemaVersion int                          `json:"schemaVersion"`
	State         string                       `json:"state"`
	Health        HealthReport                 `json:"health"`
	Modules       []ModuleDiagnostic           `json:"modules,omitempty"`
	Components    []RuntimeComponentDiagnostic `json:"components,omitempty"`
	Routes        []string                     `json:"routes,omitempty"`
	Runtime       RuntimeDiagnostic            `json:"runtime"`
}

type ModuleDiagnostic struct {
	Name          string `json:"name"`
	Composition   string `json:"composition,omitempty"`
	Startable     bool   `json:"startable,omitempty"`
	Shutdownable  bool   `json:"shutdownable,omitempty"`
	HealthChecked bool   `json:"healthChecked,omitempty"`
}

type RuntimeComponentDiagnostic struct {
	Name          string `json:"name"`
	Startable     bool   `json:"startable"`
	Shutdownable  bool   `json:"shutdownable"`
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
	var legacyRoutes []string
	if app.rhTree != nil {
		legacyRoutes = app.rhTree.Paths()
	}
	routes := mergedRuntimeRoutes(legacyRoutes, app.runtimeInventory)
	modules := app.composedModuleSnapshot()
	diagnostics := make([]ModuleDiagnostic, 0, len(modules))
	for _, module := range modules {
		if module != nil {
			diagnostics = append(diagnostics, diagnosticForComposedModule(module))
		}
	}
	components := app.runtimeComponentSnapshot()
	componentDiagnostics := make([]RuntimeComponentDiagnostic, 0, len(components))
	for _, component := range components {
		componentDiagnostics = append(componentDiagnostics, RuntimeComponentDiagnostic{
			Name: component.Name, Startable: component.StartFunc != nil, Shutdownable: component.ShutdownFunc != nil, HealthChecked: component.HealthFunc != nil,
		})
	}
	return DiagnosticsReport{
		SchemaVersion: DiagnosticsSchemaVersion,
		State:         app.State().String(),
		Health:        app.Health(ctx),
		Modules:       diagnostics,
		Components:    componentDiagnostics,
		Routes:        routes,
		Runtime: RuntimeDiagnostic{
			RouteCount:          len(routes),
			RPCClientConfigured: app.runtimeInventory.RPCClientConfigured,
			RPCServerCount:      app.runtimeInventory.RPCServerCount,
			EventBusConfigured:  app.eventBus != nil,
		},
	}
}

func diagnosticForComposedModule(module modulecatalog.Instance) ModuleDiagnostic {
	_, startable := module.(Startable)
	_, shutdownable := module.(Shutdowner)
	_, healthChecked := module.(HealthChecker)
	return ModuleDiagnostic{Name: module.Name(), Composition: "typed", Startable: startable, Shutdownable: shutdownable, HealthChecked: healthChecked}
}
