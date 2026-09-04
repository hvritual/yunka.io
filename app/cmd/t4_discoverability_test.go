package main

import (
	"strings"
	"testing"

	"github.com/urfave/cli"
)

func TestT4DiscoverabilityExposesPlanFirstChangeSetAndRemediationProof(t *testing.T) {
	app := cli.NewApp()
	applyDiscoverability(app)
	for _, expected := range []string{
		"add operation --plan -> change set begin -> add operation -> generate -> change set check",
		"change set remediation bind",
		"change set remediation check",
		"Declaring a remediation target never proves it fixed",
		"rejects new proven debt",
	} {
		if !strings.Contains(app.Description, expected) {
			t.Fatalf("T4 discoverability missing %q: %q", expected, app.Description)
		}
	}
}
