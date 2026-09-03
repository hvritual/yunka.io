package audit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/auditcore"
)

func TestBuildUsesCanonicalManifestAndRemainsReadOnly(t *testing.T) {
	root := t.TempDir()
	writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files: []contract.File{
			{Name: "tenant.proto", Domain: &contract.DomainDeclaration{Name: "tenant"}},
			{Name: "device.proto", Domain: &contract.DomainDeclaration{Name: "device"}},
		},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(manifestBytes, '\n')))
	writeAuditProjectFile(t, filepath.Join(root, "internal", "tenant", "application", "service.go"), `package application

import (
	"example.com/demo/internal/device/ports"
	"github.com/hvritual/yunka.io/framework/platform"
	"github.com/hvritual/yunka.io/gateway/authz"
)

var _ = platform.Provider{}
var _ authz.Authorizer
`)

	before, err := auditTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	after, err := auditTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("audit mutated project tree: before=%s after=%s", before, after)
	}
	if len(first.Findings) != 3 {
		t.Fatalf("findings=%d want=3: %#v", len(first.Findings), first.Findings)
	}
	seen := map[string]bool{}
	for _, finding := range first.Findings {
		seen[finding.Rule] = true
	}
	for _, rule := range []string{auditcore.RuleCrossDomainRepositoryBypass, auditcore.RulePlatformProviderBypass, auditcore.RuleAuthorizationBypass} {
		if !seen[rule] {
			t.Fatalf("missing integrated finding %s: %#v", rule, first.Findings)
		}
	}
	firstJSON, err := Render(first, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := Render(second, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	if firstJSON != secondJSON {
		t.Fatalf("audit machine output is not deterministic:\nfirst:\n%s\nsecond:\n%s", firstJSON, secondJSON)
	}
}

func writeAuditProjectFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

// auditTreeDigest measures developer-visible project contents, not Git's
// internal object/index metadata. Audit may execute read-only Git plumbing for
// baseline evidence; repository-internal metadata is not a project mutation.
func auditTreeDigest(root string) (string, error) {
	var records []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != root && info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(contents)
		records = append(records, filepath.ToSlash(relative)+":"+hex.EncodeToString(digest[:]))
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(records)
	digest := sha256.Sum256([]byte(strings.Join(records, "\n")))
	return hex.EncodeToString(digest[:]), nil
}
