package sourceaudit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"

	"golang.org/x/mod/module"
)

var label = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

// ReadPolicy rejects unknown fields, duplicate keys, null and trailing values.
// Explicit policy is reviewed input, not a second inventory of source facts.
func ReadPolicy(data []byte) (Policy, error) {
	var p Policy
	if len(data) == 0 || len(data) > 1<<20 {
		return p, fmt.Errorf("source policy must contain 1..1048576 bytes")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueJSON(d); err != nil {
		return p, fmt.Errorf("source policy: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return p, fmt.Errorf("source policy: trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, fmt.Errorf("source policy: %w", err)
	}
	if err := p.Validate(); err != nil {
		return p, err
	}
	return p, nil
}

func uniqueJSON(d *json.Decoder) error {
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("null values are not permitted")
	}
	x, ok := t.(json.Delim)
	if !ok {
		return nil
	}
	switch x {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			k, e := d.Token()
			if e != nil {
				return e
			}
			s, ok := k.(string)
			if !ok || seen[s] {
				return fmt.Errorf("invalid/duplicate field %q", s)
			}
			seen[s] = true
			if e = uniqueJSON(d); e != nil {
				return e
			}
		}
	case '[':
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("invalid delimiter")
	}
	_, err = d.Token()
	return err
}

func relative(p string, allowRoot bool) bool {
	if p == "." {
		return allowRoot
	}
	return p != "" && path.Clean(p) == p && p != ".." && !strings.HasPrefix(p, "../") && !strings.HasPrefix(p, "/") && !strings.ContainsAny(p, "\\\x00\r\n\t:")
}

func (p Policy) Validate() error {
	bad := func(s string) error { return fmt.Errorf("source policy: %s", s) }
	if p.SchemaVersion != SchemaVersion || len(p.Profiles) == 0 || len(p.Profiles) > 32 || len(p.Modules) == 0 || len(p.Modules) > 128 || len(p.Components) == 0 || len(p.Components) > 256 || len(p.Exclusions) > 1024 {
		return bad("schemaVersion 1 and bounded nonempty profiles/modules/components are required")
	}
	profiles := map[string]bool{}
	for _, v := range p.Profiles {
		if !label.MatchString(v.Name) || profiles[v.Name] || !label.MatchString(v.GOOS) || !label.MatchString(v.GOARCH) {
			return bad("invalid/duplicate build profile")
		}
		profiles[v.Name] = true
		seen := map[string]bool{}
		for _, t := range v.Tags {
			if !label.MatchString(t) || seen[t] {
				return bad("invalid/duplicate build tag")
			}
			seen[t] = true
		}
	}
	modules := map[string]bool{}
	usedProfiles := map[string]bool{}
	for _, m := range p.Modules {
		if !relative(m.Path, true) || modules[m.Path] || ((m.Kind == "" || m.Kind == "source") && len(m.Profiles) == 0) || (m.Kind == "manifest-only" && len(m.Profiles) != 0) || (m.Kind != "" && m.Kind != "source" && m.Kind != "manifest-only") {
			return bad("invalid/duplicate module path or empty profile set")
		}
		modules[m.Path] = true
		if m.Workspace != "off" && (!relative(m.Workspace, false) || path.Base(m.Workspace) != "go.work") {
			return bad("module workspace must be off or an exact relative go.work")
		}
		seen := map[string]bool{}
		for _, n := range m.Profiles {
			if !profiles[n] || seen[n] {
				return bad("unknown/duplicate module profile")
			}
			seen[n] = true
			usedProfiles[n] = true
		}
	}
	if len(usedProfiles) != len(profiles) {
		return bad("every declared profile must be selected by a module")
	}
	components, paths := map[string]bool{}, map[string]bool{}
	for _, c := range p.Components {
		if !label.MatchString(c.Name) || components[c.Name] || !relative(c.Path, true) || paths[c.Path] || (c.Kind != "production" && c.Kind != "test-support") {
			return bad("invalid/duplicate component name/path/kind")
		}
		components[c.Name] = true
		paths[c.Path] = true
		if !validImports(c.DenyImports) {
			return bad("invalid/duplicate denied import prefix")
		}
	}
	for _, c := range p.Components {
		seen := map[string]bool{}
		for _, n := range c.Allow {
			if !components[n] || seen[n] || n == c.Name {
				return bad("unknown/duplicate/redundant allowed component")
			}
			seen[n] = true
		}
	}
	seen := map[string]bool{}
	for _, x := range p.Exclusions {
		if !relative(x.Path, false) || seen[x.Path] || (x.Kind != "fixture" && x.Kind != "dependency") || len(strings.TrimSpace(x.Reason)) < 8 {
			return bad("each exclusion needs an exact relative path, kind and meaningful reason")
		}
		seen[x.Path] = true
		for _, y := range p.Exclusions {
			if x.Path != y.Path && under(x.Path, y.Path) {
				return bad("overlapping exclusions are not permitted")
			}
		}
		for _, m := range p.Modules {
			if under(m.Path, x.Path) {
				return bad("selected module is excluded")
			}
		}
		for _, c := range p.Components {
			if under(c.Path, x.Path) {
				return bad("selected component is excluded")
			}
		}
	}
	if !validImports(p.TestSupportImports) {
		return bad("invalid/duplicate test-support import prefix")
	}
	return nil
}

func validImports(a []string) bool {
	seen := map[string]bool{}
	for _, s := range a {
		if module.CheckImportPath(s) != nil || seen[s] {
			return false
		}
		seen[s] = true
	}
	return true
}
func under(name, prefix string) bool {
	return prefix == "." || name == prefix || strings.HasPrefix(name, prefix+"/")
}
func (p Policy) exclusion(name string) *Exclusion {
	for i := range p.Exclusions {
		if under(name, p.Exclusions[i].Path) {
			return &p.Exclusions[i]
		}
	}
	return nil
}
func (p Policy) component(name string) *Component {
	var best *Component
	for i := range p.Components {
		c := &p.Components[i]
		if under(name, c.Path) && (best == nil || (best.Path == "." && c.Path != ".") || len(c.Path) > len(best.Path)) {
			best = c
		}
	}
	return best
}
