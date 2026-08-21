package devruntime

import (
	"fmt"
	"strconv"
	"strings"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

func BuildRuntimeGraph(plan Plan, report RuntimeReport) (applicationgraph.Graph, error) {
	builder := applicationgraph.NewBuilder()
	for _, node := range plan.BaseGraph.Nodes {
		if err := builder.AddNode(node); err != nil {
			return applicationgraph.Graph{}, err
		}
	}
	for _, edge := range plan.BaseGraph.Edges {
		if err := builder.AddEdge(edge); err != nil {
			return applicationgraph.Graph{}, err
		}
	}

	application := strings.TrimSpace(report.Application)
	if application == "" && plan.Runtime != nil {
		application = strings.TrimSpace(plan.Runtime.Application)
	}
	if application == "" {
		application = DefaultRuntimeApplication
	}
	appID := applicationgraph.ID(applicationgraph.NodeApplication, application)
	declared := []applicationgraph.Evidence{applicationgraph.Declared("dev.manifest", "explicit local runtime topology")}
	if err := builder.AddNode(applicationgraph.Node{
		ID: appID, Kind: applicationgraph.NodeApplication, Name: application, Evidence: declared,
	}); err != nil {
		return applicationgraph.Graph{}, err
	}

	byName := make(map[string]ProcessRuntimeReport, len(report.Processes))
	for _, process := range report.Processes {
		byName[process.Name] = process
	}
	for _, process := range plan.Processes {
		processID := applicationgraph.ID(applicationgraph.NodeProcess, process.Name)
		attributes := map[string]string{}
		evidence := append([]applicationgraph.Evidence(nil), declared...)
		if state, ok := byName[process.Name]; ok {
			attributes["state"] = string(state.State)
			attributes["ready"] = strconv.FormatBool(state.Ready)
			if state.Diagnostics != nil {
				attributes["diagnosticsState"] = state.Diagnostics.State
				attributes["healthState"] = state.Diagnostics.HealthState
				attributes["live"] = strconv.FormatBool(state.Diagnostics.Live)
				attributes["diagnosticsReady"] = strconv.FormatBool(state.Diagnostics.Ready)
				attributes["routeCount"] = strconv.Itoa(state.Diagnostics.RouteCount)
				attributes["rpcClientConfigured"] = strconv.FormatBool(state.Diagnostics.RPCClientConfigured)
				attributes["rpcServerCount"] = strconv.Itoa(state.Diagnostics.RPCServerCount)
				attributes["eventBusConfigured"] = strconv.FormatBool(state.Diagnostics.EventBusConfigured)
			}
			evidence = append(evidence, applicationgraph.Observed("dev.runtime", "supervised direct-child runtime state"))
		}
		if process.GraphNode != "" {
			attributes["graphNode"] = process.GraphNode
		}
		if err := builder.AddNode(applicationgraph.Node{
			ID: processID, Kind: applicationgraph.NodeProcess, Name: process.Name,
			Attributes: attributes, Evidence: evidence,
		}); err != nil {
			return applicationgraph.Graph{}, err
		}
		if err := builder.AddEdge(applicationgraph.Edge{
			From: appID, To: processID, Kind: applicationgraph.EdgeContains, Evidence: declared,
		}); err != nil {
			return applicationgraph.Graph{}, err
		}
	}

	for _, process := range plan.Processes {
		processID := applicationgraph.ID(applicationgraph.NodeProcess, process.Name)
		for _, dependency := range process.DependsOn {
			dependencyID := applicationgraph.ID(applicationgraph.NodeProcess, dependency)
			if !builder.HasNode(dependencyID) {
				return applicationgraph.Graph{}, fmt.Errorf("devruntime: process graph dependency %q missing", dependency)
			}
			if err := builder.AddEdge(applicationgraph.Edge{
				From: processID, To: dependencyID, Kind: applicationgraph.EdgeDependsOn, Evidence: declared,
			}); err != nil {
				return applicationgraph.Graph{}, err
			}
		}
		if process.GraphNode != "" && builder.HasNode(process.GraphNode) {
			if err := builder.AddEdge(applicationgraph.Edge{
				From: processID, To: process.GraphNode, Kind: applicationgraph.EdgeRuns, Evidence: declared,
			}); err != nil {
				return applicationgraph.Graph{}, err
			}
		}
	}
	return builder.Build()
}
