package devruntime

import (
	"strings"
	"testing"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

func TestC103ClosureApplicationOwnerRequiresDiagnosticsBarrier(t *testing.T) {
	root := t.TempDir()
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{{
		ID: "application:device/transfer", Kind: applicationgraph.NodeApplication, Name: "device/transfer",
	}}}
	base := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "demo", Closure: true},
		Processes: []Process{{
			Name: "api", Command: []string{"api"}, GraphNode: "application:device/transfer",
		}},
	}

	if _, err := BuildPlan(base, root, nil, graph); err == nil || !strings.Contains(err.Error(), "readiness") {
		t.Fatalf("missing application readiness err=%v", err)
	}

	base.Processes[0].Readiness = &Readiness{URL: "http://127.0.0.1:18080/_diagnostics", CaptureDiagnostics: true}
	if _, err := BuildPlan(base, root, nil, graph); err == nil || !strings.Contains(err.Error(), "diagnosticsReady=true") {
		t.Fatalf("missing diagnosticsReady err=%v", err)
	}

	base.Processes[0].Readiness.DiagnosticsReady = true
	base.Processes[0].Readiness.CaptureDiagnostics = false
	if _, err := BuildPlan(base, root, nil, graph); err == nil || !strings.Contains(err.Error(), "captureDiagnostics=true") {
		t.Fatalf("missing captureDiagnostics err=%v", err)
	}

	base.Processes[0].Readiness.CaptureDiagnostics = true
	plan, err := BuildPlan(base, root, nil, graph)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Closure || plan.Runtime == nil || len(plan.Processes) != 1 {
		t.Fatalf("unexpected closure plan: %+v", plan)
	}
}

func TestC103ClosureKeepsNonApplicationOwnerCompatibility(t *testing.T) {
	root := t.TempDir()
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{{
		ID: "service:database", Kind: applicationgraph.NodeService, Name: "database",
	}}}
	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "demo", Closure: true},
		Processes: []Process{{
			Name: "database", Command: []string{"database"}, GraphNode: "service:database",
		}},
	}
	if _, err := BuildPlan(manifest, root, nil, graph); err != nil {
		t.Fatalf("non-application closure owner unexpectedly requires diagnostics: %v", err)
	}
}

func TestC103ValidateRuntimeClosureRequiresExactLiveApplicationEvidence(t *testing.T) {
	plan := Plan{
		Closure: true,
		Runtime: &RuntimeConfig{Application: "demo"},
		BaseGraph: applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{
			{ID: "service:database", Kind: applicationgraph.NodeService, Name: "database"},
			{ID: "application:device/transfer", Kind: applicationgraph.NodeApplication, Name: "device/transfer"},
		}},
		Processes: []Process{
			{Name: "database", Command: []string{"database"}, GraphNode: "service:database"},
			{Name: "api", Command: []string{"api"}, GraphNode: "application:device/transfer", DependsOn: []string{"database"}, Readiness: &Readiness{
				URL: "http://127.0.0.1:18080/_diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true,
			}},
		},
	}
	valid := func() RuntimeReport {
		return RuntimeReport{
			SchemaVersion: RuntimeReportSchemaVersion,
			Application:   "demo",
			State:         RuntimeRunRunning,
			Plan:          []string{"database", "api"},
			Processes: []ProcessRuntimeReport{
				{Name: "database", GraphNode: "service:database", State: ProcessRunning},
				{Name: "api", GraphNode: "application:device/transfer", State: ProcessReady, Ready: true, Diagnostics: &RuntimeCoreSummary{
					State: "ready", HealthState: "ready", Live: true, Ready: true, RouteCount: 2, RPCServerCount: 1,
				}},
			},
		}
	}

	if err := ValidateRuntimeClosure(plan, valid()); err != nil {
		t.Fatalf("valid closure rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*RuntimeReport)
		want string
	}{
		{name: "not-running", edit: func(report *RuntimeReport) { report.State = RuntimeRunStopping }, want: "not running"},
		{name: "plan-drift", edit: func(report *RuntimeReport) { report.Plan = []string{"api", "database"} }, want: "plan does not match"},
		{name: "ownership-drift", edit: func(report *RuntimeReport) { report.Processes[1].GraphNode = "service:database" }, want: "ownership drift"},
		{name: "not-ready", edit: func(report *RuntimeReport) { report.Processes[1].State = ProcessRunning; report.Processes[1].Ready = false }, want: "readiness"},
		{name: "missing-diagnostics", edit: func(report *RuntimeReport) { report.Processes[1].Diagnostics = nil }, want: "no captured diagnostics"},
		{name: "diagnostics-not-ready", edit: func(report *RuntimeReport) { report.Processes[1].Diagnostics.Ready = false }, want: "diagnostics are incomplete"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report := valid()
			test.edit(&report)
			err := ValidateRuntimeClosure(plan, report)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want substring %q", err, test.want)
			}
		})
	}
}
