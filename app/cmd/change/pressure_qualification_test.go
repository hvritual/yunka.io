package change

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"yunka.io/app/cmd/add"
	"yunka.io/app/cmd/projectflow"
)

type pressureFixture struct {
	Root         string
	ProtoPath    string
	ContractPath string
	Contract     ChangeContract
}

func TestAX7RealConsumerAdversarialPressure(t *testing.T) {
	fixture := newPressureFixture(t)

	t.Run("out-of-envelope handwritten file is rejected", func(t *testing.T) {
		fixture.Reset(t)
		writePressureFile(t, filepath.Join(fixture.Root, "internal", "common", "global_service.go"), "package common\n")
		report, err := ReconcileGitDelta(fixture.Root, fixture.Contract)
		if err != nil {
			t.Fatal(err)
		}
		assertViolation(t, report, "scope", "internal/common/global_service.go")
	})

	t.Run("generated artifact tampering reaches final drift gate", func(t *testing.T) {
		fixture.Reset(t)
		manifest := filepath.Join(fixture.Root, "contracts", "generated", "manifest.json")
		contents := readPressureFile(t, manifest)
		writePressureFile(t, manifest, contents+" \n")
		fast, err := ReconcileGitDelta(fixture.Root, fixture.Contract)
		if err != nil {
			t.Fatal(err)
		}
		if len(fast.Violations) != 0 {
			t.Fatalf("expected generated effect to remain eligible for final drift validation: %#v", fast.Violations)
		}
		attestation, _, err := VerifyChange(context.Background(), VerifyOptions{Root: fixture.Root, Contract: fixture.ContractPath, ProtoPaths: []string{fixture.ProtoPath}, SkipTests: true})
		if err != nil {
			t.Fatal(err)
		}
		if attestation.Conformant || gateStatus(attestation.Gates, "yunka-check") != "fail" {
			t.Fatalf("generated tamper escaped final verify: %#v", attestation)
		}
	})

	t.Run("tenant semantic drift is rejected", func(t *testing.T) {
		fixture.Reset(t)
		mutateRPCOption(t, fixture, "Suspend", "tenant_required: true", "tenant_required: false")
		generatePressureProject(t, fixture)
		assertSemanticViolation(t, fixture, SemanticTenant)
	})

	t.Run("permission semantic drift is rejected", func(t *testing.T) {
		fixture.Reset(t)
		mutateRPCOption(t, fixture, "Suspend", `permissions: "tenant.suspend"`, `permissions: "tenant.admin"`)
		generatePressureProject(t, fixture)
		assertSemanticViolation(t, fixture, SemanticPermission)
	})

	t.Run("transaction semantic drift is rejected", func(t *testing.T) {
		fixture.Reset(t)
		mutateRPCOption(t, fixture, "Suspend", "TRANSACTION_LOCAL", "TRANSACTION_NONE")
		generatePressureProject(t, fixture)
		assertSemanticViolation(t, fixture, SemanticTransaction)
	})

	t.Run("undeclared capability drift is rejected", func(t *testing.T) {
		fixture.Reset(t)
		contents := readPressureFile(t, fixture.protoFile())
		old := "    name: \"lifecycle\"\n"
		replacement := old + "    capabilities: { name: \"cache\" go_package: \"example.com/cache\" go_type: \"Cache\" }\n"
		if !strings.Contains(contents, old) {
			t.Fatalf("application declaration not found in pressure fixture:\n%s", contents)
		}
		writePressureFile(t, fixture.protoFile(), strings.Replace(contents, old, replacement, 1))
		generatePressureProject(t, fixture)
		assertSemanticViolation(t, fixture, SemanticCapabilities)
	})

	t.Run("unrelated operation semantic drift is rejected", func(t *testing.T) {
		fixture.Reset(t)
		mutateRPCOption(t, fixture, "Resume", "tenant_required: true", "tenant_required: false")
		generatePressureProject(t, fixture)
		report, err := ReconcileSemanticDelta(fixture.Root, fixture.Contract)
		if err != nil {
			t.Fatal(err)
		}
		for _, violation := range report.Violations {
			if violation.Category == "scope" && violation.Subject == "operation:tenant.resume" {
				return
			}
		}
		t.Fatalf("unrelated operation drift escaped: %#v", report)
	})

	t.Run("same-application arbitrary helper must not escape organization control", func(t *testing.T) {
		fixture.Reset(t)
		writePressureFile(t, filepath.Join(fixture.Root, "internal", "tenant", "application", "global_helper.go"), "package application\n\nfunc globalHelper() {}\n")
		attestation, _, err := VerifyChange(context.Background(), VerifyOptions{Root: fixture.Root, Contract: fixture.ContractPath, ProtoPaths: []string{fixture.ProtoPath}, SkipTests: true})
		if err != nil {
			t.Fatal(err)
		}
		if attestation.Conformant {
			t.Fatalf("AX7 pressure proved an architecture-placement escape: arbitrary handwritten file inside the broad application scope was accepted; preserve this failure before adding Architecture Delta/placement evidence")
		}
	})
}

func newPressureFixture(t *testing.T) pressureFixture {
	t.Helper()
	root := t.TempDir()
	writePressureFile(t, filepath.Join(root, ".gitignore"), ".yunka/\n")
	writePressureFile(t, filepath.Join(root, "go.mod"), "module example.com/ax7pressure\n\ngo 1.25.0\n")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	writePressureFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"), pressureDomainProto())

	if _, err := add.AddApplication(add.ApplicationOptions{Root: root, Key: "tenant/lifecycle"}); err != nil {
		t.Fatalf("add application: %v", err)
	}
	for _, operation := range []struct {
		id, useCase string
	}{
		{id: "tenant.suspend", useCase: "suspend_tenant"},
		{id: "tenant.resume", useCase: "resume_tenant"},
	} {
		if _, err := add.AddOperation(add.OperationOptions{
			Root: root, ApplicationKey: "tenant/lifecycle", OperationID: operation.id, UseCase: operation.useCase,
			Access: "protected", Permissions: []string{operation.id}, PermissionMode: "all", Tenant: "required",
			Authentication: []string{"jwt"}, Transaction: "local", Idempotency: "none", Composition: "local",
		}); err != nil {
			t.Fatalf("add operation %s: %v", operation.id, err)
		}
	}

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "../../.."))
	protoPath := filepath.Join(repositoryRoot, "contracts", "proto")
	fixture := pressureFixture{Root: root, ProtoPath: protoPath, ContractPath: DefaultChangeContractPath}
	generatePressureProject(t, fixture)
	gitPressure(t, root, "init")
	gitPressure(t, root, "config", "user.email", "ax7-pressure@example.invalid")
	gitPressure(t, root, "config", "user.name", "AX7 Pressure")
	gitPressure(t, root, "add", "-A")
	gitPressure(t, root, "commit", "-m", "AX7 pressure baseline")

	contractValue, projectRoot, err := BuildChangeContract(root, "tenant.suspend", IntentBoth, "HEAD", nil, nil, 3)
	if err != nil {
		t.Fatalf("build change contract: %v", err)
	}
	if projectRoot != root {
		t.Fatalf("project root=%q want=%q", projectRoot, root)
	}
	if _, err := WriteChangeContract(root, fixture.ContractPath, contractValue); err != nil {
		t.Fatalf("write change contract: %v", err)
	}
	fixture.Contract = contractValue
	return fixture
}

func (fixture pressureFixture) Reset(t *testing.T) {
	t.Helper()
	gitPressure(t, fixture.Root, "reset", "--hard", fixture.Contract.BaseSHA)
	gitPressure(t, fixture.Root, "clean", "-fd")
	if _, err := WriteChangeContract(fixture.Root, fixture.ContractPath, fixture.Contract); err != nil {
		t.Fatal(err)
	}
}

func (fixture pressureFixture) protoFile() string {
	return filepath.Join(fixture.Root, "contracts", "proto", "tenant.proto")
}

func generatePressureProject(t *testing.T, fixture pressureFixture) {
	t.Helper()
	if _, err := projectflow.Generate(context.Background(), projectflow.Options{Root: fixture.Root, ProtoPaths: []string{fixture.ProtoPath}}); err != nil {
		t.Fatalf("canonical generate: %v", err)
	}
}

func pressureDomainProto() string {
	return `syntax = "proto3";
package tenant.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/ax7pressure/contracts/tenant;tenantv1";
option (yunka.dsl.v1.domain) = { name: "tenant" version: "v1" };
`
}

func mutateRPCOption(t *testing.T, fixture pressureFixture, rpcName, old, replacement string) {
	t.Helper()
	contents := readPressureFile(t, fixture.protoFile())
	start := strings.Index(contents, "rpc "+rpcName+"(")
	if start < 0 {
		t.Fatalf("rpc %s not found:\n%s", rpcName, contents)
	}
	endRelative := strings.Index(contents[start:], "\n  }")
	if endRelative < 0 {
		t.Fatalf("rpc %s closing block not found", rpcName)
	}
	end := start + endRelative
	block := contents[start:end]
	if !strings.Contains(block, old) {
		t.Fatalf("rpc %s does not contain %q:\n%s", rpcName, old, block)
	}
	block = strings.Replace(block, old, replacement, 1)
	writePressureFile(t, fixture.protoFile(), contents[:start]+block+contents[end:])
}

func assertSemanticViolation(t *testing.T, fixture pressureFixture, category string) {
	t.Helper()
	report, err := ReconcileSemanticDelta(fixture.Root, fixture.Contract)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range report.Violations {
		if violation.Category == category && violation.Subject == "tenant.suspend" {
			return
		}
	}
	t.Fatalf("missing %s semantic violation: %#v", category, report)
}

func assertViolation(t *testing.T, report Reconciliation, kind, path string) {
	t.Helper()
	for _, violation := range report.Violations {
		if violation.Kind == kind && violation.Path == path {
			return
		}
	}
	t.Fatalf("missing violation %s %s: %#v", kind, path, report)
}

func gateStatus(gates []GateResult, name string) string {
	for _, gate := range gates {
		if gate.Name == name {
			return gate.Status
		}
	}
	return ""
}

func gitPressure(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return strings.TrimSpace(string(output))
}

func writePressureFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readPressureFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

var _ = fmt.Sprintf
