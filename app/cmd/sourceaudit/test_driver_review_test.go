package sourceaudit

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A Go test launch may carry the installation GOROOT in its original environment.
// Exercise real go test with that controlled input, not a guard exemption.
func TestGoTestDriverGOROOTBoundary(t *testing.T) {
	root := t.TempDir()
	put(t, root, "go.mod", "module example.com/testdriver\n\ngo 1.23.0\n")
	put(t, root, "driver_test.go", `package testdriver
import("os"; "testing")
var startupGOROOT = os.Getenv("GOROOT")
func TestDriverStartup(t *testing.T) {
 if startupGOROOT != "" { t.Fatal("AG05_DRIVER_INJECTED_GOROOT") }
}
`)
	goBinary := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, wrapped := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		args := []string{"test", "-count=1", "-timeout=20s"}
		if wrapped {
			args = append(args, "-exec=/usr/bin/env -u GOROOT")
		}
		args = append(args, "./...")
		cmd := exec.CommandContext(ctx, goBinary, args...)
		cmd.Dir = root
		// Deliberately supply the real installation path. Even a matching path is
		// not an approved production override; only the test launcher removes it.
		cmd.Env = environment(Profile{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}, "off")
		cmd.WaitDelay = time.Second
		if err := prepareCommand(cmd); err != nil {
			cancel()
			t.Fatal(err)
		}
		out, err := cmd.CombinedOutput()
		timedOut := ctx.Err()
		cleanupCommand(cmd)
		cancel()
		if timedOut != nil {
			t.Fatalf("test-driver evidence incomplete: %v\n%s", timedOut, out)
		}
		if !wrapped {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(out), "AG05_DRIVER_INJECTED_GOROOT") {
				t.Fatalf("wrong unwrapped-driver failure: %v\n%s", err, out)
			}
		} else if err != nil || strings.Contains(string(out), "AG05_DRIVER_INJECTED_GOROOT") || !strings.Contains(string(out), "ok  ") && !strings.Contains(string(out), "ok\t") {
			t.Fatalf("wrapped driver did not execute successfully: %v\n%s", err, out)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
}
