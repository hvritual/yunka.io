package deviceops

import (
	"os"
	"testing"
)

func TestModulePackageIsCompositionOnly(t *testing.T) {
	allowed := map[string]struct{}{
		"config.go":               {},
		"dependencies.go":         {},
		"module.go":               {},
		"zz_yunka_module_gen.go":  {},
		"module_boundary_test.go": {},
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || len(entry.Name()) < 3 || entry.Name()[len(entry.Name())-3:] != ".go" {
			continue
		}
		if _, ok := allowed[entry.Name()]; !ok {
			t.Fatalf("modules/deviceops is composition-only; move %s under internal/<domain>", entry.Name())
		}
	}
}
