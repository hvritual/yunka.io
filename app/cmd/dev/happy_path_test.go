package dev

import (
	"flag"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/urfave/cli"
)

func TestC115ABareDevAndExplicitRunShareOneAction(t *testing.T) {
	command := Command()
	if command.Action == nil {
		t.Fatal("bare yunka dev has no action")
	}
	var run cli.Command
	found := false
	for _, subcommand := range command.Subcommands {
		if subcommand.Name == "run" {
			run = subcommand
			found = true
			break
		}
	}
	if !found || run.Action == nil {
		t.Fatal("explicit yunka dev run action is unavailable")
	}
	parentPointer := reflect.ValueOf(command.Action).Pointer()
	runPointer := reflect.ValueOf(run.Action).Pointer()
	if parentPointer != runPointer {
		t.Fatalf("bare dev action=%x run action=%x; expected one canonical action", parentPointer, runPointer)
	}
}

func TestC115ABareDevExposesRunHappyPathFlags(t *testing.T) {
	seen := commandFlagNames(Command())
	for _, required := range []string{"root", "config", "graph", "target", "closure"} {
		if !seen[required] {
			t.Fatalf("bare dev missing happy-path flag %s", required)
		}
	}
}

func TestC115ALoadPlanUsesProfileDevManifestWhenConfigIsImplicit(t *testing.T) {
	root := t.TempDir()
	writeDevTestFile(t, filepath.Join(root, ".yunka", "project.json"), `{
  "version": 2,
  "database": {"tablePrefix": "yk"},
  "workflow": {
    "contract": {"protoRoot": "contracts/proto", "generated": "contracts/generated"},
    "modules": {"root": "modules"},
    "generatedGo": {"root": "internal"},
    "dev": {"manifest": "dev/custom.json"}
  }
}
`)
	writeDevTestFile(t, filepath.Join(root, "dev", "custom.json"), `{
  "schemaVersion": 2,
  "processes": [
    {"name": "api", "command": ["go", "version"]}
  ]
}
`)

	set := flag.NewFlagSet("dev", flag.ContinueOnError)
	for _, item := range commonFlags() {
		item.Apply(set)
	}
	if err := set.Set("root", root); err != nil {
		t.Fatal(err)
	}
	ctx := cli.NewContext(cli.NewApp(), set, nil)
	plan, err := loadPlan(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Processes) != 1 || plan.Processes[0].Name != "api" {
		t.Fatalf("unexpected profile-backed plan: %#v", plan)
	}
}
