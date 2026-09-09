package sourceaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/modfile"
)

const maxInputBytes int64 = 512 << 20
const maxInputFiles = 100000

type inputFile struct {
	data       []byte
	hash       string
	executable bool
}
type snapshot struct {
	files  map[string]inputFile
	names  []string
	bytes  int64
	digest string
}

func hash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func scan(ctx context.Context, root string) (snapshot, error) {
	s := snapshot{files: map[string]inputFile{}}
	err := filepath.WalkDir(root, func(name string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("cannot enumerate source")
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.Name() == ".git" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("cannot stat %s", rel)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular source is unsupported: %s", rel)
		}
		if len(s.files) >= maxInputFiles || info.Size() > 128<<20 || s.bytes+info.Size() > maxInputBytes {
			return fmt.Errorf("source snapshot exceeds file/byte budget")
		}
		b, err := os.ReadFile(name)
		if err != nil {
			return fmt.Errorf("cannot read %s", rel)
		}
		if int64(len(b)) != info.Size() {
			return fmt.Errorf("source changed while reading %s", rel)
		}
		s.files[rel] = inputFile{data: b, hash: hash(b), executable: info.Mode()&0111 != 0}
		s.names = append(s.names, rel)
		s.bytes += int64(len(b))
		return nil
	})
	sort.Strings(s.names)
	h := sha256.New()
	for _, n := range s.names {
		f := s.files[n]
		fmt.Fprintf(h, "%s\x00%t\x00%s\n", n, f.executable, f.hash)
	}
	s.digest = hex.EncodeToString(h.Sum(nil))
	return s, err
}

func manifest(name string) bool {
	switch path.Base(name) {
	case "go.mod", "go.sum", "go.work", "go.work.sum":
		return true
	}
	return false
}

type topology struct {
	modules     map[string]string // physical directory -> canonical module declaration
	moduleFiles map[string]*modfile.File
	workFiles   map[string]*modfile.WorkFile
}

func inventory(root string, s snapshot, p Policy, r *Report) topology {
	t := topology{modules: map[string]string{}, moduleFiles: map[string]*modfile.File{}, workFiles: map[string]*modfile.WorkFile{}}
	r.Inventory = Inventory{Complete: true, Files: len(s.names), Bytes: s.bytes, Digest: s.digest}
	declared := map[string]ModulePolicy{}
	for _, m := range p.Modules {
		declared[m.Path] = m
	}
	for _, c := range p.Components {
		found := c.Path == "."
		for _, n := range s.names {
			if strings.HasPrefix(n, c.Path+"/") {
				found = true
				break
			}
		}
		if _, isFile := s.files[c.Path]; isFile || !found {
			r.add("AG-SRC-002", Unknown, c.Path, "", c.Name, "", "component must name an existing source directory, not a file or absent path")
		}
	}
	for _, x := range p.Exclusions {
		found, hasModule := false, false
		for _, n := range s.names {
			if under(n, x.Path) {
				found = true
				if path.Base(n) == "go.mod" {
					hasModule = true
				}
			}
		}
		if !found || (x.Kind == "dependency" && !hasModule) {
			r.add("AG-SRC-001", Unknown, x.Path, "", "", "", "exclusion must bind existing input; dependency exclusions require a module subtree")
		}
	}
	for _, n := range s.names {
		if strings.HasSuffix(n, ".go") {
			r.Inventory.GoFiles++
		}
		if manifest(n) {
			r.Inventory.Manifests++
		}
		x := p.exclusion(n)
		if path.Base(n) == "go.mod" {
			dir := path.Dir(n)
			m := Module{Path: dir, Disposition: "declared"}
			if declared[dir].Kind == "manifest-only" {
				m.Disposition = "manifest-only"
			}
			if x != nil {
				m.Disposition = "excluded-" + x.Kind
			}
			if x == nil || x.Kind == "dependency" {
				f, err := modfile.Parse(n, s.files[n].data, nil)
				if err != nil || f.Module == nil || f.Module.Mod.Path == "" {
					r.add("AG-SRC-001", Unknown, n, "", "", "", "invalid module manifest")
				} else {
					m.Identity = f.Module.Mod.Path
					t.modules[dir] = m.Identity
					t.moduleFiles[n] = f
				}
			}
			if _, ok := declared[dir]; !ok && x == nil {
				m.Disposition = "undeclared"
				r.add("AG-SRC-001", Unknown, n, "", "", "", "discovered module has no declared build matrix")
			}
			r.Modules = append(r.Modules, m)
		}
		if path.Base(n) == "go.work" && (x == nil || x.Kind == "dependency") {
			w, err := modfile.ParseWork(n, s.files[n].data, nil)
			if err != nil {
				r.add("AG-SRC-001", Unknown, n, "", "", "", "invalid workspace manifest")
			} else {
				t.workFiles[n] = w
			}
		}
	}
	identities := map[string]string{}
	for _, m := range p.Modules {
		if identity := t.modules[m.Path]; identity != "" {
			if previous, exists := identities[identity]; exists && previous != m.Path {
				r.add("AG-SRC-001", Unknown, path.Join(m.Path, "go.mod"), "", identity, "", "owned module identity is duplicated")
			}
			identities[identity] = m.Path
		}
		if _, ok := t.modules[m.Path]; !ok {
			r.add("AG-SRC-001", Unknown, path.Join(m.Path, "go.mod"), "", "", "", "declared module manifest is missing or invalid")
		}
		if m.Workspace != "off" {
			w := t.workFiles[m.Workspace]
			included := false
			if w != nil {
				for _, u := range w.Use {
					target, err := localTarget(root, path.Dir(m.Workspace), u.Path)
					if err == nil && target == m.Path {
						included = true
					}
				}
			}
			if !included {
				r.add("AG-SRC-008", Unknown, m.Workspace, "", "", "", "selected workspace does not include declared module "+m.Path)
			}
		}
	}
	for file, f := range t.moduleFiles {
		for _, v := range f.Replace {
			t.reference(root, s, p, r, file, "replace", v.Old.Path, v.Old.Version, v.New.Path, v.New.Version)
		}
	}
	for file, w := range t.workFiles {
		for _, u := range w.Use {
			t.reference(root, s, p, r, file, "use", "", "", u.Path, "")
		}
		for _, v := range w.Replace {
			t.reference(root, s, p, r, file, "replace", v.Old.Path, v.Old.Version, v.New.Path, v.New.Version)
		}
	}
	for _, n := range s.names {
		isGo := strings.HasSuffix(n, ".go")
		if !isGo && !manifest(n) {
			continue
		}
		f := File{Path: n, SHA256: s.files[n].hash, Kind: "manifest", Disposition: "inventoried"}
		if isGo {
			f.Kind = "production"
			if strings.HasSuffix(n, "_test.go") {
				f.Kind = "test"
			}
			f.Disposition = "uncovered"
		}
		if x := p.exclusion(n); x != nil {
			f.Disposition = "excluded"
			f.Reason = x.Kind + ": " + x.Reason
			if isGo {
				f.Kind = x.Kind
			}
		} else if isGo {
			f.Module = t.owner(path.Dir(n))
			if declared[f.Module].Kind == "manifest-only" {
				r.add("AG-SRC-001", Unknown, n, "", "", "", "Go source was added to a manifest-only module")
			}
			if f.Module == "" {
				r.add("AG-SRC-001", Unknown, n, "", "", "", "Go source is not owned by an inventoried module")
			}
			if c := p.component(n); c != nil {
				f.Component = c.Name
			} else {
				r.add("AG-SRC-002", Unknown, n, "", "", "", "Go source has no component policy")
			}
		}
		r.Files = append(r.Files, f)
	}
	if r.Inventory.GoFiles == 0 {
		r.add("AG-SRC-003", Unknown, "", "", "", "", "repository has no inventoried Go source")
	}
	return t
}

func (t topology) owner(dir string) string {
	best := ""
	for m := range t.modules {
		if under(dir, m) && (best == "" || best == "." || len(m) > len(best)) {
			best = m
		}
	}
	return best
}

func localTarget(root, base, target string) (string, error) {
	actual := target
	if !filepath.IsAbs(actual) {
		actual = filepath.Join(root, filepath.FromSlash(base), filepath.FromSlash(target))
	}
	rel, err := filepath.Rel(root, filepath.Clean(actual))
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if !relative(rel, true) {
		return "", fmt.Errorf("local module/workspace target escapes the selected root")
	}
	return rel, nil
}

func (t topology) reference(root string, s snapshot, p Policy, r *Report, file, kind, module, version, target, targetVersion string) {
	ref := LocalReference{File: file, Kind: kind, Module: module, Version: version, Target: target, TargetVersion: targetVersion, Disposition: "versioned-dependency"}
	if targetVersion == "" {
		dir, err := localTarget(root, path.Dir(file), target)
		if err != nil {
			ref.Target = "<outside-root>"
			ref.Disposition = "unresolved"
			r.add("AG-SRC-008", Unknown, file, "", module, "", "local module/workspace target is outside the selected root; select a root containing all local inputs")
		} else {
			ref.Target = dir
			ref.Disposition = "owned-module"
			if _, ok := s.files[path.Join(dir, "go.mod")]; !ok {
				ref.Disposition = "unresolved"
				r.add("AG-SRC-008", Unknown, file, "", module, dir, "local target lacks an inventoried go.mod")
			} else if x := p.exclusion(dir); x != nil {
				ref.Disposition = "excluded-" + x.Kind
				if x.Kind == "fixture" {
					r.add("AG-SRC-008", Unknown, file, "", module, dir, "local manifest selects a fixture-only module whose dependency context is not qualified")
				}
			}
		}
	}
	r.References = append(r.References, ref)
}

// The Go tool reads only this private projection. Absolute, root-contained local
// directives are rebased with Go's own manifest editor; original bytes are hashed
// above and never modified. A second snapshot detects any tool-side mutation.
func materialize(root, tmp string, s snapshot, t topology) (snapshot, error) {
	for _, n := range s.names {
		f := s.files[n]
		data := f.data
		if m := t.moduleFiles[n]; m != nil {
			changed := false
			for _, v := range append([]*modfile.Replace(nil), m.Replace...) {
				if v.New.Version == "" && filepath.IsAbs(v.New.Path) {
					dir, err := localTarget(root, path.Dir(n), v.New.Path)
					if err != nil {
						return snapshot{}, err
					}
					if err = m.AddReplace(v.Old.Path, v.Old.Version, filepath.Join(tmp, filepath.FromSlash(dir)), ""); err != nil {
						return snapshot{}, err
					}
					changed = true
				}
			}
			if changed {
				var err error
				data, err = m.Format()
				if err != nil {
					return snapshot{}, err
				}
			}
		}
		if w := t.workFiles[n]; w != nil {
			changed := false
			for _, u := range append([]*modfile.Use(nil), w.Use...) {
				if filepath.IsAbs(u.Path) {
					dir, err := localTarget(root, path.Dir(n), u.Path)
					if err != nil {
						return snapshot{}, err
					}
					if err = w.DropUse(u.Path); err != nil {
						return snapshot{}, err
					}
					if err = w.AddUse(filepath.Join(tmp, filepath.FromSlash(dir)), ""); err != nil {
						return snapshot{}, err
					}
					changed = true
				}
			}
			for _, v := range append([]*modfile.Replace(nil), w.Replace...) {
				if v.New.Version == "" && filepath.IsAbs(v.New.Path) {
					dir, err := localTarget(root, path.Dir(n), v.New.Path)
					if err != nil {
						return snapshot{}, err
					}
					if err = w.AddReplace(v.Old.Path, v.Old.Version, filepath.Join(tmp, filepath.FromSlash(dir)), ""); err != nil {
						return snapshot{}, err
					}
					changed = true
				}
			}
			if changed {
				w.Cleanup()
				data = modfile.Format(w.Syntax)
			}
		}
		name := filepath.Join(tmp, filepath.FromSlash(n))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			return snapshot{}, err
		}
		mode := os.FileMode(0600)
		if f.executable {
			mode = 0700
		}
		if err := os.WriteFile(name, data, mode); err != nil {
			return snapshot{}, err
		}
	}
	return scan(context.Background(), tmp)
}
