package dev

import (
	"strings"
	"testing"

	"github.com/urfave/cli"
)

func TestC103StatusCommandExposesExplicitClosureInputs(t *testing.T) {
	seen := commandFlagNames(statusCommand())
	for _, required := range []string{"root", "config", "graph", "target", "closure", "state", "format"} {
		if !seen[required] {
			t.Fatalf("dev status missing explicit closure input %s", required)
		}
	}
}

func TestC103PlanRunAndStatusShareClosureFlag(t *testing.T) {
	for _, command := range []cli.Command{planCommand(), runCommand(), statusCommand()} {
		if !commandFlagNames(command)["closure"] {
			t.Fatalf("dev %s does not expose --closure", command.Name)
		}
	}
}

func commandFlagNames(command cli.Command) map[string]bool {
	seen := map[string]bool{}
	for _, flag := range command.Flags {
		for _, name := range strings.Split(flag.GetName(), ",") {
			seen[strings.TrimSpace(name)] = true
		}
	}
	return seen
}
