package assembly

import (
	"strings"
	"testing"
)

func TestCommandExposesGenerateCheckInspect(t *testing.T) {
	command := Command()
	if command.Name != AppName {
		t.Fatalf("name=%s", command.Name)
	}
	seen := map[string]bool{}
	for _, subcommand := range command.Subcommands {
		seen[subcommand.Name] = true
	}
	for _, name := range []string{"generate", "check", "inspect"} {
		if !seen[name] {
			t.Fatalf("assembly command missing %s", name)
		}
	}
}

func TestAssemblyFlagsKeepCanonicalOwnersExplicit(t *testing.T) {
	command := Command()
	for _, subcommand := range command.Subcommands {
		if subcommand.Name != "generate" && subcommand.Name != "check" {
			continue
		}
		seen := map[string]bool{}
		for _, flag := range subcommand.Flags {
			for _, name := range strings.Split(flag.GetName(), ",") {
				seen[strings.TrimSpace(name)] = true
			}
		}
		for _, required := range []string{"proto-dir", "module-root", "out", "code-out", "code-import"} {
			if !seen[required] {
				t.Fatalf("%s missing explicit flag %s", subcommand.Name, required)
			}
		}
	}
}
