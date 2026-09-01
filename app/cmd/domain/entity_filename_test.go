package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedDomainModelsUseEntityFilenames(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct { Serial string }
`)
	writeTestPO(t, persistence, "device_group.go", `package persistence

type DeviceGroupPO struct { Name string }
`)

	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}

	domainRoot := filepath.Join(root, "internal", "device", "domain")
	for entity, filename := range map[string]string{
		"CoffeeMachine": "coffee_machine.go",
		"DeviceGroup":   "device_group.go",
	} {
		contents, err := os.ReadFile(filepath.Join(domainRoot, filename))
		if err != nil {
			t.Fatalf("read generated entity file %s: %v", filename, err)
		}
		if !strings.Contains(string(contents), "type "+entity+" struct") {
			t.Fatalf("generated file %s does not contain entity %s:\n%s", filename, entity, contents)
		}
		for sibling := range map[string]struct{}{"CoffeeMachine": {}, "DeviceGroup": {}} {
			if sibling != entity && strings.Contains(string(contents), "type "+sibling+" struct") {
				t.Fatalf("generated file %s unexpectedly contains sibling entity %s", filename, sibling)
			}
		}
	}

	for _, legacy := range []string{
		"zz_yunka_coffee_machine_entity_gen.go",
		"zz_yunka_device_group_entity_gen.go",
	} {
		if _, err := os.Stat(filepath.Join(domainRoot, legacy)); !os.IsNotExist(err) {
			t.Fatalf("legacy aggregate-style entity filename still generated: %s", legacy)
		}
	}
}

func TestRegenerateRemovesLegacyEntityFilename(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct { Serial string }
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}

	domainRoot := filepath.Join(root, "internal", "device")
	legacy := filepath.Join(domainRoot, "domain", "zz_yunka_coffee_machine_entity_gen.go")
	if err := os.WriteFile(legacy, []byte(generatedDomainMarker+"\n\npackage domain\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := Regenerate(domainRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("legacy generated entity filename was not removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(domainRoot, "domain", "coffee_machine.go")); err != nil {
		t.Fatalf("canonical entity file missing after regenerate: %v", err)
	}
}
