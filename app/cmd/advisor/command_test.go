package advisor

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
	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/auditcore"
)

func TestBuildRequestReusesAuditAndRemainsReadOnly(t *testing.T) {
	root := t.TempDir()
	writeAdvisorFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	writeAdvisorFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), "syntax = \"proto3\";\n")
	manifest := contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Files:         []contract.File{{Name: "tenant.proto", Domain: &contract.DomainDeclaration{Name: "tenant"}}},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeAdvisorFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(manifestBytes, '\n')))
	writeAdvisorFile(t, filepath.Join(root, "internal", "tenant", "application", "service.go"), `package application

import "github.com/hvritual/yunka.io/framework/platform"

var _ = platform.Provider{}
`)

	before, err := advisorTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	first, err := BuildRequest(root, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := BuildRequest(root, "")
	if err != nil {
		t.Fatal(err)
	}
	after, err := advisorTreeDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("advisor request mutated project: before=%s after=%s", before, after)
	}
	if len(first.Audit.Findings) != 1 || first.Audit.Findings[0].Rule != auditcore.RulePlatformProviderBypass {
		t.Fatalf("advisor did not reuse canonical Audit finding: %#v", first.Audit.Findings)
	}
	firstJSON, err := advisorcore.MarshalRequest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondJSON, err := advisorcore.MarshalRequest(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstJSON) != string(secondJSON) {
		t.Fatal("advisor request output is not byte-stable")
	}
	text, err := RenderRequest(first, "text")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "authority=advisory_only") || !strings.Contains(text, "findings=1") || !strings.Contains(text, "mutationAuthorized=false") {
		t.Fatalf("unexpected advisor request text: %s", text)
	}
}

func TestValidateFilesProducesAdvisoryOnlyAttestation(t *testing.T) {
	request, err := advisorcore.NewRequest(testCommandAuditReport())
	if err != nil {
		t.Fatal(err)
	}
	requestBytes, err := advisorcore.MarshalRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	response := advisorcore.Response{
		SchemaVersion: advisorcore.SchemaVersion,
		RequestDigest: request.RequestDigest,
		Recommendations: []advisorcore.Recommendation{{
			ID:         "R-1",
			Kind:       advisorcore.KindRemediationRecommendation,
			Title:      "Use the typed capability boundary",
			Rationale:  "The recommendation is bound to a proven direct provider bypass.",
			FindingIDs: []string{"PROVEN-1"},
		}},
	}
	responseBytes, err := advisorcore.MarshalResponse(response)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	requestPath := filepath.Join(root, "request.json")
	responsePath := filepath.Join(root, "response.json")
	writeAdvisorFile(t, requestPath, string(requestBytes))
	writeAdvisorFile(t, responsePath, string(responseBytes))

	attestation, err := ValidateFiles(requestPath, responsePath)
	if err != nil {
		t.Fatal(err)
	}
	if attestation.Authority != advisorcore.AuthorityAdvisoryOnly || attestation.Result != advisorcore.ResultValid || len(attestation.Bindings) != 1 {
		t.Fatalf("unexpected attestation: %#v", attestation)
	}
	output, err := RenderAttestation(attestation, "agent-json")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"authority": "advisory_only"`, `"result": "valid"`, `"recommendationId": "R-1"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("attestation output missing %q:\n%s", expected, output)
		}
	}
}

func TestValidateFilesRequiresExplicitRequestAndResponse(t *testing.T) {
	if _, err := ValidateFiles("", ""); err == nil || !strings.Contains(err.Error(), "--request and --response are required") {
		t.Fatalf("expected required path failure, got %v", err)
	}
}

func testCommandAuditReport() auditcore.Report {
	report := auditcore.NewReport(auditcore.ProjectIdentity{GoModule: "example.com/demo"})
	report.Findings = []auditcore.Finding{{
		ID:        "PROVEN-1",
		Rule:      auditcore.RulePlatformProviderBypass,
		Class:     auditcore.FindingProvenViolation,
		Subject:   "tenant",
		Summary:   "application imports the framework platform provider directly",
		Invariant: "business Applications receive declared typed capabilities",
		Evidence:  []auditcore.Evidence{{Kind: auditcore.EvidenceSource, Source: "go.import", Path: "internal/tenant/application/service.go", Detail: "github.com/hvritual/yunka.io/framework/platform"}},
	}}
	auditcore.Normalize(&report)
	return report
}

func writeAdvisorFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func advisorTreeDigest(root string) (string, error) {
	var records []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
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
