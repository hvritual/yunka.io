package sourceaudit

import (
	"path"
	"sort"
	"strings"
)

func checkImports(root string, p Policy, profile string, graph map[string]packageMeta, infos map[string]sourceInfo, r *Report) {
	for _, key := range sortedPackages(graph) {
		pkg := graph[key]
		dir, inside := packageDir(root, pkg.Dir)
		if !inside || p.exclusion(dir) != nil {
			continue
		}
		for _, raw := range sourceNames(pkg) {
			file, ok := inputPath(root, pkg.Dir, raw)
			if !ok {
				continue
			}
			info, ok := infos[file]
			if !ok {
				continue
			}
			from := p.component(file)
			if from == nil {
				continue
			}
			for _, im := range info.imports {
				if im.path == "C" {
					continue
				} // canonical cgo pseudo-import, not a Go capability
				finding := Finding{Class: Proven, Rule: "AG-SRC-004", File: file, Line: im.line, Profile: profile, From: from.Name, To: im.path}
				for _, deny := range from.DenyImports {
					if under(im.path, deny) {
						finding.Message = "import forbidden by the component's explicit package-prefix policy"
						r.Findings = append(r.Findings, finding)
					}
				}
				// Cross-component test code is explicitly non-applicable to the production
				// layer rule. It is still inventoried, parsed and loaded by Go -test.
				if strings.HasSuffix(file, "_test.go") {
					continue
				}
				target, found := graph[metadataTarget(pkg, im.path)]
				if !found {
					r.add("AG-SRC-003", Unknown, file, profile, from.Name, im.path, "Go did not supply the resolved production import target")
					continue
				}
				targetDir, targetInside := packageDir(root, target.Dir)
				if targetInside {
					if x := p.exclusion(targetDir); x != nil {
						if x.Kind == "fixture" {
							finding.Rule = "AG-SRC-006"
							finding.Message = "production source imports excluded fixture code"
							r.Findings = append(r.Findings, finding)
						} else if !from.AllowExternal {
							finding.Message = "external dependency import is not allowed by this component"
							r.Findings = append(r.Findings, finding)
						}
						continue
					}
					to := p.component(targetDir)
					if to == nil {
						r.add("AG-SRC-002", Unknown, file, profile, from.Name, im.path, "resolved owned dependency has no component policy")
						continue
					}
					if from.Kind == "production" && to.Kind == "test-support" {
						finding.Rule = "AG-SRC-005"
						finding.Message = "production source imports declared test-support code"
						r.Findings = append(r.Findings, finding)
						continue
					}
					if to.Name != from.Name && !contains(from.Allow, to.Name) {
						finding.Message = "component dependency is not permitted: " + from.Name + " -> " + to.Name
						r.Findings = append(r.Findings, finding)
					}
				} else if !target.Standard && !from.AllowExternal {
					finding.Message = "external dependency import is not allowed by this component"
					r.Findings = append(r.Findings, finding)
				}
			}
		}
	}
}

// Production reachability uses only original package Imports, never synthetic
// test variants. It prevents a neutral intermediary (even an external package)
// from concealing a dependency on an explicitly declared test-support package.
func checkProductionReachability(root string, p Policy, profile string, graph map[string]packageMeta, r *Report) {
	for _, key := range sortedPackages(graph) {
		start := graph[key]
		dir, ok := packageDir(root, start.Dir)
		if !ok || p.exclusion(dir) != nil || len(start.GoFiles)+len(start.CgoFiles) == 0 {
			continue
		}
		from := p.component(dir)
		if from == nil || from.Kind != "production" {
			continue
		}
		type hop struct {
			id    string
			chain []string
		}
		queue := []hop{{key, []string{key}}}
		seen := map[string]bool{key: true}
		for len(queue) > 0 {
			h := queue[0]
			queue = queue[1:]
			current, ok := graph[h.id]
			if !ok {
				continue
			}
			imports := append([]string(nil), current.Imports...)
			sort.Strings(imports)
			for _, raw := range imports {
				id := metadataTarget(current, raw)
				if seen[id] {
					continue
				}
				seen[id] = true
				chain := append(append([]string{}, h.chain...), id)
				isSupport := id == "testing"
				for _, prefix := range p.TestSupportImports {
					if under(id, prefix) {
						isSupport = true
					}
				}
				if target, ok := graph[id]; ok {
					if targetDir, inside := packageDir(root, target.Dir); inside && p.exclusion(targetDir) == nil {
						if to := p.component(targetDir); to != nil && to.Kind == "test-support" {
							isSupport = true
						}
					}
				}
				if isSupport {
					r.add("AG-SRC-005", Proven, path.Join(dir, "."), profile, from.Name, id, "production dependency reaches test support: "+strings.Join(chain, " -> "))
					continue
				}
				queue = append(queue, hop{id, chain})
			}
		}
	}
}
