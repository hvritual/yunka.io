package doctor

import (
	"encoding/json"
	"strings"
	"testing"

	"yunka.io/pkg/devruntime"
	"yunka.io/pkg/diagnostic"
)

func TestDoctorMappingsCoverCurrentProbeNames(t *testing.T) {
	names := []string{
		"workspace.root",
		"workspace.go_work",
		"toolchain.lock",
		"tool.go",
		"tool.protoc",
		"tool.protoc-gen-go",
		"tool.protoc-gen-go-grpc",
		"tool.gcc",
		"tool.git",
		"contract.manifest",
		"application_graph.contract",
		"git.status",
		"dev.manifest",
	}
	for _, name := range names {
		mapping, ok := doctorMappings[name]
		if !ok {
			t.Fatalf("missing mapping for %s", name)
		}
		if mapping.Code == "" || mapping.Stage == "" {
			t.Fatalf("incomplete mapping for %s: %#v", name, mapping)
		}
	}
}

func TestDoctorStrictChangesExitTruthWithoutChangingWarningSeverity(t *testing.T) {
	root := t.TempDir()
	report := devruntime.DoctorReport{
		Root: root,
		Checks: []devruntime.Check{
			{Name: "workspace.root", Status: devruntime.CheckPass, Detail: root},
			{Name: "git.status", Status: devruntime.CheckWarn, Detail: "working tree has local changes", Action: "review git status before broad generation or cleanup commands"},
		},
	}

	nonStrictBytes, err := renderDoctorJSON(report, false)
	if err != nil {
		t.Fatal(err)
	}
	strictBytes, err := renderDoctorJSON(report, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(strictBytes), root) || strings.Contains(string(nonStrictBytes), root) {
		t.Fatalf("doctor json leaked absolute root: %s", strictBytes)
	}

	var nonStrict doctorEnvelope
	if err := json.Unmarshal(nonStrictBytes, &nonStrict); err != nil {
		t.Fatal(err)
	}
	var strict doctorEnvelope
	if err := json.Unmarshal(strictBytes, &strict); err != nil {
		t.Fatal(err)
	}
	if !nonStrict.OK || strict.OK {
		t.Fatalf("nonStrict.OK=%v strict.OK=%v", nonStrict.OK, strict.OK)
	}
	if len(strict.Diagnostics) != 2 {
		t.Fatalf("diagnostics=%#v", strict.Diagnostics)
	}
	var warning *diagnostic.Diagnostic
	for index := range strict.Diagnostics {
		if strict.Diagnostics[index].Code == "YUNKA-DX-DEV-101" {
			warning = &strict.Diagnostics[index]
		}
	}
	if warning == nil || warning.Severity != diagnostic.SeverityWarning {
		t.Fatalf("strict mode changed warning fact: %#v", warning)
	}
}

func TestAdaptDoctorReportPreservesRemediationAsNonExecutingAction(t *testing.T) {
	report := devruntime.DoctorReport{
		Root: t.TempDir(),
		Checks: []devruntime.Check{{
			Name:   "tool.protoc-gen-go",
			Status: devruntime.CheckFail,
			Detail: "not found",
			Action: "run make rpc-tools to install the exact protoc-gen-go version",
		}},
	}
	items, err := adaptDoctorReport(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Code != "YUNKA-DX-TOOLCHAIN-112" || len(items[0].Actions) != 1 {
		t.Fatalf("items=%#v", items)
	}
	if items[0].Actions[0].Kind != diagnostic.ActionCommand {
		t.Fatalf("action=%#v", items[0].Actions[0])
	}
}

func TestAdaptDoctorReportFailsClosedOnUnmappedProbe(t *testing.T) {
	_, err := adaptDoctorReport(devruntime.DoctorReport{Checks: []devruntime.Check{{Name: "future.check", Status: devruntime.CheckPass}}})
	if err == nil {
		t.Fatal("expected unmapped Doctor probe to fail adapter")
	}
}
