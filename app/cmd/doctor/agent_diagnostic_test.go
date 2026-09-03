package doctor

import (
	"strings"
	"testing"

	"github.com/hvritual/yunka.io/pkg/devruntime"
)

func TestDoctorAgentJSONPreservesStrictFailureAndRetryCommand(t *testing.T) {
	root := t.TempDir()
	report := devruntime.DoctorReport{
		Root: root,
		Checks: []devruntime.Check{{
			Name:   "git.status",
			Status: devruntime.CheckWarn,
			Detail: "working tree has local changes",
			Action: "review git status before broad generation or cleanup commands",
		}},
	}

	strict, err := renderDoctorAgentJSON(report, true)
	if err != nil {
		t.Fatal(err)
	}
	text := string(strict)
	for _, expected := range []string{
		`"ok": false`,
		`"cause"`,
		`"target"`,
		`"remediation"`,
		`"retry"`,
		`"value": "yunka doctor --strict"`,
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("strict agent diagnostics missing %s:\n%s", expected, text)
		}
	}
	if strings.Contains(text, root) {
		t.Fatalf("agent diagnostics leaked absolute root: %s", text)
	}

	nonStrict, err := renderDoctorAgentJSON(report, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nonStrict), `"ok": true`) || !strings.Contains(string(nonStrict), `"retry": null`) {
		t.Fatalf("non-strict agent diagnostics should succeed without retry:\n%s", nonStrict)
	}
}
