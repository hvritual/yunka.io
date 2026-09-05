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
	writeTestFile(t, filepath.Join(root, "contracts", "third_party", "yunka", "dsl", "v1", "options.proto"), `syntax = "proto3";
package yunka.dsl.v1;
option go_package = "yunka.io/pkg/contractdsl/v1;contractdslv1";
// option go_package = "yunka.io/framework/ignored";
/*
option go_package = "yunka.io/gateway/ignored";
*/
option java_package = "yunka.io/pkg/not-a-go-package";
option objc_class_prefix = "option go_package = 'yunka.io/framework/inside-string';";
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
	if len(report.Findings) != 4 {
		t.Fatalf("findings=%#v", report.Findings)
	}
	want := map[string]bool{
		"go.mod\x00yunka.io/framework":                                 false,
		"go.mod\x00yunka.io/gateway":                                   false,
		"internal/service.go\x00yunka.io/framework/core/modulecatalog": false,
		"contracts/third_party/yunka/dsl/v1/options.proto\x00yunka.io/pkg/contractdsl/v1;contractdslv1": false,
	}
	for _, finding := range report.Findings {
		key := finding.Path + "\x00" + finding.Legacy
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected finding=%#v", finding)
		}
		want[key] = true
	}
	for key, found := range want {
		if !found {
			t.Fatalf("missing finding %q: %#v", key, report.Findings)
		}
	}
}

func TestMigrateRewritesKnownIdentitySourcesWithoutTouchingOpaqueStrings(t *testing.T) {
	root := t.TempDir()
	goMod := filepath.Join(root, "go.mod")
	goFile := filepath.Join(root, "generated.go")
	protoFile := filepath.Join(root, "contracts", "third_party", "yunka", "dsl", "v1", "options.proto")
	writeTestFile(t, goMod, `module example.com/demo

go 1.25.0

require (
	yunka.io/framework v0.1.0
	yunka.io/pkg v0.1.0
)
replace yunka.io/gateway => ../yunka.io/gateway
`)
	writeTestFile(t, goFile, "package demo\n\nimport (\n\t\"yunka.io/framework/core/modulecatalog\"\n\t_ `yunka.io/gateway/authz`\n)\n\nconst rawDescriptor = \"yunka.io/pkg/contractdsl/v1\"\n")
	writeTestFile(t, protoFile, `syntax = "proto3";
package yunka.dsl.v1;
option go_package = "yunka.io/pkg/contractdsl/v1;contractdslv1";
// option go_package = "yunka.io/framework/ignored";
/*
option go_package = "yunka.io/gateway/ignored";
*/
option java_package = "yunka.io/pkg/not-a-go-package";
option objc_class_prefix = "option go_package = 'yunka.io/framework/inside-string';";
`)

	result, err := Migrate(root)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Conformant || len(result.After) != 0 {
		t.Fatalf("migration=%#v", result)
	}
	wantChanged := []string{
		"contracts/third_party/yunka/dsl/v1/options.proto",
		"generated.go",
		"go.mod",
	}
	if strings.Join(result.ChangedFiles, "\n") != strings.Join(wantChanged, "\n") {
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
		"replace github.com/hvritual/yunka.io/gateway => ../yunka.io/gateway",
	} {
		if !strings.Contains(modText, expected) {
			t.Fatalf("migrated go.mod missing %q:\n%s", expected, modText)
		}
	}
	if strings.Contains(modText, "../github.com/hvritual/yunka.io/gateway") {
		t.Fatalf("migration rewrote local replace path as a module token:\n%s", modText)
	}

	protoContents, err := os.ReadFile(protoFile)
	if err != nil {
		t.Fatal(err)
	}
	protoText := string(protoContents)
	for _, expected := range []string{
		`option go_package = "github.com/hvritual/yunka.io/pkg/contractdsl/v1;contractdslv1";`,
		`// option go_package = "yunka.io/framework/ignored";`,
		`option go_package = "yunka.io/gateway/ignored";`,
		`option java_package = "yunka.io/pkg/not-a-go-package";`,
		`option objc_class_prefix = "option go_package = 'yunka.io/framework/inside-string';";`,
	} {
		if !strings.Contains(protoText, expected) {
			t.Fatalf("migrated proto missing preserved or canonical value %q:\n%s", expected, protoText)
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
	goPackage, changed := canonicalizeGoPackage("yunka.io/pkg/contractdsl/v1;contractdslv1")
	if !changed || goPackage != "github.com/hvritual/yunka.io/pkg/contractdsl/v1;contractdslv1" {
		t.Fatalf("canonicalize go_package=(%q,%t)", goPackage, changed)
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
