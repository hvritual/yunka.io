package moduleidentity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectFindsOnlyLegacyModuleIdentitySurfaces(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), `module example.com/demo

go 1.25.0

require yunka.io/framework v0.1.0
replace yunka.io/gateway => ../gateway
`)
	writeTestFile(t, filepath.Join(root, "internal", "service.go"), `package internal

import (
	legacy "yunka.io/framework/core/modulecatalog"
	"github.com/hvritual/yunka.io/gateway/authz"
)

var _ legacy.Descriptor
const observation = "yunka.io/pkg/contractdsl/v1"
`)
	writeTestFile(t, filepath.Join(root, "vendor", "ignored.go"), `package vendor
import _ "yunka.io/pkg/logExt"
`)

	report, err := Inspect(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.Conformant {
		t.Fatal("legacy module identities unexpectedly conformed")
	}
	if len(report.Findings) != 3 {
		t.Fatalf("findings=%#v", report.Findings)
	}
	if report.Findings[0].Path != "go.mod" || report.Findings[0].Legacy != "yunka.io/framework" {
		t.Fatalf("first finding=%#v", report.Findings[0])
	}
	if report.Findings[1].Path != "go.mod" || report.Findings[1].Legacy != "yunka.io/gateway" {
		t.Fatalf("second finding=%#v", report.Findings[1])
	}
	if report.Findings[2].Path != "internal/service.go" || report.Findings[2].Legacy != "yunka.io/framework/core/modulecatalog" {
		t.Fatalf("third finding=%#v", report.Findings[2])
	}
	for _, finding := range report.Findings {
		if finding.Legacy == "yunka.io/pkg/contractdsl/v1" {
			t.Fatalf("ordinary string literal must not be treated as an import: %#v", finding)
		}
	}
}

func TestMigrateRewritesImportSpecsAndModuleTokensWithoutTouchingOpaqueStrings(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	goFile := filepath.Join(root, "generated.go")
	writeTestFile(t, goMod, `module example.com/demo

go 1.25.0

require (
	yunka.io/framework v0.1.0
	yunka.io/pkg v0.1.0
)
replace yunka.io/gateway => ../gateway
`)
	writeTestFile(t, goFile, "package demo\n\nimport (\n\t\"yunka.io/framework/core/modulecatalog\"\n\t_ `yunka.io/gateway/authz`\n)\n\nconst rawDescriptor = \"yunka.io/pkg/contractdsl/v1\"\n")

	result, err := Migrate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Conformant || len(result.After) != 0 {
		t.Fatalf("migration=%#v", result)
	}
	if len(result.ChangedFiles) != 2 || result.ChangedFiles[0] != "generated.go" || result.ChangedFiles[1] != "go.mod" {
		t.Fatalf("changedFiles=%#v", result.ChangedFiles)
	}
	contents, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, expected := range []string{
		`"github.com/hvritual/yunka.io/framework/core/modulecatalog"`,
		"`github.com/hvritual/yunka.io/gateway/authz`",
		`const rawDescriptor = "yunka.io/pkg/contractdsl/v1"`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("migrated Go file missing %q:\n%s", expected, text)
		}
	}
	modContents, err := os.ReadFile(goMod)
	if err != nil {
		t.Fatal(err)
	}
	modText := string(modContents)
	for _, expected := range []string{
		"github.com/hvritual/yunka.io/framework v0.1.0",
		"github.com/hvritual/yunka.io/pkg v0.1.0",
		"replace github.com/hvritual/yunka.io/gateway => ../gateway",
	} {
		if !strings.Contains(modText, expected) {
			t.Fatalf("migrated go.mod missing %q:\n%s", expected, modText)
		}
	}

	second, err := Migrate(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.ChangedFiles) != 0 || len(second.Before) != 0 || !second.Conformant {
		t.Fatalf("second migration is not idempotent: %#v", second)
	}
}

func TestCanonicalizeRejectsPrefixLookalikes(t *testing.T) {
	for _, value := range []string{"yunka.io/frameworkx", "example.com/yunka.io/framework", "github.com/hvritual/yunka.io/framework"} {
		if got, changed := Canonicalize(value); changed || got != value {
			t.Fatalf("canonicalize(%q)=(%q,%t)", value, got, changed)
		}
	}
	got, changed := Canonicalize("yunka.io/pkg/contractdsl/v1")
	if !changed || got != "github.com/hvritual/yunka.io/pkg/contractdsl/v1" {
		t.Fatalf("canonicalize legacy package=(%q,%t)", got, changed)
	}
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
