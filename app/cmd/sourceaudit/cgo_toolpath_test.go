package sourceaudit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSourceToolPathDoesNotInheritCallerPaths(t *testing.T) {
	parent := t.TempDir()
	t.Setenv("PATH", parent+string(os.PathListSeparator)+".")
	t.Setenv("CC", filepath.Join(parent, "gcc"))
	t.Setenv("CXX", filepath.Join(parent, "g++"))
	t.Setenv("PKG_CONFIG", filepath.Join(parent, "pkg-config"))
	for _, cgo := range []bool{false, true} {
		env := environment(Profile{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CGO: cgo}, "off")
		values := map[string]string{}
		for _, entry := range env {
			k, v, ok := strings.Cut(entry, "=")
			if !ok {
				t.Fatal("invalid environment entry")
			}
			values[k] = v
		}
		want := filepath.Join(runtime.GOROOT(), "bin")
		if runtime.GOOS != "windows" {
			want += string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
		}
		if values["PATH"] != want {
			t.Fatalf("cgo=%t: untrusted tool search path %q", cgo, values["PATH"])
		}
		for _, key := range []string{"CC", "CXX", "PKG_CONFIG"} {
			if _, ok := values[key]; ok {
				t.Fatalf("inherited %s override", key)
			}
		}
	}
}

func cgoSourceFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Fatal("CGO tool-path regression requires the declared Linux qualification host")
	}
	p := fixturePolicy()
	p.Profiles[0].CGO = true
	return fixture(t, p, map[string]string{"cgo.go": `package nebula
/* int source_policy_value(void) { return 7; } */
import "C"
func Value() int { return int(C.source_policy_value()) }
`})
}

func TestCGOProfileIgnoresParentCompilerWrappers(t *testing.T) {
	root := cgoSourceFixture(t)
	bin := t.TempDir()
	marker := filepath.Join(bin, "invoked")
	for _, tool := range []string{"gcc", "cc", "clang", "g++", "c++", "pkg-config"} {
		// Use only shell builtins. Any invocation writes to a test-owned marker,
		// then fails; no wrapper forwards to an unknown external executable.
		put(t, bin, tool, fmt.Sprintf("#!/bin/sh\nprintf invoked > %q\nexit 42\n", marker))
		if err := os.Chmod(filepath.Join(bin, tool), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CC", filepath.Join(bin, "gcc"))
	t.Setenv("CXX", filepath.Join(bin, "g++"))
	t.Setenv("PKG_CONFIG", filepath.Join(bin, "pkg-config"))
	// A fresh cache forces cgo metadata processing, not reuse of an earlier run.
	t.Setenv("GOCACHE", t.TempDir())
	report := execute(t, root)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("parent compiler wrapper executed: %v", err)
	}
	expect(t, report, Pass, "", "")
	if !report.SourceUnchanged || report.Analysis.CompletedProfiles != 1 || report.Analysis.CheckedGoFiles != 1 {
		t.Fatalf("incomplete CGO proof: %+v", report)
	}
}

func TestMissingCGOToolDoesNotFallBackToParentPATH(t *testing.T) {
	root := cgoSourceFixture(t)
	t.Setenv("GOCACHE", t.TempDir())
	// Model a declared profile for which the trusted host installation has no
	// compiler. Do not probe the original PATH or silently disable CGO.
	missing := filepath.Join(t.TempDir(), "no-compiler")
	run := func(ctx context.Context, binary, dir string, env []string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list" {
			env = append(append([]string(nil), env...), "CC="+missing, "CXX="+missing)
		}
		return runGo(ctx, binary, dir, env, args...)
	}
	report, err := check(context.Background(), root, "policy.json", run)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, report, Incomplete, "AG-SRC-000", "")
	if !report.SourceUnchanged || report.Analysis.Complete {
		t.Fatal("missing compiler was counted as complete")
	}
}

func TestCGOToolEnvironmentRequiresTrustedExecutable(t *testing.T) {
	untrusted := filepath.Join(t.TempDir(), "cc")
	if err := os.WriteFile(untrusted, []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, compiler := range []string{"cc", untrusted, untrusted + ".missing"} {
		if _, err := cgoToolEnvironment([]string{"CGO_ENABLED=1", "CC=" + compiler}); err == nil {
			t.Fatalf("untrusted or missing compiler accepted: %s", compiler)
		}
	}
	// No native compiler is required for the non-CGO profile.
	if _, err := cgoToolEnvironment([]string{"CGO_ENABLED=0", "CC=" + untrusted}); err != nil {
		t.Fatal(err)
	}
}
