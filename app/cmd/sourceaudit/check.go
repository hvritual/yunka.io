package sourceaudit

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type sourceImport struct {
	path string
	line int
}
type sourceInfo struct {
	index   int
	imports []sourceImport
}

// Check never runs business code and never gives the Go command the original
// repository as its working tree. Invalid policy is an input error; missing
// source/tool/profile evidence is a non-PASS report, not empty success.
func Check(ctx context.Context, root, policyPath string) (Report, error) {
	return check(ctx, root, policyPath, runGo)
}

func check(ctx context.Context, root, policyPath string, run runner) (r Report, err error) {
	r = Report{SchemaVersion: SchemaVersion, Status: Incomplete, Backend: "go-list-source-policy-v1", Files: []File{}, Modules: []Module{}, References: []LocalReference{}, Profiles: []ProfileResult{}, Findings: []Finding{}}
	if ctx == nil {
		return r, fmt.Errorf("source audit requires a context")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return r, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return r, fmt.Errorf("source root is unavailable")
	}
	if !filepath.IsAbs(policyPath) {
		policyPath = filepath.Join(root, policyPath)
	}
	rel, err := filepath.Rel(root, policyPath)
	if err != nil || !relative(filepath.ToSlash(rel), false) {
		return r, fmt.Errorf("source policy must be a root-contained regular file")
	}
	policyPath = filepath.ToSlash(rel)
	before, scanErr := scan(ctx, root)
	if scanErr != nil {
		r.Inventory = Inventory{Complete: false, Files: len(before.names), Bytes: before.bytes}
		r.add("AG-SRC-000", Unknown, "", "", "", "", scanErr.Error())
		r.finish()
		return r, nil
	}
	policyFile, ok := before.files[policyPath]
	if !ok {
		return r, fmt.Errorf("source policy is missing")
	}
	p, err := ReadPolicy(policyFile.data)
	if err != nil {
		return r, err
	}
	r.PolicySHA256 = policyFile.hash
	for _, m := range p.Modules {
		r.Analysis.RequiredProfiles += len(m.Profiles)
	}
	t := inventory(root, before, p, &r)
	// Always recheck original inputs, including additions, removals and policy
	// edits, even when loading or a required profile fails.
	defer func() {
		after, e := scan(context.WithoutCancel(ctx), root)
		r.SourceUnchanged = e == nil && after.digest == before.digest
		if !r.SourceUnchanged {
			r.add("AG-SRC-007", Unknown, "", "", "", "", "original source or policy changed during analysis")
		}
		r.finish()
	}()
	infos := parseSource(before, &r)
	if hasUnknown(r.Findings) {
		return r, nil
	}
	tmpBase, err := filepath.Abs(os.TempDir())
	if err != nil {
		return r, err
	}
	if rel, e := filepath.Rel(root, tmpBase); e == nil && relative(filepath.ToSlash(rel), true) {
		r.add("AG-SRC-000", Unknown, "", "", "", "", "temporary workspace must be outside the audited root")
		return r, nil
	}
	tmp, err := os.MkdirTemp(tmpBase, "yunka-sourceaudit-")
	if err != nil {
		r.add("AG-SRC-000", Unknown, "", "", "", "", "cannot create private analysis workspace")
		return r, nil
	}
	defer os.RemoveAll(tmp)
	projection, err := materialize(root, tmp, before, t)
	if err != nil {
		r.add("AG-SRC-000", Unknown, "", "", "", "", "cannot materialize private source projection")
		return r, nil
	}
	binary, version, targets, err := toolchain(ctx, run, tmp)
	if err != nil {
		r.add("AG-SRC-000", Unknown, "", "", "", "", err.Error())
		return r, nil
	}
	r.Toolchain = version
	profiles := map[string]Profile{}
	for _, v := range p.Profiles {
		v.Tags = append([]string{}, v.Tags...)
		sort.Strings(v.Tags)
		profiles[v.Name] = v
	}
	modules := append([]ModulePolicy(nil), p.Modules...)
	sort.Slice(modules, func(i, j int) bool { return modules[i].Path < modules[j].Path })
	for _, m := range modules {
		if m.Kind == "manifest-only" {
			continue
		}
		names := append([]string(nil), m.Profiles...)
		sort.Strings(names)
		for _, n := range names {
			profile := profiles[n]
			if !targets[profile.GOOS+"/"+profile.GOARCH] {
				r.add("AG-SRC-003", Unknown, path.Join(m.Path, "go.mod"), n, "", "", "unsupported target for the checker Go toolchain")
				r.Profiles = append(r.Profiles, ProfileResult{Module: m.Path, Profile: n, Status: Incomplete})
				continue
			}
			if ctx.Err() != nil {
				r.add("AG-SRC-000", Unknown, "", n, "", "", "analysis cancelled before required profile")
				r.Profiles = append(r.Profiles, ProfileResult{Module: m.Path, Profile: n, Status: Incomplete})
				continue
			}
			runProfile(ctx, run, binary, tmp, before, p, t, m, profile, infos, &r)
		}
	}
	for i := range r.Files {
		f := &r.Files[i]
		if f.Disposition == "uncovered" {
			if len(f.Profiles) > 0 {
				f.Disposition = "checked"
			} else {
				r.add("AG-SRC-003", Unknown, f.Path, "", f.Component, "", "Go source has no active coverage in its declared module matrix")
			}
		}
	}
	afterProjection, e := scan(context.WithoutCancel(ctx), tmp)
	if e != nil || afterProjection.digest != projection.digest {
		r.add("AG-SRC-007", Unknown, "", "", "", "", "Go tool changed its private source/manifest projection; evidence is not for the declared inputs")
	}
	if ctx.Err() != nil {
		r.add("AG-SRC-000", Unknown, "", "", "", "", "analysis budget was cancelled or exceeded")
	}
	return r, nil
}

func parseSource(s snapshot, r *Report) map[string]sourceInfo {
	infos := map[string]sourceInfo{}
	for i := range r.Files {
		f := &r.Files[i]
		if f.Kind != "production" && f.Kind != "test" {
			continue
		}
		set := token.NewFileSet()
		tree, err := parser.ParseFile(set, f.Path, s.files[f.Path].data, parser.ParseComments|parser.AllErrors)
		if err != nil {
			r.add("AG-SRC-000", Unknown, f.Path, "", "", "", "Go source cannot be parsed")
			continue
		}
		f.Generated = ast.IsGenerated(tree)
		info := sourceInfo{index: i}
		for _, im := range tree.Imports {
			v, e := strconv.Unquote(im.Path.Value)
			if e != nil {
				r.add("AG-SRC-000", Unknown, f.Path, "", "", "", "invalid Go import literal")
				continue
			}
			info.imports = append(info.imports, sourceImport{path: v, line: set.PositionFor(im.Pos(), false).Line})
		}
		infos[f.Path] = info
	}
	return infos
}
func hasUnknown(a []Finding) bool {
	for _, f := range a {
		if f.Class == Unknown {
			return true
		}
	}
	return false
}
func canonicalImport(s string) string {
	if i := strings.Index(s, " ["); i >= 0 {
		return s[:i]
	}
	return s
}
