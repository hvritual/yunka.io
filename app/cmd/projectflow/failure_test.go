package projectflow

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDiagnoseStableFailureClasses(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name string
		kind FailureKind
		code string
		stage string
	}{
		{name: "project", kind: FailureProject, code: "YUNKA-DX-PROJECT-001", stage: "project"},
		{name: "contract", kind: FailureContract, code: "YUNKA-DX-CONTRACT-001", stage: "contract"},
		{name: "contract drift", kind: FailureContractDrift, code: "YUNKA-DX-CONTRACT-002", stage: "contract"},
		{name: "module", kind: FailureModule, code: "YUNKA-DX-MODULE-001", stage: "module"},
		{name: "assembly", kind: FailureAssembly, code: "YUNKA-DX-ASSEMBLY-001", stage: "assembly"},
		{name: "assembly drift", kind: FailureAssemblyDrift, code: "YUNKA-DX-ASSEMBLY-002", stage: "assembly"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := wrapFailure(test.kind, root, "contracts/generated/manifest.json", errors.New("failure at "+filepath.Join(root, "contracts", "generated")))
			item := Diagnose(err)
			if item.Code != test.code || item.Stage != test.stage {
				t.Fatalf("diagnostic=%#v", item)
			}
			if item.Detail == "" || item.Detail == "failure at "+filepath.Join(root, "contracts", "generated") {
				t.Fatalf("detail was not sanitized: %q", item.Detail)
			}
			if item.Location == nil || item.Location.Path != "contracts/generated/manifest.json" {
				t.Fatalf("location=%#v", item.Location)
			}
		})
	}
}

func TestCheckMissingProjectProducesTypedProjectFailure(t *testing.T) {
	_, err := Check(nil, Options{Root: t.TempDir()})
	if err == nil {
		t.Fatal("expected check failure")
	}
	item := Diagnose(err)
	if item.Code != "YUNKA-DX-PROJECT-001" {
		t.Fatalf("code=%q detail=%q", item.Code, item.Detail)
	}
}
