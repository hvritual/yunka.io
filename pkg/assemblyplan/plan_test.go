package assemblyplan

import (
	"bytes"
	"strings"
	"testing"
)

func reused(source, ref string) Evidence {
	return Evidence{Ownership: OwnershipReused, Source: source, Ref: ref}
}

func sampleInput() Input {
	return Input{
		Identity: "root",
		Applications: []ApplicationInput{
			{ID: "device/transfer", Domain: "device", Name: "transfer", DependsOn: []string{"site/query"}, Evidence: reused("manifest", "applications/device/transfer")},
			{ID: "site/query", Domain: "site", Name: "query", Evidence: reused("manifest", "applications/site/query")},
		},
		Operations: []OperationInput{
			{ID: "device.transfer", Application: "device/transfer", RequiresOperations: []string{"site.validate"}, Bindings: []BindingInput{{Transport: "http", Index: 0, Evidence: reused("operation-plan", "operations/device.transfer/bindings/http/0")}}, Evidence: reused("operation-plan", "operations/device.transfer")},
			{ID: "site.validate", Application: "site/query", Evidence: reused("operation-plan", "operations/site.validate")},
		},
		Modules: []ModuleInput{
			{Name: "device", DependsOn: []string{"site"}, Requirements: ModuleRequirements{ConfigKey: "device", Logger: true, Databases: []string{"primary"}, EventBus: true}, Evidence: reused("module-catalog", "modules/device")},
			{Name: "site", Requirements: ModuleRequirements{RPC: []string{"inventory"}}, Evidence: reused("module-catalog", "modules/site")},
		},
	}
}

func TestCompileIsDeterministicAndCarriesClosure(t *testing.T) {
	first, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	input := sampleInput()
	input.Applications[0], input.Applications[1] = input.Applications[1], input.Applications[0]
	input.Operations[0], input.Operations[1] = input.Operations[1], input.Operations[0]
	input.Modules[0], input.Modules[1] = input.Modules[1], input.Modules[0]
	second, err := Compile(input)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := CanonicalJSON(second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatalf("canonical assembly plan drifted:\nfirst=%s\nsecond=%s", firstBytes, secondBytes)
	}
	firstDigest, err := Digest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("digest drift: %s != %s", firstDigest, secondDigest)
	}
	if len(first.ApplicationDependencyClosure) != 1 || first.ApplicationDependencyClosure[0].From != "device/transfer" || first.ApplicationDependencyClosure[0].To != "site/query" {
		t.Fatalf("unexpected application closure: %#v", first.ApplicationDependencyClosure)
	}
	if got := strings.Join(first.Targets[0].InternalOperations, ","); got != "site.validate" {
		t.Fatalf("internal operation exposure drift: %s", got)
	}
	if got := strings.Join(first.Targets[0].ExternalOperations, ","); got != "device.transfer" {
		t.Fatalf("external operation exposure drift: %s", got)
	}
}

func TestValidateFailsClosedForUnknownApplication(t *testing.T) {
	input := sampleInput()
	input.Operations[0].Application = "missing/app"
	_, err := Compile(input)
	if err == nil || !strings.Contains(err.Error(), "unknown application") {
		t.Fatalf("expected unknown application failure, got %v", err)
	}
}

func TestValidateFailsClosedForApplicationCycle(t *testing.T) {
	input := sampleInput()
	input.Applications[1].DependsOn = []string{"device/transfer"}
	_, err := Compile(input)
	if err == nil || !strings.Contains(err.Error(), "application dependency cycle") {
		t.Fatalf("expected application cycle failure, got %v", err)
	}
}

func TestValidateFailsClosedForModuleCycle(t *testing.T) {
	input := sampleInput()
	input.Modules[1].DependsOn = []string{"device"}
	_, err := Compile(input)
	if err == nil || !strings.Contains(err.Error(), "module dependency cycle") {
		t.Fatalf("expected module cycle failure, got %v", err)
	}
}

func TestValidateFailsClosedWhenRequirementInventoryIsMissing(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	plan.Requirements = plan.Requirements[1:]
	if err := Validate(plan); err == nil || !strings.Contains(err.Error(), "requirement inventory") {
		t.Fatalf("expected incomplete requirement failure, got %v", err)
	}
}

func TestInternalOperationCannotGainInferredTransport(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range plan.Operations {
		if operation.ID == "site.validate" && len(operation.Bindings) != 0 {
			t.Fatalf("internal operation gained transport bindings: %#v", operation.Bindings)
		}
	}
}

func TestLoadBytesRejectsStaleDerivedTarget(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	plan.Targets[0].InternalOperations = nil
	if err := Validate(plan); err == nil || !strings.Contains(err.Error(), "bootstrap target") {
		t.Fatalf("expected stale target failure, got %v", err)
	}
}
