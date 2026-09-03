package change

import "testing"

func TestGeneratedImpactScopeDoesNotAuthorizeHandwrittenFile(t *testing.T) {
	contractValue := ChangeContract{
		SchemaVersion:  ChangeContractSchemaVersion,
		GeneratedScopes: []string{"internal"},
		EditableScopes:  []string{"internal/tenant/application"},
	}

	change, violation, err := reconcileFile(t.TempDir(), contractValue, FileChange{Status: "A", Path: "internal/common/global_service.go"})
	if err != nil {
		t.Fatal(err)
	}
	if violation == nil || violation.Kind != "scope" || change.Class != "outside" {
		t.Fatalf("change=%#v violation=%#v", change, violation)
	}
}
