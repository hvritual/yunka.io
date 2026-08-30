package assemblyplan

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadSecondWriteIsZeroDrift(t *testing.T) {
	plan, err := Compile(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "assembly-plan.json")
	if err := Save(path, plan); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("second AssemblyPlan write drifted:\nfirst=%s\nsecond=%s", first, second)
	}
	want, err := CanonicalJSON(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, want) {
		t.Fatalf("saved AssemblyPlan is not canonical:\nwant=%s\ngot=%s", want, first)
	}
}
