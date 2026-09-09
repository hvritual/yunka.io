package sourceaudit

import (
	"bytes"
	"context"
	"encoding/json"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type packageError struct{ Err string }
type moduleMeta struct {
	Path, Dir, GoMod, Version string
	Replace                   *moduleMeta
}
type packageMeta struct {
	Dir, ImportPath, Name, ForTest                               string
	Standard, Incomplete, DepOnly                                bool
	Module                                                       *moduleMeta
	GoFiles, CgoFiles, TestGoFiles, XTestGoFiles, IgnoredGoFiles []string
	Imports                                                      []string
	ImportMap                                                    map[string]string
	Error                                                        *packageError
	DepsErrors                                                   []packageError
}

func jsonDecoder(b []byte) *json.Decoder { return json.NewDecoder(bytes.NewReader(b)) }
func inputPath(root, dir, file string) (string, bool) {
	n := file
	if !filepath.IsAbs(n) {
		n = filepath.Join(dir, n)
	}
	rel, err := filepath.Rel(root, n)
	if err != nil || !relative(filepath.ToSlash(rel), false) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
func packageDir(root, dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil || !relative(filepath.ToSlash(rel), true) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
func sourceNames(p packageMeta) []string {
	out := append([]string{}, p.GoFiles...)
	out = append(out, p.CgoFiles...)
	out = append(out, p.TestGoFiles...)
	return append(out, p.XTestGoFiles...)
}
func inactive(p packageMeta) bool {
	if p.Error == nil || len(sourceNames(p)) > 0 || len(p.DepsErrors) > 0 {
		return false
	}
	return p.Error.Err == "build constraints exclude all Go files in "+p.Dir || p.Error.Err == "no Go files in "+p.Dir
}
func synthetic(root string, p packageMeta) bool {
	if p.ForTest != "" || strings.Contains(p.ImportPath, " [") {
		return true
	}
	if p.Name != "main" || !strings.HasSuffix(p.ImportPath, ".test") || len(p.GoFiles) == 0 {
		return false
	}
	for _, n := range p.GoFiles {
		if _, ok := inputPath(root, p.Dir, n); ok {
			return false
		}
	}
	return true
}

func runProfile(ctx context.Context, run runner, binary, root string, s snapshot, policy Policy, t topology, m ModulePolicy, profile Profile, infos map[string]sourceInfo, r *Report) {
	start := len(r.Findings)
	result := ProfileResult{Module: m.Path, Profile: profile.Name, Status: Pass, Rules: []string{"AG-SRC-004", "AG-SRC-005", "AG-SRC-006"}}
	defer func() {
		for _, f := range r.Findings[start:] {
			if f.Class == Proven && result.Status == Pass {
				result.Status = Fail
			}
			if f.Class == Unknown {
				result.Status = Incomplete
			}
		}
		r.Profiles = append(r.Profiles, result)
	}()
	dirs := map[string]bool{}
	for n, info := range infos {
		f := r.Files[info.index]
		if f.Module == m.Path {
			dirs[path.Dir(n)] = true
		}
	}
	ordered := make([]string, 0, len(dirs))
	for d := range dirs {
		ordered = append(ordered, d)
	}
	sort.Strings(ordered)
	if len(ordered) == 0 {
		r.add("AG-SRC-003", Unknown, path.Join(m.Path, "go.mod"), profile.Name, "", "", "module has no applicable Go source directories")
		return
	}
	workspace := "off"
	if m.Workspace != "off" {
		workspace = filepath.Join(root, filepath.FromSlash(m.Workspace))
	}
	env := environment(profile, workspace)
	graph := map[string]packageMeta{}
	seenDirs, activeFiles, activePackages := map[string]bool{}, map[string]bool{}, map[string]bool{}
	// Explicit physical roots include wildcard-omitted and unimported packages.
	// Batches bound command size; all share the exact same immutable projection.
	for offset := 0; offset < len(ordered); offset += 128 {
		end := offset + 128
		if end > len(ordered) {
			end = len(ordered)
		}
		args := []string{"list", "-e", "-deps", "-test", "-json", "-mod=readonly"}
		if len(profile.Tags) > 0 {
			args = append(args, "-tags="+strings.Join(profile.Tags, ","))
		}
		for _, d := range ordered[offset:end] {
			rel, _ := filepath.Rel(filepath.FromSlash(m.Path), filepath.FromSlash(d))
			args = append(args, "./"+filepath.ToSlash(rel))
		}
		b, err := run(ctx, binary, filepath.Join(root, filepath.FromSlash(m.Path)), env, args...)
		if err != nil {
			r.add("AG-SRC-000", Unknown, path.Join(m.Path, "go.mod"), profile.Name, "", "", "package loading failed; prepare approved cached dependencies and verify the declared profile")
			return
		}
		loaded, err := decodePackages(b)
		if err != nil {
			r.add("AG-SRC-000", Unknown, path.Join(m.Path, "go.mod"), profile.Name, "", "", err.Error())
			return
		}
		for _, pkg := range loaded {
			dir, inside := packageDir(root, pkg.Dir)
			if !inactive(pkg) && (pkg.Error != nil || len(pkg.DepsErrors) > 0 || pkg.Incomplete) {
				file := ""
				if inside {
					file = dir
				}
				r.add("AG-SRC-000", Unknown, file, profile.Name, pkg.ImportPath, "", "package or dependency metadata is incomplete")
			}
			if synthetic(root, pkg) {
				continue
			}
			graph[pkg.ImportPath] = pkg
			if !inside {
				if pkg.Module != nil {
					for _, own := range policy.Modules {
						if t.modules[own.Path] == pkg.Module.Path {
							r.add("AG-SRC-008", Unknown, path.Join(own.Path, "go.mod"), profile.Name, pkg.Module.Path, "", "an owned module identity resolved to source outside the selected root")
						}
					}
				}
				continue
			}
			if dirs[dir] {
				seenDirs[dir] = true
			}
			if x := policy.exclusion(dir); x != nil {
				if x.Kind == "fixture" && len(sourceNames(pkg)) > 0 {
					r.add("AG-SRC-006", Proven, dir, profile.Name, pkg.ImportPath, "", "excluded fixture entered the active package graph")
				}
				continue
			}
			if !inactive(pkg) && (pkg.Module == nil || pkg.Module.Dir == "") {
				r.add("AG-SRC-008", Unknown, dir, profile.Name, pkg.ImportPath, "", "owned package lacks resolved module identity")
			}
			known := map[string]bool{}
			for _, n := range sourceNames(pkg) {
				name, ok := inputPath(root, pkg.Dir, n)
				if !ok {
					r.add("AG-SRC-003", Unknown, dir, profile.Name, pkg.ImportPath, "", "loaded source escapes its inventoried root")
					continue
				}
				known[name] = true
				if _, ok := s.files[name]; !ok {
					r.add("AG-SRC-003", Unknown, name, profile.Name, pkg.ImportPath, "", "loaded source was not in the physical inventory")
					continue
				}
				if x := policy.exclusion(name); x != nil {
					if x.Kind == "fixture" {
						r.add("AG-SRC-006", Proven, name, profile.Name, pkg.ImportPath, "", "excluded fixture file entered the active package graph")
					}
					continue
				}
				info, ok := infos[name]
				if !ok {
					r.add("AG-SRC-003", Unknown, name, profile.Name, pkg.ImportPath, "", "active Go source has no parsed rule input")
					continue
				}
				if r.Files[info.index].Module == m.Path {
					activeFiles[name] = true
					activePackages[dir] = true
				}
			}
			for _, n := range pkg.IgnoredGoFiles {
				if name, ok := inputPath(root, pkg.Dir, n); ok {
					known[name] = true
				}
			}
			if dirs[dir] {
				for _, n := range s.names {
					if path.Dir(n) == dir && strings.HasSuffix(n, ".go") && policy.exclusion(n) == nil && !known[n] {
						r.add("AG-SRC-003", Unknown, n, profile.Name, pkg.ImportPath, "", "Go metadata omitted an inventoried source file")
					}
				}
			}
		}
	}
	for _, dir := range ordered {
		if !seenDirs[dir] {
			r.add("AG-SRC-003", Unknown, dir, profile.Name, "", "", "Go metadata omitted an explicit source directory")
		}
	}
	for name := range activeFiles {
		f := &r.Files[infos[name].index]
		if !contains(f.Profiles, profile.Name) {
			f.Profiles = append(f.Profiles, profile.Name)
		}
	}
	result.Packages = len(activePackages)
	result.Files = len(activeFiles)
	if result.Files == 0 {
		r.add("AG-SRC-003", Unknown, path.Join(m.Path, "go.mod"), profile.Name, "", "", "required module profile has no active source")
	}
	checkImports(root, policy, profile.Name, graph, infos, r)
	checkProductionReachability(root, policy, profile.Name, graph, r)
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}

// Preserve byte-for-byte report stability independently of Go metadata order.
func sortedPackages(graph map[string]packageMeta) []string {
	out := make([]string, 0, len(graph))
	for n := range graph {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
func metadataTarget(pkg packageMeta, importPath string) string {
	if mapped := pkg.ImportMap[importPath]; mapped != "" {
		return canonicalImport(mapped)
	}
	return importPath
}
