package dev

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestC115ADevManifestResolutionUsesProfileWhenConfigIsImplicit(t *testing.T) {
	root := t.TempDir()
	writeDevTestFile(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "contracts/proto", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal"},
    "dev": {"manifest": "dev/local.json"}
  }
}
`)

	got, err := resolveDevManifestPath(root, defaultDevManifest, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "dev/local.json" {
		t.Fatalf("manifest=%q want dev/local.json", got)
	}
}

func TestC115AExplicitConfigWinsEvenWhenProjectProfileIsInvalid(t *testing.T) {
	root := t.TempDir()
	writeDevTestFile(t, filepath.Join(root, ".yunka", "project.json"), "{broken")

	got, err := resolveDevManifestPath(root, "custom/dev.json", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom/dev.json" {
		t.Fatalf("manifest=%q want custom/dev.json", got)
	}
}

func TestC115AInvalidPresentProfileFailsClosedForImplicitConfig(t *testing.T) {
	root := t.TempDir()
	writeDevTestFile(t, filepath.Join(root, ".yunka", "project.json"), "{broken")

	_, err := resolveDevManifestPath(root, defaultDevManifest, false)
	if err == nil || !strings.Contains(err.Error(), "dev project profile") {
		t.Fatalf("expected project-profile failure, got %v", err)
	}
}

func TestC115AAbsentProfileKeepsConventionalManifest(t *testing.T) {
	root := t.TempDir()
	got, err := resolveDevManifestPath(root, defaultDevManifest, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != defaultDevManifest {
		t.Fatalf("manifest=%q want %q", got, defaultDevManifest)
	}
}

func TestC115ALegacyProfileUsesInMemoryDefaultWithoutMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".yunka", "project.json")
	original := []byte("{\n  \"version\": 1,\n  \"database\": {\"tablePrefix\": \"yk\"}\n}\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveDevManifestPath(root, defaultDevManifest, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != defaultDevManifest {
		t.Fatalf("manifest=%q want %q", got, defaultDevManifest)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("legacy project profile was mutated:\nbefore=%s\nafter=%s", original, after)
	}
}

func writeDevTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}
