package auditcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectGoSourceCapturesDurableDeclarations(t *testing.T) {
	root := t.TempDir()
	domainDir := filepath.Join(root, "internal", "access", "domain")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatal(err)
	}
	source := `package domain

// Stage2State persists an external workflow vocabulary term.
// yunka:audit-name-exception business-concept Stage2 is part of the persisted public vocabulary.
type Stage2State struct{}

type Tenant struct{}

func NewTenant() *Tenant { return &Tenant{} }
func (t *Tenant) Activate() {}
func helper() {}
`
	if err := os.WriteFile(filepath.Join(domainDir, "tenant.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	testSource := `package domain

import "testing"

func TestCE09Replay(t *testing.T) {}
func helperTest(t *testing.T) {}
`
	if err := os.WriteFile(filepath.Join(domainDir, "tenant_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatal(err)
	}

	snapshot, err := CollectGoSource(root, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("files = %d, want 2: %#v", len(snapshot.Files), snapshot.Files)
	}

	byPath := map[string]GoSourceFile{}
	for _, file := range snapshot.Files {
		byPath[file.Path] = file
	}
	production := byPath["internal/access/domain/tenant.go"]
	if len(production.Declarations) != 4 {
		t.Fatalf("production declarations = %d, want 4: %#v", len(production.Declarations), production.Declarations)
	}
	var exception string
	for _, declaration := range production.Declarations {
		if declaration.Name == "Stage2State" {
			exception = declaration.Exception
		}
		if declaration.Name == "helper" {
			t.Fatalf("unexported helper must not become durable audit identity")
		}
	}
	if exception != "business-concept: Stage2 is part of the persisted public vocabulary." {
		t.Fatalf("exception = %q", exception)
	}

	testFile := byPath["internal/access/domain/tenant_test.go"]
	if !testFile.Test {
		t.Fatalf("tenant_test.go must be marked test source")
	}
	if len(testFile.Declarations) != 1 || testFile.Declarations[0].Kind != "test" || testFile.Declarations[0].Name != "TestCE09Replay" {
		t.Fatalf("durable test identity = %#v", testFile.Declarations)
	}
}

func TestNameExceptionRequiresCategoryAndReason(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "bad.go"), []byte(`package sample

// yunka:audit-name-exception business-concept
type Stage2State struct{}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := CollectGoSource(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 1 || len(snapshot.Files[0].Declarations) != 1 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Files[0].Declarations[0].Exception != "" {
		t.Fatalf("exception without reason must not be accepted: %#v", snapshot.Files[0].Declarations[0])
	}
}
