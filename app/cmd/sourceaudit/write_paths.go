package sourceaudit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Validate writable locations before Go runs, rather than merely detecting their
// writes afterwards. The audit reads an approved external cache; it does not
// silently redirect or initialize a caller's workspace-local cache.
func checkWritePaths(root string, env []string) error {
	values := map[string]string{}
	for _, entry := range env {
		if k, v, ok := strings.Cut(entry, "="); ok {
			values[k] = v
		}
	}
	paths := map[string]string{}
	for _, k := range []string{"HOME", "USERPROFILE", "TMP", "TEMP", "TMPDIR", "GOCACHE", "GOMODCACHE"} {
		if value := values[k]; value != "" && !(k == "GOCACHE" && value == "off") {
			paths[k] = value
		}
	}
	gopaths := filepath.SplitList(values["GOPATH"])
	for i, value := range gopaths {
		paths[fmt.Sprintf("GOPATH[%d]", i)] = value
	}
	// Go's environment is controlled by environment(): XDG_CACHE_HOME and
	// LOCALAPPDATA are not inherited. Account for defaults derived from HOME.
	home := values["HOME"]
	if runtime.GOOS == "windows" {
		home = values["USERPROFILE"]
	}
	if values["GOMODCACHE"] == "" && len(gopaths) > 0 {
		paths["default GOMODCACHE"] = filepath.Join(gopaths[0], "pkg", "mod")
	}
	if home != "" {
		if values["GOCACHE"] == "" {
			cache := filepath.Join(home, ".cache", "go-build")
			if runtime.GOOS == "darwin" {
				cache = filepath.Join(home, "Library", "Caches", "go-build")
			}
			paths["default GOCACHE"] = cache
		}
		if values["GOPATH"] == "" && values["GOMODCACHE"] == "" {
			paths["default GOMODCACHE"] = filepath.Join(home, "go", "pkg", "mod")
		}
	}
	keys := make([]string, 0, len(paths))
	for k := range paths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		resolved, err := resolveWritePath(paths[key])
		if err != nil {
			return fmt.Errorf("cannot establish an absolute writable path for %s", key)
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || relative(filepath.ToSlash(rel), true) {
			return fmt.Errorf("writable %s must be outside the audited root", key)
		}
		// A cache that is itself an ancestor of the source can write cache entries
		// over that source too. General home/temp parent directories are normal.
		if strings.Contains(key, "CACHE") {
			if rel, err := filepath.Rel(resolved, root); err != nil || relative(filepath.ToSlash(rel), true) {
				return fmt.Errorf("writable %s must not overlap the audited root", key)
			}
		}
	}
	return nil
}

// Resolve symlinked ancestors even when a cache does not yet exist. No directory
// is created and no tool is invoked while establishing this prerequisite.
func resolveWritePath(value string) (string, error) {
	if !filepath.IsAbs(value) {
		return "", fmt.Errorf("relative write path")
	}
	p := filepath.Clean(value)
	var suffix []string
	for {
		_, err := os.Lstat(p)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(p)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", fmt.Errorf("no resolvable ancestor")
		}
		suffix = append(suffix, filepath.Base(p))
		p = parent
	}
}
