package devruntime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
)

func TestC117AMultiApplicationProcessClosure(t *testing.T) {
	root := t.TempDir()
	appNodes := []string{
		"application:deviceops/device_management",
		"application:deviceops/device_transfer",
		"application:deviceops/site_management",
	}
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion}
	for _, id := range appNodes {
		graph.Nodes = append(graph.Nodes, applicationgraph.Node{ID: id, Kind: applicationgraph.NodeApplication, Name: strings.TrimPrefix(id, "application:")})
	}
	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime: &RuntimeConfig{
			Application: "biz",
			StatePath:   ".yunka/c117-runtime.json",
			GraphPath:   ".yunka/c117-runtime-graph.json",
			Closure:     true,
		},
		Processes: []Process{{
			Name:       "biz",
			Command:    []string{"biz"},
			GraphNodes: []string{appNodes[2], appNodes[0], appNodes[1]},
			Readiness: &Readiness{
				URL:                "http://127.0.0.1:18080/_yunka/diagnostics",
				DiagnosticsReady:   true,
				CaptureDiagnostics: true,
			},
		}},
	}

	plan, err := BuildPlan(manifest, root, nil, graph)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Processes) != 1 || plan.Processes[0].GraphNode != "" {
		t.Fatalf("unexpected plan: %#v", plan.Processes)
	}
	if got := strings.Join(plan.Processes[0].GraphNodes, ","); got != strings.Join(appNodes, ",") {
		t.Fatalf("graphNodes=%q want %q", got, strings.Join(appNodes, ","))
	}

	now := time.Date(2026, 8, 31, 10, 0, 0, 0, time.UTC)
	recorder, err := newRuntimeRecorder(root, plan, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	summary := &RuntimeCoreSummary{State: "ready", HealthState: "ready", Live: true, Ready: true, RouteCount: 5, RPCServerCount: 1}
	if err := recorder.transition("biz", ProcessStarting, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := recorder.transition("biz", ProcessReady, summary, nil); err != nil {
		t.Fatal(err)
	}
	if err := recorder.setState(RuntimeRunRunning, "", false); err != nil {
		t.Fatal(err)
	}

	report, err := LoadRuntimeReport(root, plan.Runtime.StatePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Processes) != 1 || report.Processes[0].GraphNode != "" {
		t.Fatalf("unexpected report process: %#v", report.Processes)
	}
	if got := strings.Join(report.Processes[0].GraphNodes, ","); got != strings.Join(appNodes, ",") {
		t.Fatalf("report graphNodes=%q want %q", got, strings.Join(appNodes, ","))
	}
	if err := ValidateRuntimeClosure(plan, report); err != nil {
		t.Fatalf("valid multi-application closure rejected: %v", err)
	}

	runtimeGraph, err := BuildRuntimeGraph(plan, report)
	if err != nil {
		t.Fatal(err)
	}
	processID := applicationgraph.ID(applicationgraph.NodeProcess, "biz")
	runs := make(map[string]struct{})
	for _, edge := range runtimeGraph.Edges {
		if edge.From == processID && edge.Kind == applicationgraph.EdgeRuns {
			runs[edge.To] = struct{}{}
		}
	}
	for _, id := range appNodes {
		if _, ok := runs[id]; !ok {
			t.Fatalf("runtime graph missing runs edge %s -> %s: %#v", processID, id, runtimeGraph.Edges)
		}
	}
	if len(runs) != len(appNodes) {
		t.Fatalf("runs edges=%v", runs)
	}
}

func TestC117ARejectsAmbiguousOrLossyOwnership(t *testing.T) {
	root := t.TempDir()
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{
		{ID: "application:a", Kind: applicationgraph.NodeApplication, Name: "a"},
		{ID: "application:b", Kind: applicationgraph.NodeApplication, Name: "b"},
	}}
	base := DevManifest{SchemaVersion: RuntimeClosureSchemaVersion, Runtime: &RuntimeConfig{Application: "demo", Closure: true}}

	tests := []struct {
		name    string
		process Process
		want    string
	}{
		{name: "singular-and-plural", process: Process{Name: "api", Command: []string{"api"}, GraphNode: "application:a", GraphNodes: []string{"application:b"}}, want: "mutually exclusive"},
		{name: "empty-plural-node", process: Process{Name: "api", Command: []string{"api"}, GraphNodes: []string{"application:a", " "}}, want: "must not be empty"},
		{name: "duplicate-plural-node", process: Process{Name: "api", Command: []string{"api"}, GraphNodes: []string{"application:a", "application:a"}}, want: "duplicate graph node"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := base
			manifest.Processes = []Process{test.process}
			_, err := BuildPlan(manifest, root, nil, graph)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want %q", err, test.want)
			}
		})
	}

	legacySchema := DevManifest{SchemaVersion: DevSchemaVersion, Processes: []Process{{Name: "api", Command: []string{"api"}, GraphNodes: []string{"application:a"}}}}
	if _, err := BuildPlan(legacySchema, root, nil, graph); err == nil || !strings.Contains(err.Error(), "requires schemaVersion 3") {
		t.Fatalf("schema-v2 plural ownership err=%v", err)
	}
}

func TestC117ARejectsDuplicateOwnershipAcrossProcesses(t *testing.T) {
	root := t.TempDir()
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{{
		ID: "application:shared", Kind: applicationgraph.NodeApplication, Name: "shared",
	}}}
	readiness := func(port string) *Readiness {
		return &Readiness{URL: "http://127.0.0.1:" + port + "/_yunka/diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true}
	}
	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "demo", Closure: true},
		Processes: []Process{
			{Name: "one", Command: []string{"one"}, GraphNodes: []string{"application:shared"}, Readiness: readiness("18081")},
			{Name: "two", Command: []string{"two"}, GraphNodes: []string{"application:shared"}, Readiness: readiness("18082")},
		},
	}
	if _, err := BuildPlan(manifest, root, nil, graph); err == nil || !strings.Contains(err.Error(), "owned by both") {
		t.Fatalf("duplicate ownership err=%v", err)
	}
}

func TestC117APluralApplicationOwnershipStillRequiresDiagnosticsOnce(t *testing.T) {
	root := t.TempDir()
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{
		{ID: "application:a", Kind: applicationgraph.NodeApplication, Name: "a"},
		{ID: "application:b", Kind: applicationgraph.NodeApplication, Name: "b"},
	}}
	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "demo", Closure: true},
		Processes: []Process{{Name: "api", Command: []string{"api"}, GraphNodes: []string{"application:a", "application:b"}}},
	}
	if _, err := BuildPlan(manifest, root, nil, graph); err == nil || !strings.Contains(err.Error(), "readiness") {
		t.Fatalf("missing readiness err=%v", err)
	}
	manifest.Processes[0].Readiness = &Readiness{URL: "http://127.0.0.1:18080/_yunka/diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true}
	if _, err := BuildPlan(manifest, root, nil, graph); err != nil {
		t.Fatalf("single diagnostics barrier should cover explicit multi-application ownership: %v", err)
	}
}

func TestC117ALegacySingularRuntimeReportJSONIsUnchanged(t *testing.T) {
	contents, err := json.Marshal(ProcessRuntimeReport{Name: "api", GraphNode: "application:legacy", State: ProcessRunning})
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	if !strings.Contains(text, `"graphNode":"application:legacy"`) {
		t.Fatalf("legacy graphNode missing: %s", text)
	}
	if strings.Contains(text, "graphNodes") {
		t.Fatalf("legacy report unexpectedly emits graphNodes: %s", text)
	}
}

func TestC117AClosureRejectsPluralOwnershipDrift(t *testing.T) {
	plan := Plan{
		Closure: true,
		Runtime: &RuntimeConfig{Application: "demo"},
		BaseGraph: applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{
			{ID: "application:a", Kind: applicationgraph.NodeApplication, Name: "a"},
			{ID: "application:b", Kind: applicationgraph.NodeApplication, Name: "b"},
		}},
		Processes: []Process{{
			Name: "api", Command: []string{"api"}, GraphNodes: []string{"application:a", "application:b"},
			Readiness: &Readiness{URL: "http://127.0.0.1:18080/_yunka/diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true},
		}},
	}
	report := RuntimeReport{
		SchemaVersion: RuntimeReportSchemaVersion,
		Application:   "demo",
		State:         RuntimeRunRunning,
		Plan:          []string{"api"},
		Processes: []ProcessRuntimeReport{{
			Name: "api", GraphNodes: []string{"application:a"}, State: ProcessReady, Ready: true,
			Diagnostics: &RuntimeCoreSummary{State: "ready", HealthState: "ready", Live: true, Ready: true},
		}},
	}
	if err := ValidateRuntimeClosure(plan, report); err == nil || !strings.Contains(err.Error(), "ownership drift") {
		t.Fatalf("ownership drift err=%v", err)
	}
}
