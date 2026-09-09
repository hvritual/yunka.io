package sourceaudit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestManifestOnlyAggregatorIsNotASourceExemption(t *testing.T) {
	p := fixturePolicy()
	p.Modules = []ModulePolicy{{Path: ".", Kind: "manifest-only", Workspace: "off", Profiles: []string{}}, {Path: "child", Workspace: "off", Profiles: []string{"host"}}}
	root := fixture(t, p, map[string]string{"child/go.mod": "module example.com/child\ngo 1.23.0\n", "child/base.go": "package child\n"})
	r := execute(t, root)
	expect(t, r, Pass, "", "")
	if r.Analysis.RequiredProfiles != 1 || r.Analysis.CompletedProfiles != 1 {
		t.Fatal("invented aggregator profile")
	}
	put(t, root, "new.go", "package nebula\n")
	expect(t, execute(t, root), Incomplete, "AG-SRC-001", "new.go")
}

func TestDuplicateOwnedModuleIdentity(t *testing.T) {
	p := fixturePolicy()
	p.Modules = append(p.Modules, ModulePolicy{Path: "duplicate", Workspace: "off", Profiles: []string{"host"}})
	root := fixture(t, p, map[string]string{"base.go": "package nebula\n", "duplicate/go.mod": "module example.com/nebula\ngo 1.23.0\n", "duplicate/base.go": "package nebula\n"})
	expect(t, execute(t, root), Incomplete, "AG-SRC-001", "duplicate/go.mod")
}

func TestExcludedDependencyRemainsInventoriedAndResolved(t *testing.T) {
	p := fixturePolicy()
	p.Exclusions = []Exclusion{{Path: "dependency", Kind: "dependency", Reason: "Pinned external implementation; source bodies are not owned"}}
	root := fixture(t, p, map[string]string{"go.mod": "module example.com/nebula\ngo 1.23.0\nrequire example.com/dependency v0.0.0\nreplace example.com/dependency => ./dependency\n", "base.go": "package nebula\nimport _ \"example.com/dependency\"\n", "dependency/go.mod": "module example.com/dependency\ngo 1.23.0\n", "dependency/base.go": "package dependency\n"})
	r := execute(t, root)
	expect(t, r, Pass, "", "")
	if r.Inventory.GoFiles != 2 || r.Analysis.ExcludedGoFiles != 1 || r.Analysis.CheckedGoFiles != 1 {
		t.Fatalf("lost dependency inventory: %+v", r.Analysis)
	}
	p.TestSupportImports = []string{"example.com/dependency"}
	writePolicy(t, root, p)
	expect(t, execute(t, root), Fail, "AG-SRC-005", "")
	p.TestSupportImports = nil
	writePolicy(t, root, p)
	put(t, root, "dependency/base.go", "package dependency\nimport _ \"testing\"\n")
	expect(t, execute(t, root), Fail, "AG-SRC-005", "")
}

func TestAbsoluteGoIgnoresParentWrapperAndBuildHooks(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"base.go": "package nebula\n"})
	bin := t.TempDir()
	marker := filepath.Join(bin, "invoked")
	put(t, bin, "go", fmt.Sprintf("#!/bin/sh\ntouch %q\nexit 42\n", marker))
	if e := os.Chmod(filepath.Join(bin, "go"), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GOFLAGS", "-invalid-parent-flag")
	t.Setenv("GOWORK", filepath.Join(bin, "nonexistent"))
	t.Setenv("GOPACKAGESDRIVER", filepath.Join(bin, "go"))
	t.Setenv("GOEXPERIMENT", "invalidexperiment")
	expect(t, execute(t, root), Pass, "", "")
	if _, e := os.Stat(marker); !os.IsNotExist(e) {
		t.Fatalf("parent wrapper executed: %v", e)
	}
}

func TestToolchainMismatchAndCancellation(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"base.go": "package nebula\n"})
	bad := func(ctx context.Context, binary, dir string, env []string, args ...string) ([]byte, error) {
		if args[0] == "version" {
			return []byte("go version go0.0 linux/amd64\n"), nil
		}
		return runGo(ctx, binary, dir, env, args...)
	}
	r, e := check(context.Background(), root, "policy.json", bad)
	if e != nil {
		t.Fatal(e)
	}
	expect(t, r, Incomplete, "AG-SRC-000", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = Check(ctx, root, "policy.json")
	if e != nil {
		t.Fatal(e)
	}
	expect(t, r, Incomplete, "AG-SRC-000", "")
}

func TestMetadataTruncationAndUnknownProductionTarget(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"base.go": "package nebula\nimport _ \"fmt\"\n"})
	run := func(ctx context.Context, binary, dir string, env []string, args ...string) ([]byte, error) {
		if args[0] != "list" {
			return runGo(ctx, binary, dir, env, args...)
		}
		v := packageMeta{Dir: dir, ImportPath: "example.com/nebula", Name: "nebula", Module: &moduleMeta{Path: "example.com/nebula", Dir: dir}, GoFiles: []string{"base.go"}, Imports: []string{"fmt"}}
		return json.Marshal(v)
	}
	r, e := check(context.Background(), root, "policy.json", run)
	if e != nil {
		t.Fatal(e)
	}
	expect(t, r, Incomplete, "AG-SRC-003", "base.go")
	for _, raw := range []string{"{", "{}", "null", `{"ImportPath":"a"} trailing`} {
		if _, e := decodePackages([]byte(raw)); e == nil {
			t.Fatalf("accepted invalid metadata: %s", raw)
		}
	}
}

func TestOutsideWorkspaceTargetAndMissingModuleSource(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"base.go": "package nebula\n", "go.work": "go 1.23.0\nuse ../outside\n"})
	expect(t, execute(t, root), Incomplete, "AG-SRC-008", "go.work")
	root = fixture(t, fixturePolicy(), nil)
	expect(t, execute(t, root), Incomplete, "AG-SRC-003", "")
}

func TestUnixRunnerTerminatesDescendants(t *testing.T) {
	// Qualification runs on Linux. Other native hosts must not silently mark this
	// process-ownership proof as passed; the !unix runner is explicitly incomplete.
	if runtime.GOOS != "linux" {
		t.Fatal("process ownership regression requires the declared Linux qualification host")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 20 &\necho $! > child.pid\nwait\n"
	put(t, dir, "runner", script)
	if e := os.Chmod(filepath.Join(dir, "runner"), 0700); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, e := runGo(ctx, filepath.Join(dir, "runner"), dir, os.Environ()); e == nil {
		t.Fatal("timeout unexpectedly passed")
	}
	if time.Since(start) > 4*time.Second {
		t.Fatal("process cleanup exceeded bounded wait")
	}
	b, e := os.ReadFile(filepath.Join(dir, "child.pid"))
	if e != nil {
		t.Fatal(e)
	}
	pid := strings.TrimSpace(string(b))
	for i := 0; i < 100; i++ {
		b, e = os.ReadFile(filepath.Join("/proc", pid, "stat"))
		if os.IsNotExist(e) || strings.Contains(string(b), ") Z ") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant %s survived runner cancellation", pid)
}

func TestPolicyPathsBindActualSourceUnits(t *testing.T) {
	for _, mode := range []string{"component_file", "stale_fixture", "nonmodule_dependency"} {
		t.Run(mode, func(t *testing.T) {
			p := fixturePolicy()
			switch mode {
			case "component_file":
				p.Components = append(p.Components, Component{Name: "file", Path: "base.go", Kind: "production", Allow: []string{}, AllowExternal: true})
			case "stale_fixture":
				p.Exclusions = []Exclusion{{Path: "missing", Kind: "fixture", Reason: "No such fixture should be silently accepted"}}
			case "nonmodule_dependency":
				p.Exclusions = []Exclusion{{Path: "helper", Kind: "dependency", Reason: "An owned package is not an external module"}}
			}
			root := fixture(t, p, map[string]string{"base.go": "package nebula\n", "helper/h.go": "package helper\n"})
			r := execute(t, root)
			rule := "AG-SRC-001"
			if mode == "component_file" {
				rule = "AG-SRC-002"
			}
			expect(t, r, Incomplete, rule, "")
		})
	}
}
