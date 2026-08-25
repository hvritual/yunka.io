package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainNewScansMultipleSnakeCasePOs(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct {
	Serial string `+"`gorm:\"column:serial;type:varchar(64)\"`"+`
	Enabled bool `+"`gorm:\"column:enabled\"`"+`
	SearchHash string `+"`gorm:\"column:search_hash\" yunka:\"-\"`"+`
}
`)
	writeTestPO(t, persistence, "device_group.go", `package persistence

type DeviceGroupPO struct { Name string `+"`gorm:\"column:name;type:varchar(128)\"`"+` }
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal"), NoRPC: true}); err != nil {
		t.Fatal(err)
	}
	spec, err := readManifest(filepath.Join(root, "internal", "device"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Version != SpecVersion || len(spec.Objects) != 2 {
		t.Fatalf("unexpected manifest: %+v", spec)
	}
	if spec.Objects[0].Name != "coffee_machine" || spec.Objects[0].File != "coffee_machine.go" || spec.Objects[0].TableName != "yk_device_coffee_machine" {
		t.Fatalf("unexpected first object: %+v", spec.Objects[0])
	}
	if len(spec.Objects[0].Fields) != 2 {
		t.Fatalf("persistence-only field leaked into API contract: %+v", spec.Objects[0].Fields)
	}
	if spec.Objects[0].Fields[0].ProtoNumber <= 0 || spec.Objects[0].Fields[1].ProtoNumber <= spec.Objects[0].Fields[0].ProtoNumber {
		t.Fatalf("proto numbers not stable/ordered: %+v", spec.Objects[0].Fields)
	}
	if err := Check(filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
}

func TestRegeneratePreservesProtoNumberAndReservesRemovedField(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	path := filepath.Join(persistence, "coffee_machine.go")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct {
	Serial string
	Enabled bool
}
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal"), NoRPC: true}); err != nil {
		t.Fatal(err)
	}
	before, _ := readManifest(filepath.Join(root, "internal", "device"))
	var serialNumber, enabledNumber int
	for _, field := range before.Objects[0].Fields {
		if field.Name == "serial" {
			serialNumber = field.ProtoNumber
		}
		if field.Name == "enabled" {
			enabledNumber = field.ProtoNumber
		}
	}
	contents := `package persistence

type CoffeeMachinePO struct {
	Serial string
	FirmwareVersion string `+"`gorm:\"column:firmware_version\"`"+`
}
`
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Regenerate(filepath.Join(root, "internal", "device")); err != nil {
		t.Fatal(err)
	}
	after, _ := readManifest(filepath.Join(root, "internal", "device"))
	var gotSerial, gotFirmware int
	for _, field := range after.Objects[0].Fields {
		if field.Name == "serial" {
			gotSerial = field.ProtoNumber
		}
		if field.Name == "firmware_version" {
			gotFirmware = field.ProtoNumber
		}
	}
	if gotSerial != serialNumber {
		t.Fatalf("serial proto number changed: before=%d after=%d", serialNumber, gotSerial)
	}
	if gotFirmware <= enabledNumber {
		t.Fatalf("new field reused historical number: firmware=%d removed=%d", gotFirmware, enabledNumber)
	}
	if !containsInt(after.Objects[0].ReservedProtoNumbers, enabledNumber) || !containsString(after.Objects[0].ReservedProtoNames, "enabled") {
		t.Fatalf("removed field was not reserved: %+v", after.Objects[0])
	}
}

func TestPOFilenameMustMatchSnakeCaseType(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "machine.go", `package persistence

type CoffeeMachinePO struct { Serial string }
`)
	err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal"), NoRPC: true})
	if err == nil || !strings.Contains(err.Error(), "coffee_machine.go") {
		t.Fatalf("expected snake_case filename error, got %v", err)
	}
}

func newPOFirstTestProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/biz\n\ngo 1.25.0\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeTestPO(t *testing.T, root, name, contents string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
}

func containsInt(values []int, value int) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
func containsString(values []string, value string) bool {
	for _, current := range values {
		if current == value {
			return true
		}
	}
	return false
}
