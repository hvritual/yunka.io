package projectflow

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

func TestDiagnoseStableFailureClasses(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		name  string
		kind  FailureKind
		code  string
		stage string
	}{
		{name: "project", kind: FailureProject, code: diagnostic.CodeProjectResolve, stage: "project"},
		{name: "contract", kind: FailureContract, code: diagnostic.CodeContractFailure, stage: "contract"},
		{name: "contract drift", kind: FailureContractDrift, code: diagnostic.CodeContractDrift, stage: "contract"},
		{name: "module", kind: FailureModule, code: diagnostic.CodeModuleFailure, stage: "module"},
		{name: "assembly", kind: FailureAssembly, code: diagnostic.CodeAssemblyFailure, stage: "assembly"},
		{name: "assembly drift", kind: FailureAssemblyDrift, code: diagnostic.CodeAssemblyDrift, stage: "assembly"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := wrapFailure(test.kind, root, "contracts/generated/manifest.json", errors.New("failure at "+filepath.Join(root, "contracts", "generated")))
			item := Diagnose(err)
			if item.Code != test.code || item.Stage != test.stage {
				t.Fatalf("diagnostic=%#v", item)
			}
			definition := diagnostic.MustDefinition(test.code)
			if item.Summary != definition.Meaning || !reflect.DeepEqual(item.Actions, definition.Actions) {
				t.Fatalf("diagnostic identity drifted from catalog: item=%#v definition=%#v", item, definition)
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
	if item.Code != diagnostic.CodeProjectResolve {
		t.Fatalf("code=%q detail=%q", item.Code, item.Detail)
	}
}
