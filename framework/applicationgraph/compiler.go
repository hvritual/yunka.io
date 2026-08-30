package applicationgraph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"yunka.io/framework/core"
	"yunka.io/framework/core/resilience"
	graph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/contract"
	"yunka.io/pkg/selector"
)

type Source interface {
	Name() string
	Apply(context.Context, *graph.Builder) error
}

type sourceFunc struct {
	name  string
	apply func(context.Context, *graph.Builder) error
}

func (source sourceFunc) Name() string { return source.name }
func (source sourceFunc) Apply(ctx context.Context, builder *graph.Builder) error {
	if source.apply == nil {
		return errors.New("applicationgraph: source function is nil")
	}
	return source.apply(ctx, builder)
}

func Compile(ctx context.Context, sources ...Source) (graph.Graph, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	filtered := make([]Source, 0, len(sources))
	seen := make(map[string]struct{})
	for _, source := range sources {
		if source == nil {
			continue
		}
		name := strings.TrimSpace(source.Name())
		if name == "" {
			return graph.Graph{}, errors.New("applicationgraph: source name is required")
		}
		if _, exists := seen[name]; exists {
			return graph.Graph{}, fmt.Errorf("applicationgraph: duplicate source %q", name)
		}
		seen[name] = struct{}{}
		filtered = append(filtered, source)
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name() < filtered[j].Name() })
	builder := graph.NewBuilder()
	for _, source := range filtered {
		if err := safeApply(ctx, source, builder); err != nil {
			return graph.Graph{}, fmt.Errorf("applicationgraph source %s: %w", source.Name(), err)
		}
	}
	return builder.Build()
}

func safeApply(ctx context.Context, source Source, builder *graph.Builder) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic: %v", recovered)
		}
	}()
	return source.Apply(ctx, builder)
}

func Contract(manifest contract.Manifest) Source {
	return sourceFunc{name: "contract", apply: func(_ context.Context, builder *graph.Builder) error { return graph.AddContract(builder, manifest) }}
}

func Core(app *core.App, applicationName string) Source {
	return sourceFunc{name: "runtime", apply: func(ctx context.Context, builder *graph.Builder) error {
		if app == nil {
			return errors.New("application is nil")
		}
		current := app.Diagnostics(ctx)
		snapshot := graph.RuntimeSnapshot{State: current.State, Routes: append([]string(nil), current.Routes...)}
		snapshot.Runtime = graph.RuntimeInventory{
			RouteCount:          current.Runtime.RouteCount,
			RPCClientConfigured: current.Runtime.RPCClientConfigured,
			RPCServerCount:      current.Runtime.RPCServerCount,
			EventBusConfigured:  current.Runtime.EventBusConfigured,
		}
		for _, module := range current.Modules {
			snapshot.Modules = append(snapshot.Modules, graph.RuntimeModule{Name: module.Name, Startable: module.Startable, Shutdownable: module.Shutdownable, HealthChecked: module.HealthChecked})
		}
		for _, component := range current.Components {
			snapshot.Components = append(snapshot.Components, graph.RuntimeComponent{Name: component.Name, Startable: component.Startable, Shutdownable: component.Shutdownable, HealthChecked: component.HealthChecked})
		}
		return graph.AddRuntime(builder, snapshot, applicationName)
	}}
}

func Selector(name string, snapshotter selector.Snapshotter, services ...string) Source {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "selector"
	}
	services = stableStrings(services)
	return sourceFunc{name: name, apply: func(_ context.Context, builder *graph.Builder) error {
		if snapshotter == nil {
			return nil
		}
		evidence := []graph.Evidence{graph.Observed(name, "selector passive-health snapshot")}
		for _, service := range services {
			serviceID := graph.ID(graph.NodeSelectorService, service)
			if err := builder.AddNode(graph.Node{ID: serviceID, Kind: graph.NodeSelectorService, Name: service, Evidence: evidence}); err != nil {
				return err
			}
			for _, node := range snapshotter.Snapshot(service) {
				identity := node.NodeID
				if identity == "" {
					identity = node.Address
				}
				instanceName := service + "/" + node.Version + "/" + identity
				instanceID := graph.ID(graph.NodeInstance, instanceName)
				attrs := map[string]string{
					"service": service, "version": node.Version, "nodeId": node.NodeID, "address": node.Address,
					"ewmaMillis": strconv.FormatFloat(float64(node.EWMA)/float64(time.Millisecond), 'f', -1, 64),
					"score":      strconv.FormatFloat(node.Score, 'f', -1, 64), "inFlight": strconv.FormatInt(node.InFlight, 10),
					"ejected": strconv.FormatBool(node.Ejected), "failures": strconv.FormatUint(node.Failures, 10),
				}
				if err := builder.AddNode(graph.Node{ID: instanceID, Kind: graph.NodeInstance, Name: instanceName, Attributes: attrs, Evidence: evidence}); err != nil {
					return err
				}
				if err := builder.AddEdge(graph.Edge{From: serviceID, To: instanceID, Kind: graph.EdgeSelects, Evidence: evidence}); err != nil {
					return err
				}
			}
		}
		return nil
	}}
}

func Resilience(name string, policy *resilience.RPCPolicy, operations ...string) Source {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "resilience"
	}
	operations = stableStrings(operations)
	return sourceFunc{name: name, apply: func(_ context.Context, builder *graph.Builder) error {
		if policy == nil {
			return nil
		}
		evidence := []graph.Evidence{graph.Observed(name, "resilience policy snapshot")}
		for _, operation := range operations {
			snapshot, active := policy.PeekSnapshot(operation)
			operationID := graph.ID(graph.NodeOperation, operation)
			if !builder.HasNode(operationID) {
				if err := builder.AddNode(graph.Node{ID: operationID, Kind: graph.NodeOperation, Name: operation, Evidence: evidence}); err != nil {
					return err
				}
			}
			policyID := graph.ID(graph.NodePolicy, operation)
			attrs := map[string]string{
				"active": strconv.FormatBool(active), "circuitState": string(snapshot.Circuit.State),
				"rate": strconv.FormatFloat(snapshot.Rate.Rate, 'f', -1, 64), "burst": strconv.Itoa(snapshot.Rate.Burst),
				"loadLimit": strconv.Itoa(snapshot.Load.Limit), "loadInFlight": strconv.Itoa(snapshot.Load.InFlight),
			}
			if err := builder.AddNode(graph.Node{ID: policyID, Kind: graph.NodePolicy, Name: operation, Attributes: attrs, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(graph.Edge{From: operationID, To: policyID, Kind: graph.EdgeGovernedBy, Evidence: evidence}); err != nil {
				return err
			}
		}
		return nil
	}}
}

func EventTopics(name string, topics ...string) Source {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "events"
	}
	topics = stableStrings(topics)
	return sourceFunc{name: name, apply: func(_ context.Context, builder *graph.Builder) error {
		evidence := []graph.Evidence{graph.Declared(name, "event topic catalog")}
		for _, topic := range topics {
			if err := builder.AddNode(graph.Node{ID: graph.ID(graph.NodeEventTopic, topic), Kind: graph.NodeEventTopic, Name: topic, Evidence: evidence}); err != nil {
				return err
			}
		}
		return nil
	}}
}

func stableStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
