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

func TestRootContainedBuildCacheRejectedWithoutWrites(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"ok.go": "package nebula\n"})
	cache := filepath.Join(root, "fresh-cache")
	t.Setenv("GOCACHE", cache)
	before, err := scan(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	r := execute(t, root)
	expect(t, r, Incomplete, "AG-SRC-000", "")
	if !r.SourceUnchanged || r.Analysis.CompletedProfiles != 0 {
		t.Fatalf("cache write occurred before rejection: %+v", r)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatalf("audit created workspace-local cache: %v", err)
	}
	after, err := scan(context.Background(), root)
	if err != nil || before.digest != after.digest {
		t.Fatal("original source mutated")
	}
}

func TestRootContainedWritePathsNeverInvokeGo(t *testing.T) {
	for _, key := range []string{"GOCACHE", "GOMODCACHE", "GOPATH", "HOME", "TMPDIR"} {
		t.Run(key, func(t *testing.T) {
			root := fixture(t, fixturePolicy(), map[string]string{"ok.go": "package nebula\n"})
			t.Setenv(key, filepath.Join(root, "absent-write-target"))
			calls := 0
			spy := func(context.Context, string, string, []string, ...string) ([]byte, error) {
				calls++
				return nil, fmt.Errorf("must not execute")
			}
			r, err := check(context.Background(), root, "policy.json", spy)
			if err != nil {
				t.Fatal(err)
			}
			expect(t, r, Incomplete, "AG-SRC-000", "")
			if calls != 0 || !r.SourceUnchanged {
				t.Fatalf("unsafe write target invoked Go: %d %+v", calls, r)
			}
		})
	}
}

func TestWritePathAliasesAndDefaults(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(outside, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		env  []string
	}{
		{"missing_below_symlink", []string{"GOCACHE=" + filepath.Join(link, "not-created", "cache")}},
		{"gopath_second_entry", []string{"GOPATH=" + outside + string(os.PathListSeparator) + root}},
		{"relative_cache", []string{"GOCACHE=relative"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkWritePaths(root, tc.env); err == nil {
				t.Fatal("unsafe cache identity accepted")
			}
		})
	}
	// Account for Go's implicit home-based cache, without invoking go env.
	home := t.TempDir()
	defaultCache := filepath.Join(home, ".cache", "go-build")
	if runtime.GOOS == "darwin" {
		defaultCache = filepath.Join(home, "Library", "Caches", "go-build")
	}
	if err := os.MkdirAll(defaultCache, 0700); err != nil {
		t.Fatal(err)
	}
	if err := checkWritePaths(defaultCache, []string{"HOME=" + home}); err == nil || !strings.Contains(err.Error(), "GOCACHE") {
		t.Fatalf("implicit root-contained cache accepted: %v", err)
	}
}

func TestExternalBuildCacheRemainsLegal(t *testing.T) {
	root := fixture(t, fixturePolicy(), map[string]string{"ok.go": "package nebula\n"})
	t.Setenv("GOCACHE", t.TempDir())
	r := execute(t, root)
	expect(t, r, Pass, "", "")
	if !r.SourceUnchanged {
		t.Fatal("legal external cache changed source")
	}
}
