from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def edit(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if old not in text:
        raise SystemExit(f"expected fragment not found in {path}: {old[:120]!r}")
    target.write_text(text.replace(old, new, 1))


def append(path: str, text: str) -> None:
    target = ROOT / path
    current = target.read_text()
    if text.strip() not in current:
        target.write_text(current.rstrip() + "\n\n" + text.lstrip())


edit(
    "contracts/proto/yunka/dsl/v1/options.proto",
    "message OperationDeclaration {\n",
    "enum TransactionPolicy {\n"
    "  TRANSACTION_UNSPECIFIED = 0;\n"
    "  TRANSACTION_NONE = 1;\n"
    "  TRANSACTION_READ_ONLY = 2;\n"
    "  TRANSACTION_LOCAL = 3;\n"
    "}\n\n"
    "enum IdempotencyPolicy {\n"
    "  IDEMPOTENCY_UNSPECIFIED = 0;\n"
    "  IDEMPOTENCY_NONE = 1;\n"
    "  IDEMPOTENCY_REQUIRED = 2;\n"
    "}\n\n"
    "message ExecutionPolicy {\n"
    "  // Execution semantics are explicit contract facts. They must never be\n"
    "  // inferred from transport verbs or method names.\n"
    "  TransactionPolicy transaction = 1;\n"
    "  IdempotencyPolicy idempotency = 2;\n"
    "}\n\n"
    "message OperationDeclaration {\n",
)
edit(
    "contracts/proto/yunka/dsl/v1/options.proto",
    "  CompositionBoundary composition = 9;\n}",
    "  CompositionBoundary composition = 9;\n"
    "  ExecutionPolicy execution = 10;\n}",
)

edit(
    "pkg/contract/model.go",
    "type OperationDeclaration struct {\n",
    "type ExecutionPolicy struct {\n"
    "\tTransaction string `json:\"transaction,omitempty\"`\n"
    "\tIdempotency string `json:\"idempotency,omitempty\"`\n"
    "}\n\n"
    "type OperationDeclaration struct {\n",
)
edit(
    "pkg/contract/model.go",
    "\tComposition        string   `json:\"composition,omitempty\"`\n}",
    "\tComposition        string           `json:\"composition,omitempty\"`\n"
    "\tExecution          *ExecutionPolicy `json:\"execution,omitempty\"`\n}",
)
edit(
    "pkg/contract/model.go",
    "\t\t\t\tmethod.Operation.Composition = strings.TrimSpace(method.Operation.Composition)\n",
    "\t\t\t\tmethod.Operation.Composition = strings.TrimSpace(method.Operation.Composition)\n"
    "\t\t\t\tif method.Operation.Execution != nil {\n"
    "\t\t\t\t\tmethod.Operation.Execution.Transaction = strings.TrimSpace(method.Operation.Execution.Transaction)\n"
    "\t\t\t\t\tmethod.Operation.Execution.Idempotency = strings.TrimSpace(method.Operation.Execution.Idempotency)\n"
    "\t\t\t\t}\n",
)

edit(
    "pkg/contract/dsl_descriptor.go",
    "\t\tcase 9:\n\t\t\tif field.Type == 0 {\n\t\t\t\tresult.Composition = compositionBoundaryName(field.Varint)\n\t\t\t}\n",
    "\t\tcase 9:\n\t\t\tif field.Type == 0 {\n\t\t\t\tresult.Composition = compositionBoundaryName(field.Varint)\n\t\t\t}\n"
    "\t\tcase 10:\n"
    "\t\t\tif field.Type == 2 {\n"
    "\t\t\t\texecution, err := parseExecutionPolicy(field.Bytes)\n"
    "\t\t\t\tif err != nil {\n"
    "\t\t\t\t\treturn err\n"
    "\t\t\t\t}\n"
    "\t\t\t\tresult.Execution = execution\n"
    "\t\t\t}\n",
)
edit(
    "pkg/contract/dsl_descriptor.go",
    "func compositionBoundaryName(value uint64) string {\n",
    "func parseExecutionPolicy(data []byte) (*ExecutionPolicy, error) {\n"
    "\tif len(data) == 0 {\n"
    "\t\treturn &ExecutionPolicy{}, nil\n"
    "\t}\n"
    "\tresult := &ExecutionPolicy{}\n"
    "\tif err := scanWire(data, func(field wireField) error {\n"
    "\t\tswitch field.Number {\n"
    "\t\tcase 1:\n"
    "\t\t\tif field.Type == 0 {\n"
    "\t\t\t\tresult.Transaction = transactionPolicyName(field.Varint)\n"
    "\t\t\t}\n"
    "\t\tcase 2:\n"
    "\t\t\tif field.Type == 0 {\n"
    "\t\t\t\tresult.Idempotency = idempotencyPolicyName(field.Varint)\n"
    "\t\t\t}\n"
    "\t\t}\n"
    "\t\treturn nil\n"
    "\t}); err != nil {\n"
    "\t\treturn nil, err\n"
    "\t}\n"
    "\treturn result, nil\n"
    "}\n\n"
    "func transactionPolicyName(value uint64) string {\n"
    "\tswitch value {\n"
    "\tcase 1:\n\t\treturn \"none\"\n"
    "\tcase 2:\n\t\treturn \"read_only\"\n"
    "\tcase 3:\n\t\treturn \"local\"\n"
    "\tdefault:\n\t\treturn \"\"\n"
    "\t}\n"
    "}\n\n"
    "func idempotencyPolicyName(value uint64) string {\n"
    "\tswitch value {\n"
    "\tcase 1:\n\t\treturn \"none\"\n"
    "\tcase 2:\n\t\treturn \"required\"\n"
    "\tdefault:\n\t\treturn \"\"\n"
    "\t}\n"
    "}\n\n"
    "func compositionBoundaryName(value uint64) string {\n",
)

edit("pkg/operationplan/plan.go", "const SchemaVersion = 1", "const SchemaVersion = 2")
edit(
    "pkg/operationplan/plan.go",
    "\tSecurity            Security    `json:\"security\"`\n\tComposition         Composition `json:\"composition\"`\n",
    "\tSecurity            Security    `json:\"security\"`\n"
    "\tExecution           Execution   `json:\"execution\"`\n"
    "\tComposition         Composition `json:\"composition\"`\n",
)
edit(
    "pkg/operationplan/plan.go",
    "type Composition struct {\n",
    "type Execution struct {\n"
    "\tTransaction string `json:\"transaction\"`\n"
    "\tIdempotency string `json:\"idempotency\"`\n"
    "}\n\n"
    "type Composition struct {\n",
)
edit(
    "pkg/operationplan/plan.go",
    "\tif result.SchemaVersion == 0 {\n\t\tresult.SchemaVersion = SchemaVersion\n\t}\n",
    "\tif result.SchemaVersion == 0 || result.SchemaVersion == 1 {\n"
    "\t\tresult.SchemaVersion = SchemaVersion\n"
    "\t}\n",
)
edit(
    "pkg/operationplan/plan.go",
    "\t\titem.Security.PermissionMode = strings.TrimSpace(item.Security.PermissionMode)\n",
    "\t\titem.Security.PermissionMode = strings.TrimSpace(item.Security.PermissionMode)\n"
    "\t\titem.Execution.Transaction = strings.TrimSpace(item.Execution.Transaction)\n"
    "\t\tif item.Execution.Transaction == \"\" {\n"
    "\t\t\titem.Execution.Transaction = \"none\"\n"
    "\t\t}\n"
    "\t\titem.Execution.Idempotency = strings.TrimSpace(item.Execution.Idempotency)\n"
    "\t\tif item.Execution.Idempotency == \"\" {\n"
    "\t\t\titem.Execution.Idempotency = \"none\"\n"
    "\t\t}\n",
)
edit(
    "pkg/operationplan/plan.go",
    "\t\titem.Bindings.RPC = strings.TrimSpace(item.Bindings.RPC)\n\t\tfor i := range item.Bindings.HTTP {\n",
    "\t\titem.Bindings.RPC = strings.TrimSpace(item.Bindings.RPC)\n"
    "\t\titem.Bindings.HTTP = append([]HTTPBinding(nil), item.Bindings.HTTP...)\n"
    "\t\tfor i := range item.Bindings.HTTP {\n",
)
edit(
    "pkg/operationplan/plan.go",
    "\t\tswitch item.Composition.Boundary {\n",
    "\t\tswitch item.Execution.Transaction {\n"
    "\t\tcase \"none\", \"read_only\", \"local\":\n"
    "\t\tdefault:\n"
    "\t\t\treturn fmt.Errorf(\"operationplan: operation %s has invalid transaction policy %q\", item.OperationID, item.Execution.Transaction)\n"
    "\t\t}\n"
    "\t\tswitch item.Execution.Idempotency {\n"
    "\t\tcase \"none\", \"required\":\n"
    "\t\tdefault:\n"
    "\t\t\treturn fmt.Errorf(\"operationplan: operation %s has invalid idempotency policy %q\", item.OperationID, item.Execution.Idempotency)\n"
    "\t\t}\n"
    "\t\tswitch item.Composition.Boundary {\n",
)

edit(
    "pkg/contract/operation_plan.go",
    "\t\t\tplan := operationplan.Plan{\n",
    "\t\t\texecution := operationplan.Execution{Transaction: \"none\", Idempotency: \"none\"}\n"
    "\t\t\tif operation.Execution != nil {\n"
    "\t\t\t\tif value := strings.TrimSpace(operation.Execution.Transaction); value != \"\" {\n"
    "\t\t\t\t\texecution.Transaction = value\n"
    "\t\t\t\t}\n"
    "\t\t\t\tif value := strings.TrimSpace(operation.Execution.Idempotency); value != \"\" {\n"
    "\t\t\t\t\texecution.Idempotency = value\n"
    "\t\t\t\t}\n"
    "\t\t\t}\n"
    "\t\t\tplan := operationplan.Plan{\n",
)
edit(
    "pkg/contract/operation_plan.go",
    "\t\t\t\tSecurity: operationplan.Security{\n",
    "\t\t\t\tExecution: execution,\n"
    "\t\t\t\tSecurity: operationplan.Security{\n",
)

edit(
    "pkg/contract/c9_application_codegen.go",
    "\tfmt.Fprintf(b, \"Security: operationplan.Security{Public:%t, TenantRequired:%t, Authentication:\", plan.Security.Public, plan.Security.TenantRequired)\n",
    "\tfmt.Fprintf(b, \"Execution: operationplan.Execution{Transaction:%q, Idempotency:%q}, \", plan.Execution.Transaction, plan.Execution.Idempotency)\n"
    "\tfmt.Fprintf(b, \"Security: operationplan.Security{Public:%t, TenantRequired:%t, Authentication:\", plan.Security.Public, plan.Security.TenantRequired)\n",
)

append(
    "pkg/operationplan/plan_test.go",
    r'''
func TestNormalizeUpgradesV1ExecutionDefaultsWithoutMutatingInput(t *testing.T) {
	originalHTTP := []HTTPBinding{{Method: "post", Path: " /v1/a "}}
	input := Set{SchemaVersion: 1, Operations: []Plan{{
		OperationID: "a", Domain: "d", Application: "app", UseCase: "a",
		RequestType: "d.ARequest", ResponseType: "d.AResponse",
		Security: Security{PermissionMode: "all"},
		Bindings: Bindings{RPC: "/d.App/A", HTTP: originalHTTP},
	}}}
	normalized := Normalize(input)
	if normalized.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion=%d want %d", normalized.SchemaVersion, SchemaVersion)
	}
	if got := normalized.Operations[0].Execution; got.Transaction != "none" || got.Idempotency != "none" {
		t.Fatalf("execution=%#v", got)
	}
	if input.Operations[0].Bindings.HTTP[0].Method != "post" || input.Operations[0].Bindings.HTTP[0].Path != " /v1/a " {
		t.Fatalf("Normalize mutated caller input: %#v", input.Operations[0].Bindings.HTTP)
	}
	if normalized.Operations[0].Bindings.HTTP[0].Method != "POST" || normalized.Operations[0].Bindings.HTTP[0].Path != "/v1/a" {
		t.Fatalf("normalized HTTP=%#v", normalized.Operations[0].Bindings.HTTP)
	}
}

func TestValidateRejectsUnknownExecutionPolicy(t *testing.T) {
	plan := Plan{
		OperationID: "a", Domain: "d", Application: "app", UseCase: "a",
		RequestType: "d.ARequest", ResponseType: "d.AResponse",
		Security: Security{PermissionMode: "all"},
		Execution: Execution{Transaction: "magic", Idempotency: "none"},
		Bindings: Bindings{RPC: "/d.App/A"},
	}
	if err := Validate(Set{Operations: []Plan{plan}}); err == nil || !strings.Contains(err.Error(), "invalid transaction policy") {
		t.Fatalf("transaction policy err=%v", err)
	}
	plan.Execution = Execution{Transaction: "none", Idempotency: "magic"}
	if err := Validate(Set{Operations: []Plan{plan}}); err == nil || !strings.Contains(err.Error(), "invalid idempotency policy") {
		t.Fatalf("idempotency policy err=%v", err)
	}
}
''',
)

append(
    "pkg/contract/operation_plan_test.go",
    r'''
func TestCompileOperationPlansCarriesExplicitExecutionPolicy(t *testing.T) {
	manifest := Manifest{Services: []Service{{
		Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device",
		Application: &ApplicationDeclaration{Name: "management"},
		Methods: []Method{{
			Name: "UpdateDevice", FullName: "device.v1.DeviceApplication.UpdateDevice",
			Request: "device.v1.UpdateDeviceRequest", Response: "device.v1.DeviceDTO",
			Operation: &OperationDeclaration{
				ID: "device.update", UseCase: "update_device", PermissionMode: "all",
				Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"},
			},
		}},
	}}}
	set, err := CompileOperationPlans(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Operations) != 1 {
		t.Fatalf("operations=%d", len(set.Operations))
	}
	got := set.Operations[0].Execution
	if got.Transaction != "local" || got.Idempotency != "required" {
		t.Fatalf("execution=%#v", got)
	}
}
''',
)

print("C9.7.1-2 source transformations staged")
