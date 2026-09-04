package change

import "testing"

func TestChangeCommandExposesV1AndChangeSetProtocols(t *testing.T) {
	command := Command()
	seen := map[string]bool{}
	var setSubcommands map[string]bool
	for _, subcommand := range command.Subcommands {
		seen[subcommand.Name] = true
		if subcommand.Name != "set" {
			continue
		}
		setSubcommands = map[string]bool{}
		for _, nested := range subcommand.Subcommands {
			setSubcommands[nested.Name] = true
		}
	}
	for _, expected := range []string{"plan", "begin", "check", "verify", "set"} {
		if !seen[expected] {
			t.Fatalf("missing change subcommand %s: %#v", expected, seen)
		}
	}
	if len(setSubcommands) != 2 || !setSubcommands["begin"] || !setSubcommands["check"] {
		t.Fatalf("ChangeSet protocol must expose exactly begin/check, got %#v", setSubcommands)
	}
}
