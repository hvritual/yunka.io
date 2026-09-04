package agentcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildConventionalProjectProducesStableReadOnlyContext(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(root, "contracts", "proto", "demo.proto"), "syntax = \"proto3\";\n")
	mustWrite(t, filepath.Join(root, "contracts", "generated", "manifest.json"), "{\"schemaVersion\":1}\n")
	mustWrite(t, filepath.Join(root, "contracts", "generated", "operation-plans.json"), "[]\n")
	mustWrite(t, filepath.Join(root, "contracts", "generated", "assembly-plan.json"), "{}\n")
	mustWrite(t, filepath.Join(root, "contracts", "generated", "application-graph.json"), "{}\n")

	before, err := treeDigest(root)
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
	after, err := treeDigest(root)
	if err != nil {
		t.Fatal(err)
	}

	if before != after {
		t.Fatalf("context command mutated project: before=%s after=%s", before, after)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("context snapshot is not deterministic:\nfirst=%#v\nsecond=%#v", first, second)
	}
	if SchemaVersion != 4 || first.SchemaVersion != 4 {
		t.Fatalf("schema version constant/snapshot=%d/%d want=4/4", SchemaVersion, first.SchemaVersion)
	}
	if first.Project.Profiled {
		t.Fatal("conventional project unexpectedly reported as profiled")
	}
	if first.Project.GoModule != "example.com/demo" {
		t.Fatalf("go module=%q", first.Project.GoModule)
	}
	if first.Project.ContractSourceKind != "proto-root" || first.Project.ContractSource != "contracts/proto" {
		t.Fatalf("contract source=%s %s", first.Project.ContractSourceKind, first.Project.ContractSource)
	}
	assertLocation(t, first, "operation-plans", "generated", "present")
	assertLocation(t, first, "provider-manifest", "managed", "missing")
	if first.Commands.Check != "yunka check --format agent-json" {
		t.Fatalf("check command=%q", first.Commands.Check)
	}
	if first.AgentProtocol.NewStructure != "yunka add <application|event|module> ..." {
		t.Fatalf("new structure command=%q", first.AgentProtocol.NewStructure)
	}
	if first.AgentProtocol.NewOperationPlan != "yunka add operation <application> <operation> ... --plan --format agent-json" {
		t.Fatalf("new operation plan command=%q", first.AgentProtocol.NewOperationPlan)
	}
	if first.AgentProtocol.NewOperationApply != "yunka add operation <application> <operation> ... --format agent-json" {
		t.Fatalf("new operation apply command=%q", first.AgentProtocol.NewOperationApply)
	}
	if first.AgentProtocol.ExistingPlan != "yunka change plan --operation <operation> --format agent-json" {
		t.Fatalf("change plan command=%q", first.AgentProtocol.ExistingPlan)
	}
	if first.AgentProtocol.ChangeBegin != "yunka change begin --operation <operation> --format agent-json" {
		t.Fatalf("change begin command=%q", first.AgentProtocol.ChangeBegin)
	}
	if first.AgentProtocol.ChangeCheck != "yunka change check --format agent-json" {
		t.Fatalf("change check command=%q", first.AgentProtocol.ChangeCheck)
	}
	if first.AgentProtocol.ChangeVerify != "yunka change verify --format agent-json" {
		t.Fatalf("change verify command=%q", first.AgentProtocol.ChangeVerify)
	}
	if first.AgentProtocol.ChangeSetBegin != "yunka change set begin [--contract <contract.json>] [--create-plan <plan.json>] --format agent-json" {
		t.Fatalf("change set begin command=%q", first.AgentProtocol.ChangeSetBegin)
	}
	if first.AgentProtocol.ChangeSetCheck != "yunka change set check --format agent-json" {
		t.Fatalf("change set check command=%q", first.AgentProtocol.ChangeSetCheck)
	}
	if first.AgentProtocol.RemediationBind != "yunka change set remediation bind --finding <finding-id> --format agent-json" {
		t.Fatalf("remediation bind command=%q", first.AgentProtocol.RemediationBind)
	}
	if first.AgentProtocol.RemediationCheck != "yunka change set remediation check --format agent-json" {
		t.Fatalf("remediation check command=%q", first.AgentProtocol.RemediationCheck)
	}
	if first.AgentProtocol.Audit != "yunka audit --format agent-json" {
		t.Fatalf("audit command=%q", first.AgentProtocol.Audit)
	}
	if first.AgentProtocol.AdvisorRequest != "yunka advisor request --format agent-json" {
		t.Fatalf("advisor request command=%q", first.AgentProtocol.AdvisorRequest)
	}
	if first.AgentProtocol.AdvisorValidate != "yunka advisor validate --request <request.json> --response <response.json> --format agent-json" {
		t.Fatalf("advisor validate command=%q", first.AgentProtocol.AdvisorValidate)
	}
	if first.AgentProtocol.RuntimeEvent != "yunka dev --event-format jsonl" {
		t.Fatalf("runtime event command=%q", first.AgentProtocol.RuntimeEvent)
	}
	jsonOne, err := MarshalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	jsonTwo, err := MarshalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(jsonOne) != string(jsonTwo) {
		t.Fatal("machine-readable output is not byte-stable")
	}
	for _, expected := range []string{
		"\"schemaVersion\": 4",
		"\"newOperationPlan\"",
		"\"newOperationApply\"",
		"\"changeSetBegin\"",
		"\"changeSetCheck\"",
		"\"remediationBind\"",
		"\"remediationCheck\"",
		"\"advisorRequest\"",
		"\"advisorValidate\"",
	} {
		if !strings.Contains(string(jsonOne), expected) {
			t.Fatalf("context v4 json missing %q:\n%s", expected, jsonOne)
		}
	}
}

func TestBuildReportsMissingGeneratedArtifactsWithoutInventingSemantics(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "contracts", "proto"), 0o755); err != nil {
		t.Fatal(err)
	}

	snapshot, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	item := assertLocation(t, snapshot, "assembly-plan", "generated", "missing")
	if item.Remediation != "run `yunka generate`" {
		t.Fatalf("remediation=%q", item.Remediation)
	}
}

func assertLocation(t *testing.T, snapshot Snapshot, name, role, state string) Location {
	t.Helper()
	for _, item := range snapshot.Locations {
		if item.Name == name {
			if item.Role != role || item.State != state {
				t.Fatalf("location %s role/state=%s/%s want=%s/%s", name, item.Role, item.State, role, state)
			}
			return item
		}
	}
	t.Fatalf("location %s not found", name)
	return Location{}
}

func mustWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func treeDigest(root string) (string, error) {
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(contents)
		records = append(records, filepath.ToSlash(rel)+":"+hex.EncodeToString(digest[:]))
		return nil
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(strings.Join(records, "\n")))
	return hex.EncodeToString(digest[:]), nil
}
