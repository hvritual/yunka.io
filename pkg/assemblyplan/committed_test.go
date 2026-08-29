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
