package applicationboundary

import (
	"fmt"
	"path/filepath"
	"sort"

	"golang.org/x/tools/go/packages"
)

// The wildcard is only an entry-point inventory. Imported packages in _hidden,
// .private and testdata directories are active source even though ./... omits
// them. Inventory dependency metadata first, then promote every same-module
// package to a typed root. Do not load external implementation bodies or infer
// coverage of unimported ignored directories / additional modules.
func loadModuleSource(cfg *packages.Config) ([]*packages.Package, error) {
	if err := verifyLoaderGo(cfg.Context); err != nil {
		return nil, err
	}
	inventory := *cfg
	inventory.Mode = packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedModule | packages.NeedDeps
	inventory.ParseFile = nil
	inventory.Fset = nil
	roots, err := packages.Load(&inventory, "./...")
	if err != nil || cfg.Context.Err() != nil {
		return nil, fmt.Errorf("module import inventory failed or was cancelled")
	}
	modulePath := ""
	for _, p := range roots {
		if p.Module == nil || filepath.Clean(p.Module.Dir) != cfg.Dir {
			return nil, fmt.Errorf("inventory root is outside the selected module")
		}
		if modulePath != "" && modulePath != p.Module.Path {
			return nil, fmt.Errorf("inventory contains multiple source modules")
		}
		modulePath = p.Module.Path
	}
	active := map[string]bool{}
	inactive := map[string]*packages.Package{}
	invalid := false
	packages.Visit(roots, nil, func(p *packages.Package) {
		if len(p.Errors) > 0 {
			invalid = true
		}
		if p.Module == nil || filepath.Clean(p.Module.Dir) != cfg.Dir {
			return
		}
		if p.Module.Path != modulePath || p.PkgPath == "" {
			invalid = true
			return
		}
		if len(p.GoFiles) == 0 && len(p.CompiledGoFiles) == 0 {
			inactive[p.PkgPath] = p
			return
		}
		active[p.PkgPath] = true
	})
	if invalid {
		return nil, fmt.Errorf("module import inventory is incomplete; verify approved dependencies and source")
	}
	if len(active) == 0 {
		return nil, fmt.Errorf("module has no active production source packages")
	}
	patterns := make([]string, 0, len(active))
	for path := range active {
		patterns = append(patterns, path)
	}
	sort.Strings(patterns)
	if err := verifyLoaderGo(cfg.Context); err != nil {
		return nil, err
	}
	loaded, err := packages.Load(cfg, patterns...)
	if err != nil || cfg.Context.Err() != nil {
		return nil, fmt.Errorf("typed module loading failed or was cancelled")
	}
	if err := verifyLoaderGo(cfg.Context); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, p := range loaded {
		if !active[p.PkgPath] || seen[p.PkgPath] {
			return nil, fmt.Errorf("typed package identity differs from the import inventory")
		}
		seen[p.PkgPath] = true
	}
	if len(seen) != len(active) {
		return nil, fmt.Errorf("typed package coverage is incomplete")
	}
	// Preserve explicit test-only / inactive exclusions for the existing report;
	// selecting one as a policy subject still fails in Analyze.
	for _, p := range inactive {
		loaded = append(loaded, p)
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].PkgPath < loaded[j].PkgPath })
	return loaded, nil
}
