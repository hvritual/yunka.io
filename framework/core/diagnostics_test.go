package core

import (
	"context"
	"reflect"
	"testing"
)

func TestRouterHandleTreePathsAreSorted(t *testing.T) {
	tree := NewHandleTree()
	tree.Insert("/z", func(context.Context) ([]byte, error) { return nil, nil })
	tree.Insert("/a", func(context.Context) ([]byte, error) { return nil, nil })
	tree.Insert("/m/*id", func(context.Context) ([]byte, error) { return nil, nil })
	want := []string{"/a", "/m/*id", "/z"}
	if got := tree.Paths(); !reflect.DeepEqual(got, want) {
		t.Fatalf("paths=%v want=%v", got, want)
	}
}

func TestAppDiagnosticsIncludesReadOnlyInventory(t *testing.T) {
	app, err := NewApp(AppOptions{})
	if err != nil {
		t.Fatal(err)
	}
	path := "/_w07_test"
	app.GetHandleTree().Insert(path, func(context.Context) ([]byte, error) { return nil, nil })
	report := app.Diagnostics(context.Background())
	if report.SchemaVersion != DiagnosticsSchemaVersion {
		t.Fatalf("schemaVersion=%d", report.SchemaVersion)
	}
	if report.State == "" || report.Health.State == "" {
		t.Fatalf("missing state: %#v", report)
	}
	found := false
	for _, route := range report.Routes {
		found = found || route == path
	}
	if !found || report.Runtime.RouteCount != len(report.Routes) {
		t.Fatalf("unexpected routes: %#v", report)
	}
}
