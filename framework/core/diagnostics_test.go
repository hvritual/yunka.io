package core

import (
	"context"
	"reflect"
	"testing"
)

func TestRouterHandleTreePathsAreSorted(t *testing.T) {
	tree := NewHandleTree()
	tree.Insert("/z", func(Service) ([]byte, error) { return nil, nil })
	tree.Insert("/a", func(Service) ([]byte, error) { return nil, nil })
	tree.Insert("/m/*id", func(Service) ([]byte, error) { return nil, nil })
	want := []string{"/a", "/m/*id", "/z"}
	if got := tree.Paths(); !reflect.DeepEqual(got, want) {
		t.Fatalf("paths=%v want=%v", got, want)
	}
}

func TestAppDiagnosticsIncludesReadOnlyInventory(t *testing.T) {
	app := GetApp()
	path := "/_w07_test"
	app.GetHandleTree().Insert(path, func(Service) ([]byte, error) { return nil, nil })
	defer app.GetHandleTree().Delete(path)

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
