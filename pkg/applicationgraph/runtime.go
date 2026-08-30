package applicationgraph

import (
	"fmt"
	"strconv"
	"strings"
)

type RuntimeSnapshot struct {
	State      string             `json:"state"`
	Modules    []RuntimeModule    `json:"modules,omitempty"`
	Components []RuntimeComponent `json:"components,omitempty"`
	Routes     []string           `json:"routes,omitempty"`
	Runtime    RuntimeInventory   `json:"runtime"`
}

type RuntimeModule struct {
	Name          string `json:"name"`
	Startable     bool   `json:"startable,omitempty"`
	Shutdownable  bool   `json:"shutdownable,omitempty"`
	HealthChecked bool   `json:"healthChecked,omitempty"`
}

type RuntimeComponent struct {
	Name          string `json:"name"`
	Startable     bool   `json:"startable"`
	Shutdownable  bool   `json:"shutdownable"`
	HealthChecked bool   `json:"healthChecked,omitempty"`
}

type RuntimeInventory struct {
	RouteCount          int  `json:"routeCount"`
	RPCClientConfigured bool `json:"rpcClientConfigured"`
	RPCServerCount      int  `json:"rpcServerCount"`
	EventBusConfigured  bool `json:"eventBusConfigured"`
}

func AddRuntime(builder *Builder, snapshot RuntimeSnapshot, applicationName string) error {
	if builder == nil {
		return fmt.Errorf("applicationgraph: nil builder")
	}
	applicationName = strings.TrimSpace(applicationName)
	if applicationName == "" {
		applicationName = "yunka"
	}
	evidence := []Evidence{Observed("runtime.diagnostics", "live runtime inventory")}
	appID := ID(NodeApplication, applicationName)
	attrs := map[string]string{
		"state":               snapshot.State,
		"routeCount":          strconv.Itoa(snapshot.Runtime.RouteCount),
		"rpcClientConfigured": strconv.FormatBool(snapshot.Runtime.RPCClientConfigured),
		"rpcServerCount":      strconv.Itoa(snapshot.Runtime.RPCServerCount),
		"eventBusConfigured":  strconv.FormatBool(snapshot.Runtime.EventBusConfigured),
	}
	if err := builder.AddNode(Node{ID: appID, Kind: NodeApplication, Name: applicationName, Attributes: attrs, Evidence: evidence}); err != nil {
		return err
	}
	for _, module := range snapshot.Modules {
		name := strings.TrimSpace(module.Name)
		if name == "" {
			continue
		}
		moduleID := ID(NodeModule, name)
		if err := builder.AddNode(Node{ID: moduleID, Kind: NodeModule, Name: name, Attributes: map[string]string{
			"startable":     strconv.FormatBool(module.Startable),
			"shutdownable":  strconv.FormatBool(module.Shutdownable),
			"healthChecked": strconv.FormatBool(module.HealthChecked),
		}, Evidence: evidence}); err != nil {
			return err
		}
		if err := builder.AddEdge(Edge{From: appID, To: moduleID, Kind: EdgeContains, Evidence: evidence}); err != nil {
			return err
		}
	}
	for _, component := range snapshot.Components {
		name := strings.TrimSpace(component.Name)
		if name == "" {
			continue
		}
		componentID := ID(NodeRuntimeComponent, name)
		if err := builder.AddNode(Node{ID: componentID, Kind: NodeRuntimeComponent, Name: name, Attributes: map[string]string{
			"startable":     strconv.FormatBool(component.Startable),
			"shutdownable":  strconv.FormatBool(component.Shutdownable),
			"healthChecked": strconv.FormatBool(component.HealthChecked),
		}, Evidence: evidence}); err != nil {
			return err
		}
		if err := builder.AddEdge(Edge{From: appID, To: componentID, Kind: EdgeContains, Evidence: evidence}); err != nil {
			return err
		}
	}
	for _, route := range snapshot.Routes {
		route = strings.TrimSpace(route)
		if route == "" {
			continue
		}
		routeID := ID(NodeRuntimeRoute, route)
		if err := builder.AddNode(Node{ID: routeID, Kind: NodeRuntimeRoute, Name: route, Evidence: evidence}); err != nil {
			return err
		}
		if err := builder.AddEdge(Edge{From: appID, To: routeID, Kind: EdgeExposes, Evidence: evidence}); err != nil {
			return err
		}
	}
	return nil
}
