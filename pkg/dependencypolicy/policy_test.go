package dependencypolicy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckModuleGraphRejectsForbiddenAndVersionDrift(t *testing.T) {
	root := t.TempDir()
	policy := Policy{
		ForbiddenModules: []string{"legacy.example/module"},
		RequiredModules:  []ModuleRule{{Path: "modern.example/module", Version: "v2.0.0"}},
	}
	diagnostics := checkModuleGraph(root, []Module{
		{Path: "legacy.example/module", Version: "v1.0.0"},
		{Path: "modern.example/module", Version: "v1.9.0"},
	}, policy)
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestWorkspaceCompatibilityModuleMustBeLocalMainModule(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "compat", "go-kit-kit-log")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	goMod := filepath.Join(directory, "go.mod")
	if err := os.WriteFile(goMod, []byte("module github.com/go-kit/kit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	policy := Policy{WorkspaceModules: []WorkspaceModuleRule{{Path: "github.com/go-kit/kit", Directory: "compat/go-kit-kit-log"}}}
	good := checkModuleGraph(root, []Module{{Path: "github.com/go-kit/kit", Main: true, Dir: directory, GoMod: goMod}}, policy)
	if len(good) != 0 {
		t.Fatalf("good diagnostics=%#v", good)
	}
	replaced := checkModuleGraph(root, []Module{{Path: "github.com/go-kit/kit", Dir: directory, GoMod: goMod, Replace: &Module{Path: directory}}}, policy)
	if len(replaced) == 0 || !strings.Contains(replaced[0].Message, "workspace main module") {
		t.Fatalf("replaced diagnostics=%#v", replaced)
	}
}

func TestCheckReplacesRejectsExternalAndForbiddenModule(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "go.work", "go 1.25\nreplace google.golang.org/genproto => google.golang.org/genproto v1.0.0\n")
	policy := Policy{
		ModuleFiles:             []string{"go.work"},
		ForbidExternalReplaces:  true,
		ForbiddenReplaceModules: []string{"google.golang.org/genproto"},
	}
	diagnostics, err := checkReplaces(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestRequiredVersionScopedLocalReplacement(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "compat", "go-kit-kit-log"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, root, "go.work", "go 1.25\nreplace github.com/go-kit/kit v0.10.0 => ./compat/go-kit-kit-log\n")
	writeTestFile(t, root, "pkg/go.mod", "module example.com/pkg\ngo 1.25\nreplace github.com/go-kit/kit v0.10.0 => ../compat/go-kit-kit-log\n")
	policy := Policy{
		ModuleFiles:            []string{"go.work", "pkg/go.mod"},
		ForbidExternalReplaces: true,
		RequiredLocalReplacements: []LocalReplacementRule{{
			Path: "github.com/go-kit/kit", Version: "v0.10.0", Directory: "compat/go-kit-kit-log",
			Files: []string{"go.work", "pkg/go.mod"},
		}},
	}
	diagnostics, err := checkReplaces(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestLegacyImportAllowList(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "legacy/generated.pb.go", "package legacy\nimport _ \"github.com/golang/protobuf/proto\"\n")
	writeTestFile(t, root, "newcode/new.go", "package newcode\nimport _ \"github.com/golang/protobuf/proto\"\n")
	policy := Policy{LegacyImports: []LegacyImportRule{{
		Path:            "github.com/golang/protobuf",
		AllowedPrefixes: []string{"legacy/"},
	}}}
	diagnostics, err := checkLegacyImports(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Path != "newcode/new.go" {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestOldGoKitImportIsNotAllowedInFirstPartyCode(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "newcode/new.go", "package newcode\nimport _ \"github.com/go-kit/kit/log\"\n")
	policy := Policy{LegacyImports: []LegacyImportRule{{Path: "github.com/go-kit/kit"}}}
	diagnostics, err := checkLegacyImports(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "go-kit/kit/log") {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestCompatibilityModuleOwnsHistoricalGoKitImport(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "compat/go-kit-kit-log/log/level/compat.go", "package level\nimport _ \"github.com/go-kit/kit/log\"\n")
	policy := Policy{LegacyImports: []LegacyImportRule{{
		Path:            "github.com/go-kit/kit",
		AllowedPrefixes: []string{"compat/go-kit-kit-log/"},
	}}}
	diagnostics, err := checkLegacyImports(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestAliyunSDKImportIsRestrictedToApprovedAdapters(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, root, "pkg/aliLogStore/manager.go", "package aliLogStore\nimport _ \"github.com/aliyun/aliyun-log-go-sdk\"\n")
	writeTestFile(t, root, "newcode/new.go", "package newcode\nimport _ \"github.com/aliyun/aliyun-log-go-sdk\"\n")
	policy := Policy{LegacyImports: []LegacyImportRule{{
		Path:            "github.com/aliyun/aliyun-log-go-sdk",
		AllowedPrefixes: []string{"pkg/aliLogStore/"},
	}}}
	diagnostics, err := checkLegacyImports(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 || diagnostics[0].Path != "newcode/new.go" {
		t.Fatalf("diagnostics=%#v", diagnostics)
	}
}

func TestContainedPathRejectsEscape(t *testing.T) {
	if _, err := containedPath(t.TempDir(), "../outside"); err == nil {
		t.Fatal("expected path escape error")
	}
}

func writeTestFile(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
