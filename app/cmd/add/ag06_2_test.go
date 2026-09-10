package add

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/projectflow"
)

func TestAG062AddOperationPreservesExistingSealedStarter(t *testing.T) {
	root := scaffoldProject(t, map[string]string{
		"contracts/proto/tenant.proto": typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication"),
		".yunka/source-policy.json":    "{}\n",
	})
	project := projectflow.ProjectDescriptor{Root: root, GeneratedGoRoot: "internal"}
	layout, err := projectflow.DescribeImplementationLayout(project, "tenant", "lifecycle", "Suspend")
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, filepath.FromSlash(layout.Build)), "package owner\n")
	mustWriteFile(t, filepath.Join(root, filepath.FromSlash(layout.TypePolicy)), "{}\n")

	report, err := AddOperation(OperationOptions{
		Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.suspend", UseCase: "suspend_tenant",
		Access: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Mutations) != 1 || report.Mutations[0].Path != "contracts/proto/tenant.proto" {
		t.Fatalf("sealed mutations=%#v", report.Mutations)
	}
	legacy := filepath.Join(root, "internal", "tenant", "application", "tenant_suspend.go")
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy landing unexpectedly created: %v", err)
	}
	joined := ""
	for _, note := range report.Notes {
		joined += note + "\n"
	}
	if !strings.Contains(joined, "sealed-v1") {
		t.Fatalf("sealed note missing: %#v", report.Notes)
	}
	commands := ""
	for _, next := range report.NextActions {
		commands += next.Command + "\n"
	}
	if !strings.Contains(commands, "yunka audit types") || !strings.Contains(commands, "yunka audit source --root . --format agent-json") {
		t.Fatalf("governance next actions=%#v", report.NextActions)
	}
}

func TestAG062AddOperationRejectsPartialSealedStarterBeforeMutation(t *testing.T) {
	original := typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")
	root := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": original})
	project := projectflow.ProjectDescriptor{Root: root, GeneratedGoRoot: "internal"}
	layout, err := projectflow.DescribeImplementationLayout(project, "tenant", "lifecycle", "Suspend")
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(root, filepath.FromSlash(layout.Build)), "package owner\n")
	_, err = AddOperation(OperationOptions{
		Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.suspend", UseCase: "suspend_tenant",
		Access: "public", Tenant: "optional", Transaction: "none", Idempotency: "none", Composition: "none",
	})
	if err == nil {
		t.Fatal("partial sealed starter accepted")
	}
	if got := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto")); got != original {
		t.Fatal("contract mutated before partial starter rejection")
	}
}
