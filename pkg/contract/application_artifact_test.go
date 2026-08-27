package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationArtifactsWriteCheckAndRemoveOnlyGeneratedStaleFiles(t *testing.T) {
	root := t.TempDir()
	files := []GeneratedApplicationFile{
		{Path: "device/application/zz_yunka_device_application_port_gen.go", Content: []byte(GeneratedApplicationMarker + "\n\npackage application\n")},
		{Path: "device/policy/zz_yunka_device_policy_gen.go", Content: []byte(GeneratedApplicationMarker + "\n\npackage policy\n")},
	}
	if err := WriteApplicationCode(root, files); err != nil {
		t.Fatal(err)
	}
	if drift, err := CheckApplicationCode(root, files); err != nil || len(drift) != 0 {
		t.Fatalf("unexpected generated drift=%#v err=%v", drift, err)
	}
	stale := filepath.Join(root, "device", "application", "zz_yunka_stale_gen.go")
	if err := os.WriteFile(stale, []byte(GeneratedApplicationMarker+"\n\npackage application\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	developer := filepath.Join(root, "device", "application", "service.go")
	if err := os.WriteFile(developer, []byte("package application\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := CheckApplicationCode(root, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || drift[0].File != "device/application/zz_yunka_stale_gen.go" {
		t.Fatalf("expected one stale generated file, got %#v", drift)
	}
	if err := WriteApplicationCode(root, files); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale generated file was not removed: %v", err)
	}
	if contents, err := os.ReadFile(developer); err != nil || string(contents) != "package application\n" {
		t.Fatalf("developer-owned file changed: %q err=%v", contents, err)
	}
}

func TestApplicationArtifactsRejectInvalidOrUnmarkedFiles(t *testing.T) {
	root := t.TempDir()
	cases := []GeneratedApplicationFile{
		{Path: "../escape.go", Content: []byte(GeneratedApplicationMarker + "\n")},
		{Path: "device/application/service.go", Content: []byte("package application\n")},
	}
	for _, file := range cases {
		if err := WriteApplicationCode(root, []GeneratedApplicationFile{file}); err == nil {
			t.Fatalf("expected invalid generated file rejection: %#v", file)
		}
	}
}

func TestApplicationArtifactsDetectContentDrift(t *testing.T) {
	root := t.TempDir()
	file := GeneratedApplicationFile{Path: "device/transport/rpc/zz_yunka_device_rpc_adapter_gen.go", Content: []byte(GeneratedApplicationMarker + "\n\npackage rpc\n")}
	if err := WriteApplicationCode(root, []GeneratedApplicationFile{file}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, filepath.FromSlash(file.Path))
	if err := os.WriteFile(path, []byte(GeneratedApplicationMarker+"\n\npackage rpc\n// drift\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drift, err := CheckApplicationCode(root, []GeneratedApplicationFile{file})
	if err != nil {
		t.Fatal(err)
	}
	if len(drift) != 1 || !strings.Contains(drift[0].Reason, "differs") {
		t.Fatalf("unexpected drift: %#v", drift)
	}
}
