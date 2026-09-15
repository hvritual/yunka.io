package auditcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectGoSourceCapturesPackageAndInterfaceDocumentation(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "internal", "tenant", "ports")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeAuditSource(t, filepath.Join(packageDir, "doc.go"), `// Package ports defines tenant-owned dependency contracts.
// Implementations remain outside the domain boundary.
package ports
`)
	writeAuditSource(t, filepath.Join(packageDir, "repository.go"), `package ports

// TenantRepository persists tenant state behind the application boundary.
type TenantRepository interface {
	Save() error
}

type UndocumentedRepository interface {
	Load() error
}
`)

	snapshot, err := CollectGoSource(root, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 2 {
		t.Fatalf("files=%d want=2: %#v", len(snapshot.Files), snapshot.Files)
	}
	byPath := map[string]GoSourceFile{}
	for _, file := range snapshot.Files {
		byPath[file.Path] = file
	}
	if !byPath["internal/tenant/ports/doc.go"].PackageDocumented {
		t.Fatal("doc.go package documentation was not captured")
	}
	repository := byPath["internal/tenant/ports/repository.go"]
	if repository.PackageDocumented {
		t.Fatal("ordinary source without package comment was marked package-documented")
	}
	seen := map[string]SourceDeclaration{}
	for _, declaration := range repository.Declarations {
		seen[declaration.Name] = declaration
	}
	if !seen["TenantRepository"].Contract || !seen["TenantRepository"].Documented {
		t.Fatalf("documented interface evidence=%#v", seen["TenantRepository"])
	}
	if !seen["UndocumentedRepository"].Contract || seen["UndocumentedRepository"].Documented {
		t.Fatalf("undocumented interface evidence=%#v", seen["UndocumentedRepository"])
	}
}

func TestDocumentationDirectiveAloneDoesNotCountAsDocumentation(t *testing.T) {
	root := t.TempDir()
	writeAuditSource(t, filepath.Join(root, "internal", "tenant", "domain", "state.go"), `// yunka:audit-name-exception business-concept Stage2 is an external vocabulary term.
package domain

// yunka:audit-name-exception business-concept Stage2 is an external vocabulary term.
type Stage2Contract interface{}
`)
	snapshot, err := CollectGoSource(root, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Files) != 1 {
		t.Fatalf("files=%d want=1", len(snapshot.Files))
	}
	file := snapshot.Files[0]
	if file.PackageDocumented {
		t.Fatal("governance directive alone must not satisfy package documentation")
	}
	if len(file.Declarations) != 1 || file.Declarations[0].Documented {
		t.Fatalf("governance directive alone must not satisfy contract documentation: %#v", file.Declarations)
	}
}
