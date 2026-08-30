package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInitializeDefaultsToYK(t *testing.T) {
	root := t.TempDir()
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != ConfigVersion {
		t.Fatalf("version=%d want %d", config.Version, ConfigVersion)
	}
	if config.Database.TablePrefix != "yk" {
		t.Fatalf("prefix=%q want yk", config.Database.TablePrefix)
	}
	if config.Workflow.Contract.ProtoRoot != "contracts/proto" || config.Workflow.Contract.Generated != "contracts/generated" {
		t.Fatalf("unexpected contract profile: %#v", config.Workflow.Contract)
	}
	if config.Workflow.Modules.Root != "modules" || config.Workflow.GeneratedGo.Root != "internal" || config.Workflow.Dev.Manifest != ".yunka/dev.json" {
		t.Fatalf("unexpected workflow profile: %#v", config.Workflow)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ConfigRelativePath))); err != nil {
		t.Fatal(err)
	}
}

func TestInitializePrefersExplicitSourceInventory(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "contracts", "sources.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{\"schemaVersion\":1,\"sourceSets\":[]}"), 0o640); err != nil {
		t.Fatal(err)
	}
	// The source inventory only needs to exist for init's deterministic location choice;
	// its semantic content is validated later by the contract compiler.
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.Workflow.Contract.Sources != "contracts/sources.json" || config.Workflow.Contract.ProtoRoot != "" {
		t.Fatalf("unexpected contract source profile: %#v", config.Workflow.Contract)
	}
}

func TestInitializeAcceptsExplicitPrefixAndIsStable(t *testing.T) {
	root := t.TempDir()
	config, err := Initialize(root, "biz")
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.TablePrefix != "biz" {
		t.Fatalf("prefix=%q want biz", config.Database.TablePrefix)
	}
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ConfigRelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(root, "biz"); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(ConfigRelativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("second initialization changed deterministic project profile bytes")
	}
	if _, err := Initialize(root, "iot"); err == nil {
		t.Fatal("expected prefix mutation to be rejected")
	}
}

func TestLoadLegacyV1UpgradesInMemoryWithoutMutation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(ConfigRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("{\n  \"version\": 1,\n  \"database\": {\n    \"tablePrefix\": \"legacy\"\n  }\n}\n")
	if err := os.WriteFile(path, legacy, 0o640); err != nil {
		t.Fatal(err)
	}
	config, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != ConfigVersion || config.Database.TablePrefix != "legacy" {
		t.Fatalf("unexpected migrated config: %#v", config)
	}
	if config.Workflow.GeneratedGo.Root != "internal" {
		t.Fatalf("legacy profile did not receive workflow defaults: %#v", config.Workflow)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy, after) {
		t.Fatal("Load mutated legacy project profile")
	}
}

func TestInitializePersistsLegacyV1Migration(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, filepath.FromSlash(ConfigRelativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	legacy := []byte("{\"version\":1,\"database\":{\"tablePrefix\":\"legacy\"}}")
	if err := os.WriteFile(path, legacy, 0o640); err != nil {
		t.Fatal(err)
	}
	config, err := Initialize(root, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if config.Version != ConfigVersion {
		t.Fatalf("version=%d", config.Version)
	}
	loaded, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != ConfigVersion || loaded.Database.TablePrefix != "legacy" {
		t.Fatalf("unexpected persisted migration: %#v", loaded)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(contents, legacy) {
		t.Fatal("legacy bytes were not migrated by explicit init")
	}
}

func TestValidateRejectsConflictingContractSources(t *testing.T) {
	config := DefaultConfig()
	config.Workflow.Contract.Sources = "contracts/sources.json"
	if err := Validate(config); err == nil {
		t.Fatal("expected sources + protoRoot conflict")
	}
}

func TestValidateRejectsProfilePathEscape(t *testing.T) {
	config := DefaultConfig()
	config.Workflow.Modules.Root = "../modules"
	if err := Validate(config); err == nil {
		t.Fatal("expected project path escape rejection")
	}
}

func TestValidateRejectsAbsoluteStyleGeneratedImport(t *testing.T) {
	config := DefaultConfig()
	config.Workflow.GeneratedGo.Import = "/example.com/demo/internal"
	if err := Validate(config); err == nil {
		t.Fatal("expected invalid generated Go import rejection")
	}
}
