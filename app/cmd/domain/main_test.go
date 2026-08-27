package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDomainNewScansMultipleSnakeCasePOsIntoV3(t *testing.T) {
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
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}
	domainRoot := filepath.Join(root, "internal", "device")
	spec, err := readManifest(domainRoot)
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
		t.Fatalf("persistence-only field leaked into generated entity contract: %+v", spec.Objects[0].Fields)
	}
	raw, err := os.ReadFile(filepath.Join(domainRoot, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"rest"`, `"rpc"`, "protoNumber", "reservedProto"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("V3 manifest retained transport/protobuf metadata %q:\n%s", forbidden, raw)
		}
	}
	for _, forbiddenPath := range []string{"application", "transport", "wire"} {
		if _, err := os.Stat(filepath.Join(domainRoot, forbiddenPath)); !os.IsNotExist(err) {
			t.Fatalf("domain compiler generated forbidden %s path", forbiddenPath)
		}
	}
	if err := Check(filepath.Join(root, "internal")); err != nil {
		t.Fatal(err)
	}
}

func TestRegenerateTracksPOFieldsWithoutProtobufState(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	path := filepath.Join(persistence, "coffee_machine.go")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct {
	Serial string
	Enabled bool
}
`)
	if err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")}); err != nil {
		t.Fatal(err)
	}
	contents := `package persistence

type CoffeeMachinePO struct {
	Serial string
	FirmwareVersion string ` + "`gorm:\"column:firmware_version\"`" + `
}
`
	if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := Regenerate(filepath.Join(root, "internal", "device")); err != nil {
		t.Fatal(err)
	}
	after, err := readManifest(filepath.Join(root, "internal", "device"))
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{after.Objects[0].Fields[0].Name, after.Objects[0].Fields[1].Name}; got[0] != "firmware_version" || got[1] != "serial" {
		t.Fatalf("unexpected V3 fields: %v", got)
	}
	raw, _ := os.ReadFile(filepath.Join(root, "internal", "device", ManifestName))
	if strings.Contains(string(raw), "proto") || strings.Contains(string(raw), "reserved") {
		t.Fatalf("protobuf state leaked into V3 domain manifest:\n%s", raw)
	}
}

func TestRegenerateUpgradesV2AndRemovesGeneratedFullStack(t *testing.T) {
	root := newPOFirstTestProject(t)
	domainRoot := filepath.Join(root, "internal", "device")
	persistence := filepath.Join(domainRoot, "infrastructure", "persistence")
	writeTestPO(t, persistence, "coffee_machine.go", `package persistence

type CoffeeMachinePO struct { Serial string }
`)
	legacy := `{
  "version": 2,
  "domain": "device",
  "tablePrefix": "yk",
  "tenantScoped": true,
  "objects": [{
    "name": "coffee_machine",
    "file": "coffee_machine.go",
    "tableName": "yk_device_coffee_machine",
    "fields": [{"name":"serial","type":"string","column":"serial","protoNumber":1,"poOwned":true}],
    "reservedProtoNumbers": [2],
    "reservedProtoNames": ["enabled"]
  }],
  "rest": {"enabled": true, "prefix": "/v1"},
  "rpc": {"enabled": true}
}`
	if err := os.MkdirAll(domainRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domainRoot, ManifestName), []byte(legacy), 0o640); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		"application/zz_yunka_service_gen.go",
		"transport/rest/zz_yunka_rest_gen.go",
		"transport/rpc/domain.proto",
		"transport/rpc/zz_yunka_grpc_bridge_gen.go",
		"wire/zz_yunka_wiring_gen.go",
	} {
		path := filepath.Join(domainRoot, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(generatedDomainMarker+"\nlegacy\n"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := Regenerate(domainRoot); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(domainRoot, ManifestName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"version": 3`) {
		t.Fatalf("manifest not upgraded:\n%s", raw)
	}
	for _, forbidden := range []string{`"rest"`, `"rpc"`, "protoNumber", "reservedProto"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("legacy key %q remains:\n%s", forbidden, raw)
		}
	}
	for _, relative := range []string{"application/zz_yunka_service_gen.go", "transport/rest/zz_yunka_rest_gen.go", "transport/rpc/domain.proto", "transport/rpc/zz_yunka_grpc_bridge_gen.go", "wire/zz_yunka_wiring_gen.go"} {
		if _, err := os.Stat(filepath.Join(domainRoot, filepath.FromSlash(relative))); !os.IsNotExist(err) {
			t.Fatalf("stale generated full-stack file remains: %s", relative)
		}
	}
}

func TestPOFilenameMustMatchSnakeCaseType(t *testing.T) {
	root := newPOFirstTestProject(t)
	persistence := filepath.Join(root, "internal", "device", "infrastructure", "persistence")
	writeTestPO(t, persistence, "machine.go", `package persistence

type CoffeeMachinePO struct { Serial string }
`)
	err := Generate(Options{Name: "device", Root: filepath.Join(root, "internal")})
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
