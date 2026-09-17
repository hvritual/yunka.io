package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/auditcore"
)

func TestBuildWithBaseBlocksDirectOperationGrowthEvenWithStaleGeneratedManifest(t *testing.T) {
	root := t.TempDir()
	writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
	support, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "proto", "yunka", "dsl", "v1", "options.proto"))
	if err != nil {
		t.Fatal(err)
	}
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "yunka", "dsl", "v1", "options.proto"), string(support))
	baselineProto := boundaryAuditProto(false)
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "sales.proto"), baselineProto)
	manifest := contract.Manifest{SchemaVersion: contract.ManifestVersion, Files: []contract.File{{Name: "sales.proto", Domain: &contract.DomainDeclaration{Name: "sales"}}}}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(data, '\n')))
	writeAuditProjectFile(t, filepath.Join(root, "internal", "sales", "application", "service.go"), "// Package application owns the fixture.\npackage application\n")
	gitAudit(t, root, "init")
	gitAudit(t, root, "config", "user.email", "audit@example.invalid")
	gitAudit(t, root, "config", "user.name", "Yunka Audit Test")
	gitAudit(t, root, "add", ".")
	gitAudit(t, root, "commit", "-m", "baseline")

	// Bypass `yunka add operation`: change canonical source only and deliberately
	// leave generated manifest stale. Base-aware audit must still see the growth.
	writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "sales.proto"), boundaryAuditProto(true))
	report, err := BuildWithBase(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, finding := range report.Debt.New {
		if finding.Rule == RuleOperationGrowthBoundary && finding.Blocking && finding.Subject == "orders.list" {
			found = true
		}
	}
	if !found {
		t.Fatalf("new debt=%#v", report.Debt.New)
	}
	if len(auditcore.BlockingNewFindings(report)) == 0 {
		t.Fatal("direct growth was not blocking")
	}
}

func boundaryAuditProto(withGrowth bool) string {
	extra := ""
	if withGrowth {
		extra = `
  rpc List(ReadRequest) returns (ReadResponse) {
    option (yunka.dsl.v1.operation) = {
      id: "orders.list" use_case: "list_orders" public: true
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
    };
  }
`
	}
	return `syntax = "proto3";
package sales.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/demo/contracts/sales;salesv1";
option (yunka.dsl.v1.domain) = { name: "sales" version: "v1" };
message ReadRequest { option (yunka.dsl.v1.dto) = { kind: DTO_INPUT }; }
message ReadResponse { option (yunka.dsl.v1.dto) = { kind: DTO_OUTPUT }; }
service Orders {
  option (yunka.dsl.v1.application) = { name: "orders" };
  rpc Read(ReadRequest) returns (ReadResponse) {
    option (yunka.dsl.v1.operation) = {
      id: "orders.read" use_case: "read_order" public: true
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
      boundary: { context: "sales.orders" aggregate: "order" }
    };
  }
` + extra + "}\n"
}

var _ = strings.Contains
