package assemblyplan

import (
	"strings"
	"testing"
)

func TestCanonicalJSONRejectsRuntimeLocalEvidence(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	plan.Applications[0].Evidence.Ownership = OwnershipRuntimeLocal
	_, err = CanonicalJSON(plan)
	if err == nil || !strings.Contains(err.Error(), "runtime-local evidence") {
		t.Fatalf("expected committed runtime-local evidence failure, got %v", err)
	}
}

func TestLoadBytesRejectsRuntimeLocalEvidence(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	data, err := CanonicalJSON(plan)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(data), `"ownership": "reused"`, `"ownership": "runtime_local"`, 1)
	if mutated == string(data) {
		t.Fatal("expected evidence ownership fixture to be replaced")
	}
	_, err = LoadBytes([]byte(mutated))
	if err == nil || !strings.Contains(err.Error(), "runtime-local evidence") {
		t.Fatalf("expected load-time runtime-local evidence failure, got %v", err)
	}
}

func TestCanonicalJSONRejectsStaleBindingEvidence(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	for index := range plan.Operations {
		if plan.Operations[index].ID != "device.transfer" {
			continue
		}
		plan.Operations[index].Bindings[0].Evidence.Ref = "operations/other/bindings/http/0"
	}
	_, err = CanonicalJSON(plan)
	if err == nil || !strings.Contains(err.Error(), "stale or inconsistent canonical evidence") {
		t.Fatalf("expected stale binding evidence failure, got %v", err)
	}
}

func TestCanonicalJSONRejectsStaleDependencyEvidence(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	plan.ApplicationDependencies[0].Evidence.Ref = "applications/device/transfer/requires/other/app"
	_, err = CanonicalJSON(plan)
	if err == nil || !strings.Contains(err.Error(), "stale or inconsistent canonical evidence") {
		t.Fatalf("expected stale dependency evidence failure, got %v", err)
	}
}
