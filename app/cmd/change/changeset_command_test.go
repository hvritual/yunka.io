package change

import "testing"

func TestChangeCommandExposesV1AndChangeSetProtocols(t *testing.T) {
	command := Command()
	seen := map[string]bool{}
	var setSubcommands map[string]bool
	var remediationSubcommands map[string]bool
	for _, subcommand := range command.Subcommands {
		seen[subcommand.Name] = true
		if subcommand.Name != "set" {
			continue
		}
		setSubcommands = map[string]bool{}
		for _, nested := range subcommand.Subcommands {
			setSubcommands[nested.Name] = true
			if nested.Name != "remediation" {
				continue
			}
			remediationSubcommands = map[string]bool{}
			for _, remediation := range nested.Subcommands {
				remediationSubcommands[remediation.Name] = true
			}
		}
	}
	for _, expected := range []string{"plan", "begin", "check", "verify", "set"} {
		if !seen[expected] {
			t.Fatalf("missing change subcommand %s: %#v", expected, seen)
		}
	}
	if len(setSubcommands) != 3 || !setSubcommands["begin"] || !setSubcommands["check"] || !setSubcommands["remediation"] {
		t.Fatalf("ChangeSet protocol must expose begin/check/remediation, got %#v", setSubcommands)
	}
	if len(remediationSubcommands) != 2 || !remediationSubcommands["bind"] || !remediationSubcommands["check"] {
		t.Fatalf("ChangeSet remediation protocol must expose exactly bind/check, got %#v", remediationSubcommands)
	}
}
