package applicationgraph

import (
	"context"
	"testing"

	"yunka.io/framework/core"
	graph "yunka.io/pkg/applicationgraph"
)

func TestCoreSourceIncludesSafeRuntimeComponentAndInventoryEvidence(t *testing.T) {
	app, err := core.NewApp(core.AppOptions{
		RuntimeComponents: []core.RuntimeComponent{{
			Name:         "grpc",
			StartFunc:    func(context.Context) error { return nil },
			HealthFunc:   func(context.Context) error { return nil },
			ShutdownFunc: func(context.Context) error { return nil },
		}},
		RuntimeInventory: core.RuntimeInventory{Routes: []string{"/v1/devices"}, RPCClientConfigured: true, RPCServerCount: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(context.Background(), Core(app, "device"))
	if err != nil {
		t.Fatal(err)
	}
	componentID := graph.ID(graph.NodeRuntimeComponent, "grpc")
	routeID := graph.ID(graph.NodeRuntimeRoute, "/v1/devices")
	var componentFound, routeFound, observed bool
	for _, node := range compiled.Nodes {
		switch node.ID {
		case componentID:
			componentFound = true
			if node.Kind != graph.NodeRuntimeComponent || node.Attributes["healthChecked"] != "true" {
				t.Fatalf("unexpected runtime component node: %#v", node)
			}
			for _, evidence := range node.Evidence {
				observed = observed || evidence.Type == graph.EvidenceObserved && evidence.Source == "runtime.diagnostics"
			}
		case routeID:
			routeFound = true
		}
	}
	if !componentFound || !routeFound || !observed {
		t.Fatalf("runtime graph missing observed closure: component=%t route=%t observed=%t graph=%#v", componentFound, routeFound, observed, compiled)
	}
	applicationID := graph.ID(graph.NodeApplication, "device")
	for _, node := range compiled.Nodes {
		if node.ID == applicationID {
			if node.Attributes["rpcClientConfigured"] != "true" || node.Attributes["rpcServerCount"] != "1" || node.Attributes["routeCount"] != "1" {
				t.Fatalf("unexpected runtime application inventory: %#v", node.Attributes)
			}
			return
		}
	}
	t.Fatal("runtime application node not found")
}
