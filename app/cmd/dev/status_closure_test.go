package dev

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli"
	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
	"github.com/hvritual/yunka.io/pkg/devruntime"
)

func TestC103StatusClosureValidatesCurrentExplicitPlan(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".yunka"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := devruntime.DevManifest{
		SchemaVersion: devruntime.RuntimeClosureSchemaVersion,
		Runtime:       &devruntime.RuntimeConfig{Application: "demo", Closure: true},
		Processes: []devruntime.Process{{
			Name: "api", Command: []string{"api"}, GraphNode: "application:device/transfer",
			Readiness: &devruntime.Readiness{
				URL: "http://127.0.0.1:18080/_diagnostics", DiagnosticsReady: true, CaptureDiagnostics: true,
			},
		}},
	}
	writeDevJSON(t, filepath.Join(root, ".yunka", "dev.json"), manifest)
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{{
		ID: "application:device/transfer", Kind: applicationgraph.NodeApplication, Name: "device/transfer",
	}}}
	writeDevJSON(t, filepath.Join(root, ".yunka", "application-graph.json"), graph)

	report := devruntime.RuntimeReport{
		SchemaVersion: devruntime.RuntimeReportSchemaVersion,
		Application:   "demo",
		State:         devruntime.RuntimeRunStopping,
		Plan:          []string{"api"},
		Processes: []devruntime.ProcessRuntimeReport{{
			Name: "api", GraphNode: "application:device/transfer", State: devruntime.ProcessReady, Ready: true,
			Diagnostics: &devruntime.RuntimeCoreSummary{State: "ready", HealthState: "ready", Live: true, Ready: true, RouteCount: 1, RPCServerCount: 1},
		}},
	}
	if err := devruntime.WriteRuntimeReport(root, devruntime.DefaultRuntimeStatePath, report); err != nil {
		t.Fatal(err)
	}
	app := cli.NewApp()
	app.Commands = []cli.Command{Command()}
	err := app.Run([]string{"yunka", "dev", "status", "--root", root, "--closure"})
	if err == nil || !strings.Contains(err.Error(), "not running") {
		t.Fatalf("status --closure err=%v", err)
	}

	report.State = devruntime.RuntimeRunRunning
	if err := devruntime.WriteRuntimeReport(root, devruntime.DefaultRuntimeStatePath, report); err != nil {
		t.Fatal(err)
	}
	if err := app.Run([]string{"yunka", "dev", "status", "--root", root, "--closure"}); err != nil {
		t.Fatalf("valid status --closure rejected: %v", err)
	}
}

func writeDevJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
