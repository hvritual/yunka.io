package sourceaudit

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSourceAuditRejectsInheritedGOROOTBeforeToolExecution(t *testing.T) {
	if mode := os.Getenv("YUNKA_AG05_GOROOT_CHILD"); mode != "" {
		if mode == "removed-after-start" {
			if err := os.Unsetenv("GOROOT"); err != nil {
				t.Fatal(err)
			}
		}
		report, err := Check(context.Background(), os.Getenv("YUNKA_AG05_FIXTURE"), "policy.json")
		if err != nil {
			t.Fatal(err)
		}
		expect(t, report, Incomplete, "AG-SRC-000", "")
		found := false
		for _, f := range report.Findings {
			found = found || strings.Contains(f.Message, "GOROOT unset")
		}
		if !found || report.Analysis.CompletedProfiles != 0 || !report.SourceUnchanged {
			t.Fatalf("untrusted startup installation not rejected precisely: %+v", report)
		}
		return
	}
	root := fixture(t, fixturePolicy(), map[string]string{"ok.go": "package nebula\n"})
	fake := t.TempDir()
	marker := filepath.Join(fake, "invoked")
	put(t, fake, "bin/go", fmt.Sprintf("#!/bin/sh\nprintf invoked > %q\nexit 42\n", marker))
	if err := os.Chmod(filepath.Join(fake, "bin/go"), 0700); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"inherited", "removed-after-start"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, self, "-test.run=^TestSourceAuditRejectsInheritedGOROOTBeforeToolExecution$", "-test.count=1")
			for _, entry := range os.Environ() {
				if !strings.HasPrefix(entry, "GOROOT=") && !strings.HasPrefix(entry, "YUNKA_AG05_GOROOT_CHILD=") && !strings.HasPrefix(entry, "YUNKA_AG05_FIXTURE=") {
					cmd.Env = append(cmd.Env, entry)
				}
			}
			cmd.Env = append(cmd.Env, "GOROOT="+fake, "YUNKA_AG05_GOROOT_CHILD="+mode, "YUNKA_AG05_FIXTURE="+root)
			cmd.WaitDelay = time.Second
			if err := prepareCommand(cmd); err != nil {
				t.Fatal(err)
			}
			defer cleanupCommand(cmd)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("startup-GOROOT regression failed: %v\n%s", err, out)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("caller Go executable ran: %v", err)
			}
		})
	}
}

func TestSourceAuditRejectsLateGOROOTOverride(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"ok.go": "package nebula\n"})
	t.Setenv("GOROOT", t.TempDir())
	calls := 0
	spy := func(context.Context, string, string, []string, ...string) ([]byte, error) {
		calls++
		return nil, fmt.Errorf("must not execute")
	}
	report, err := check(context.Background(), root, "policy.json", spy)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, report, Incomplete, "AG-SRC-000", "")
	if calls != 0 || !report.SourceUnchanged || report.Analysis.CompletedProfiles != 0 {
		t.Fatalf("late override reached tool execution: calls=%d report=%+v", calls, report)
	}
}

func TestSourceAuditAcceptsNoGOROOTOverride(t *testing.T) {
	t.Setenv("GOROOT", "")
	root := fixture(t, fixturePolicy(), map[string]string{"ok.go": "package nebula\n"})
	expect(t, execute(t, root), Pass, "", "")
}
