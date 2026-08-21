package devruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	applicationgraph "yunka.io/pkg/applicationgraph"
)

func TestRuntimeClosurePlanRequiresUniqueGraphOwnership(t *testing.T) {
	root := t.TempDir()
	graph := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{
		{ID: "service:a", Kind: applicationgraph.NodeService, Name: "a"},
		{ID: "service:b", Kind: applicationgraph.NodeService, Name: "b"},
	}}
	manifest := DevManifest{SchemaVersion: RuntimeClosureSchemaVersion, Runtime: &RuntimeConfig{Closure: true}, Processes: []Process{
		{Name: "a", Command: []string{"a"}, GraphNode: "service:a"},
		{Name: "b", Command: []string{"b"}, GraphNode: "service:a", DependsOn: []string{"a"}},
	}}
	if _, err := BuildPlan(manifest, root, nil, graph); err == nil || !strings.Contains(err.Error(), "owned by both") {
		t.Fatalf("duplicate ownership err=%v", err)
	}
	manifest.Processes[1].GraphNode = ""
	if _, err := BuildPlan(manifest, root, nil, graph); err == nil || !strings.Contains(err.Error(), "graphNode") {
		t.Fatalf("missing graphNode err=%v", err)
	}
	manifest.Processes[1].GraphNode = "service:b"
	plan, err := BuildPlan(manifest, root, nil, graph)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Closure || plan.Runtime == nil || !reflect.DeepEqual(plan.Names(), []string{"a", "b"}) {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestBuildRuntimeGraphIncludesDeclaredAndObservedProcessEvidence(t *testing.T) {
	base := applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: []applicationgraph.Node{
		{ID: "service:db", Kind: applicationgraph.NodeService, Name: "db"},
		{ID: "service:api", Kind: applicationgraph.NodeService, Name: "api"},
	}}
	plan := Plan{
		Processes: []Process{
			{Name: "db", GraphNode: "service:db"},
			{Name: "api", GraphNode: "service:api", DependsOn: []string{"db"}},
		},
		Runtime: &RuntimeConfig{Application: "local"}, BaseGraph: base, Closure: true,
	}
	report := RuntimeReport{SchemaVersion: RuntimeReportSchemaVersion, Application: "local", Processes: []ProcessRuntimeReport{
		{Name: "db", GraphNode: "service:db", State: ProcessRunning},
		{Name: "api", GraphNode: "service:api", State: ProcessReady, Ready: true, Diagnostics: &RuntimeCoreSummary{State: "ready", Live: true, Ready: true, RouteCount: 3}},
	}}
	graph, err := BuildRuntimeGraph(plan, report)
	if err != nil {
		t.Fatal(err)
	}
	processID := applicationgraph.ID(applicationgraph.NodeProcess, "api")
	node, ok := graph.Node(processID)
	if !ok || node.Attributes["state"] != string(ProcessReady) || node.Attributes["routeCount"] != "3" {
		t.Fatalf("process node=%+v ok=%v", node, ok)
	}
	var runs, depends bool
	for _, edge := range graph.Edges {
		if edge.From == processID && edge.Kind == applicationgraph.EdgeRuns && edge.To == "service:api" {
			runs = true
		}
		if edge.From == processID && edge.Kind == applicationgraph.EdgeDependsOn && edge.To == applicationgraph.ID(applicationgraph.NodeProcess, "db") {
			depends = true
		}
	}
	if !runs || !depends {
		t.Fatalf("runs=%v depends=%v edges=%+v", runs, depends, graph.Edges)
	}
}

func TestRuntimeReportAtomicWriteRedactsSecretsAndRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	report := RuntimeReport{
		SchemaVersion: RuntimeReportSchemaVersion, Application: "app", State: RuntimeRunFailed,
		Plan: []string{"api"}, StartedAt: time.Now().UTC().Format(time.RFC3339Nano), UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Reason:    sanitizeRuntimeError("token=super-secret Authorization:Bearer abc.def.ghi password=hunter2"),
		Processes: []ProcessRuntimeReport{{Name: "api", State: ProcessFailed, Error: sanitizeRuntimeError("Bearer abc.def.ghi")}},
	}
	if err := WriteRuntimeReport(root, ".yunka/runtime.json", report); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".yunka", "runtime.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode=%o", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, secret := range []string{"super-secret", "abc.def.ghi", "hunter2"} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret %q leaked in %s", secret, text)
		}
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := WriteRuntimeReport(root, "linked/runtime.json", report); err == nil {
		t.Fatal("expected symlink escape rejection")
	}
}

func TestProbeReadinessCapturesOnlySafeCoreSummary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"core":{"state":"ready","health":{"state":"ready","live":true,"ready":true},"runtime":{"routeCount":7,"rpcClientConfigured":true,"rpcServerCount":2,"eventBusConfigured":true}},"components":[{"name":"secret","data":{"token":"must-not-leak"}}]}`))
	}))
	defer server.Close()
	ready, summary, err := probeReadinessSnapshot(context.Background(), server.Client(), &Readiness{
		URL: server.URL, DiagnosticsReady: true, CaptureDiagnostics: true,
	}, "")
	if err != nil || !ready || summary == nil || !summary.Ready || summary.RouteCount != 7 {
		t.Fatalf("ready=%v summary=%+v err=%v", ready, summary, err)
	}
	encoded, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "must-not-leak") || strings.Contains(string(encoded), "components") {
		t.Fatalf("unsafe diagnostics retained: %s", encoded)
	}
}

func TestRunGracefulShutdownUsesReversePlanOrder(t *testing.T) {
	root := t.TempDir()
	orderPath := filepath.Join(root, "shutdown-order.txt")
	plan := c5ProcessPlan(t, root, "2s", []Process{
		{Name: "db", Command: c5HelperCommand("signal", orderPath, "db"), GraphNode: "service:db"},
		{Name: "api", Command: c5HelperCommand("signal", orderPath, "api"), GraphNode: "service:api", DependsOn: []string{"db"}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, plan, RunOptions{Root: root, Environ: append(os.Environ(), "GO_WANT_C5_HELPER=1")})
	}()
	waitForRuntimeStates(t, root, []string{"db", "api"}, ProcessRunning)
	waitForC5HelperFiles(t, orderPath+".ready.db", orderPath+".ready.api")
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runtime did not stop")
	}
	data, err := os.ReadFile(orderPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(string(data))
	if !reflect.DeepEqual(lines, []string{"api", "db"}) {
		t.Fatalf("shutdown order=%v", lines)
	}
}

func TestRunShutdownTimeoutKillsUnresponsiveChild(t *testing.T) {
	root := t.TempDir()
	plan := c5ProcessPlan(t, root, "100ms", []Process{{
		Name: "stuck", Command: c5HelperCommand("ignore", filepath.Join(root, "ignore-ready")), GraphNode: "service:stuck",
	}})
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- Run(ctx, plan, RunOptions{Root: root, Environ: append(os.Environ(), "GO_WANT_C5_HELPER=1")})
	}()
	waitForRuntimeStates(t, root, []string{"stuck"}, ProcessRunning)
	waitForC5HelperFiles(t, filepath.Join(root, "ignore-ready"))
	started := time.Now()
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("kill fallback did not complete")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("shutdown too slow: %s", elapsed)
	}
}

func TestUnexpectedExitStopsRemainingChildren(t *testing.T) {
	root := t.TempDir()
	orderPath := filepath.Join(root, "shutdown-order.txt")
	plan := c5ProcessPlan(t, root, "2s", []Process{
		{Name: "worker", Command: c5HelperCommand("signal", orderPath, "worker"), GraphNode: "service:worker"},
		{Name: "crash", Command: c5HelperCommand("exit-after-file", orderPath+".ready.worker"), GraphNode: "service:crash", DependsOn: []string{"worker"}},
	})
	err := Run(context.Background(), plan, RunOptions{Root: root, Environ: append(os.Environ(), "GO_WANT_C5_HELPER=1")})
	if err == nil || !strings.Contains(err.Error(), "crash") || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("err=%v", err)
	}
	data, readErr := os.ReadFile(orderPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if strings.TrimSpace(string(data)) != "worker" {
		t.Fatalf("remaining child not shut down: %q", data)
	}
	report, loadErr := LoadRuntimeReport(root, ".yunka/dev-runtime.json")
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if report.State != RuntimeRunFailed {
		t.Fatalf("report state=%s", report.State)
	}
}

func c5ProcessPlan(t *testing.T, root, shutdown string, processes []Process) Plan {
	t.Helper()
	nodes := make([]applicationgraph.Node, 0, len(processes))
	for _, process := range processes {
		nodes = append(nodes, applicationgraph.Node{ID: process.GraphNode, Kind: applicationgraph.NodeService, Name: strings.TrimPrefix(process.GraphNode, "service:")})
	}
	manifest := DevManifest{
		SchemaVersion: RuntimeClosureSchemaVersion,
		Runtime:       &RuntimeConfig{Application: "test", StatePath: ".yunka/dev-runtime.json", GraphPath: ".yunka/runtime-graph.json", ShutdownTimeout: shutdown, Closure: true},
		Processes:     processes,
	}
	plan, err := BuildPlan(manifest, root, nil, applicationgraph.Graph{SchemaVersion: applicationgraph.SchemaVersion, Nodes: nodes})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func c5HelperCommand(args ...string) []string {
	return append([]string{os.Args[0], "-test.run=TestC5HelperProcess", "--"}, args...)
}

func waitForRuntimeStates(t *testing.T, root string, names []string, state ProcessState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		report, err := LoadRuntimeReport(root, ".yunka/dev-runtime.json")
		if err == nil {
			states := make(map[string]ProcessState, len(report.Processes))
			for _, process := range report.Processes {
				states[process.Name] = process.State
			}
			ready := true
			for _, name := range names {
				if states[name] != state {
					ready = false
					break
				}
			}
			if ready {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("processes %v did not reach %s", names, state)
}

func waitForC5HelperFiles(t *testing.T, paths ...string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for _, path := range paths {
			if _, err := os.Stat(path); err != nil {
				ready = false
				break
			}
		}
		if ready {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("helper readiness files not created: %v", paths)
}

func TestC5HelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_C5_HELPER") != "1" {
		return
	}
	separator := -1
	for index, argument := range os.Args {
		if argument == "--" {
			separator = index
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		os.Exit(2)
	}
	args := os.Args[separator+1:]
	switch args[0] {
	case "exit":
		os.Exit(23)
	case "exit-after-file":
		if len(args) != 2 {
			os.Exit(2)
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if _, err := os.Stat(args[1]); err == nil {
				os.Exit(23)
			}
			if time.Now().After(deadline) {
				os.Exit(5)
			}
			time.Sleep(10 * time.Millisecond)
		}
	case "ignore":
		if len(args) != 2 {
			os.Exit(2)
		}
		signal.Ignore()
		if err := os.WriteFile(args[1], []byte("ready\n"), 0o600); err != nil {
			os.Exit(3)
		}
		select {}
	case "signal":
		if len(args) != 3 {
			os.Exit(2)
		}
		channel := make(chan os.Signal, 1)
		signal.Notify(channel)
		if err := os.WriteFile(args[1]+".ready."+args[2], []byte("ready\n"), 0o600); err != nil {
			os.Exit(3)
		}
		<-channel
		file, err := os.OpenFile(args[1], os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			os.Exit(3)
		}
		_, err = fmt.Fprintln(file, args[2])
		_ = file.Close()
		if err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
