package dev

import (
	"strings"
	"testing"
)

func TestC103StatusCommandExposesExplicitClosureInputs(t *testing.T) {
	command := statusCommand()
	seen := map[string]bool{}
	for _, flag := range command.Flags {
		for _, name := range strings.Split(flag.GetName(), ",") {
			seen[strings.TrimSpace(name)] = true
		}
	}
	for _, required := range []string{"root", "config", "graph", "target", "closure", "state", "format"} {
		if !seen[required] {
			t.Fatalf("dev status missing explicit closure input %s", required)
		}
	}
}

func TestC103PlanRunAndStatusShareClosureFlag(t *testing.T) {
	for _, command := range []struct {
		name  string
		flags []string
	}{
		{name: planCommand().Name, flags: flagNames(planCommand())},
		{name: runCommand().Name, flags: flagNames(runCommand())},
		{name: statusCommand().Name, flags: flagNames(statusCommand())},
	} {
		found := false
		for _, name := range command.flags {
			found = found || name == "closure"
		}
		if !found {
			t.Fatalf("dev %s does not expose --closure", command.name)
		}
	}
}

func flagNames(command interface{ GetName() string }) []string {
	return strings.Split(command.GetName(), ",")
}
