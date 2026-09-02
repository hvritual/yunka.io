package devruntime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	applicationgraph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/assemblyplan"
)

func TestRuntimeClosureDerivesSingleProcessOwnershipFromAssemblyPlan(t *testing.T) {
	root := t.TempDir()
	plan := testAssemblyPlan(t)
	path := filepath.Join(root, filepath.FromSlash(DefaultRuntimeAssemblyPlan))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := assemblyplan.Save(path, plan); err != nil {
		t.Fatal(err)
	}

	graph := testAssemblyApplicationGraph(plan)
	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "biz", Closure: true},
		Processes: []Process{{
			Name:    "biz",
			Command: []string{"biz"},
			Readiness: &Readiness{
				URL:                "http://127.0.0.1:18080/__yunka/diagnostics",
				DiagnosticsReady:   true,
				CaptureDiagnostics: true,
			},
		}},
	}

	resolved, err := BuildPlan(manifest, root, nil, graph)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Processes) != 1 {
		t.Fatalf("processes=%d", len(resolved.Processes))
	}
	got := strings.Join(resolved.Processes[0].GraphNodes, ",")
	want := "application:access/tenant_lifecycle,application:access/tenant_member_lifecycle,application:access/tenant_role_permission"
	if got != want {
		t.Fatalf("derived ownership=%q want %q", got, want)
	}
	if manifest.Processes[0].GraphNode != "" || len(manifest.Processes[0].GraphNodes) != 0 {
		t.Fatal("consumer manifest must not be mutated or become the ownership source")
	}
}

func TestRuntimeClosureRejectsLegacyOwnershipThatDriftsFromAssemblyPlan(t *testing.T) {
	root := t.TempDir()
	plan := testAssemblyPlan(t)
	path := filepath.Join(root, filepath.FromSlash(DefaultRuntimeAssemblyPlan))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := assemblyplan.Save(path, plan); err != nil {
		t.Fatal(err)
	}

	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "biz", Closure: true},
		Processes: []Process{{
			Name:       "biz",
			Command:    []string{"biz"},
			GraphNodes: []string{"application:access/tenant_lifecycle"},
			Readiness: &Readiness{
				URL:                "http://127.0.0.1:18080/__yunka/diagnostics",
				DiagnosticsReady:   true,
				CaptureDiagnostics: true,
			},
		}},
	}
	_, err := BuildPlan(manifest, root, nil, testAssemblyApplicationGraph(plan))
	if err == nil || !strings.Contains(err.Error(), "disagrees with generated assembly plan") {
		t.Fatalf("err=%v", err)
	}
}

func testAssemblyPlan(t *testing.T) assemblyplan.Plan {
	t.Helper()
	evidence := assemblyplan.Evidence{Ownership: assemblyplan.OwnershipCanonical, Source: "test", Ref: "test"}
	plan, err := assemblyplan.Compile(assemblyplan.Input{
		Identity: assemblyplan.RootTarget,
		Applications: []assemblyplan.ApplicationInput{
			{ID: "access/tenant_lifecycle", Domain: "access", Name: "tenant_lifecycle", DependsOn: []string{"access/tenant_member_lifecycle", "access/tenant_role_permission"}, Evidence: evidence},
			{ID: "access/tenant_member_lifecycle", Domain: "access", Name: "tenant_member_lifecycle", DependsOn: []string{"access/tenant_role_permission"}, Evidence: evidence},
			{ID: "access/tenant_role_permission", Domain: "access", Name: "tenant_role_permission", Evidence: evidence},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func testAssemblyApplicationGraph(plan assemblyplan.Plan) applicationgraph.Graph {
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion}
	for _, application := range plan.Applications {
		graph.Nodes = append(graph.Nodes, applicationgraph.Node{
			ID:   applicationgraph.ID(applicationgraph.NodeApplication, application.ID),
			Kind: applicationgraph.NodeApplication,
			Name: application.ID,
		})
	}
	return graph
}
