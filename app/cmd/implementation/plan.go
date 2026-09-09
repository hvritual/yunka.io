package implementation

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
	"golang.org/x/mod/module"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
)

type Options struct {
	Project            projectflow.Options
	Application        string
	CompositionPackage string
}
type File struct {
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
	Action  string `json:"action"`
	Content string `json:"content"`
}
type Report struct {
	SchemaVersion      int    `json:"schemaVersion"`
	Mode               string `json:"mode"`
	Application        string `json:"application"`
	Contract           string `json:"contract"`
	ContractFile       string `json:"contractFile"`
	ContractSHA256     string `json:"contractSha256"`
	CompositionPackage string `json:"compositionPackage"`
	Files              []File `json:"files"`
}

// Run always recompiles canonical inputs; it never accepts a user-supplied plan
// as mutation authority. Default plan is read-only. Apply preflights every target
// and then uses exclusive writes within an os.Root. Existing content is preserved.
func Run(ctx context.Context, options Options, apply bool) (Report, error) {
	if ctx == nil {
		return Report{}, fmt.Errorf("add implementation: context is required")
	}
	if _, _, err := applicationParts(options.Application); err != nil {
		return Report{}, err
	}
	if module.CheckImportPath(options.CompositionPackage) != nil {
		return Report{}, fmt.Errorf("add implementation: an exact composition-package is required")
	}
	snapshot, err := projectflow.DescribeContractSourceSnapshot(ctx, options.Project)
	if err != nil {
		return Report{}, err
	}
	for _, diagnostic := range contract.Lint(snapshot.Manifest) {
		if strings.EqualFold(string(diagnostic.Severity), "error") {
			return Report{}, fmt.Errorf("add implementation: contract lint %s: %s", diagnostic.Path, diagnostic.Message)
		}
	}
	report, err := render(snapshot.Project, snapshot.Manifest, options.Application, options.CompositionPackage)
	if err != nil {
		return Report{}, err
	}
	var targets []string
	for _, f := range report.Files {
		targets = append(targets, f.Path)
	}
	decisions, err := ownership.Build(snapshot.Project.Root, targets)
	if err != nil {
		return Report{}, err
	}
	for _, d := range decisions.Decisions {
		if !d.SafeAutoEdit || d.Owner != "developer-code" {
			return Report{}, fmt.Errorf("add implementation: target %s is not developer-owned: %s", d.Path, d.Reason)
		}
	}
	root, err := os.OpenRoot(snapshot.Project.Root)
	if err != nil {
		return Report{}, err
	}
	defer root.Close()
	if err = preflight(root, &report); err != nil {
		return Report{}, err
	}
	if err = ctx.Err(); err != nil {
		return Report{}, err
	}
	if !apply {
		return report, nil
	}
	report.Mode = "applied"
	for i := range report.Files {
		f := &report.Files[i]
		if f.Action == "unchanged" {
			continue
		}
		if err = ctx.Err(); err != nil {
			return report, err
		}
		// The preflight is not a workspace lock. Exclusive writes prevent overwriting
		// a file created concurrently, and os.Root prevents escaping the chosen tree.
		if err = root.MkdirAll(filepath.FromSlash(path.Dir(f.Path)), 0750); err != nil {
			return report, err
		}
		out, e := root.OpenFile(filepath.FromSlash(f.Path), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0640)
		if e != nil {
			return report, fmt.Errorf("add implementation: exclusive create %s: %w", f.Path, e)
		}
		_, writeErr := out.Write([]byte(f.Content))
		closeErr := out.Close()
		if writeErr != nil {
			return report, writeErr
		}
		if closeErr != nil {
			return report, closeErr
		}
		f.Action = "created"
	}
	return report, nil
}
func digest(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func applicationParts(key string) (string, string, error) {
	parts := strings.Split(key, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("add implementation: expected domain/application")
	}
	for _, part := range parts {
		if part == "" || part == "internal" || part == "vendor" {
			return "", "", fmt.Errorf("add implementation: unsupported or reserved template path segment %q", part)
		}
		for i, r := range part {
			if (r >= 'a' && r <= 'z') || (i > 0 && ((r >= '0' && r <= '9') || r == '_' || r == '-')) {
				continue
			}
			return "", "", fmt.Errorf("add implementation: invalid template path segment %q", part)
		}
	}
	return parts[0], parts[1], nil
}
func render(project projectflow.ProjectDescriptor, manifest contract.Manifest, key, caller string) (Report, error) {
	domain, app, err := applicationParts(key)
	if err != nil {
		return Report{}, err
	}
	if module.CheckImportPath(project.GoModule) != nil || project.GeneratedGoRoot == "" || !filepath.IsLocal(project.GeneratedGoRoot) || project.GeneratedGoRoot == "." {
		return Report{}, fmt.Errorf("add implementation: canonical module and a contained generated Go root are required")
	}
	if module.CheckImportPath(caller) != nil || !(caller == project.GoModule || strings.HasPrefix(caller, project.GoModule+"/")) {
		return Report{}, fmt.Errorf("add implementation: composition package must belong to the current module")
	}
	// A custom import root must resolve to the same physical module directory. Do
	// not invent an alternate source/import mapping that Go itself cannot build.
	if project.GeneratedGoImport != project.GoModule+"/"+filepath.ToSlash(project.GeneratedGoRoot) {
		return Report{}, fmt.Errorf("add implementation: generated import root must match its current module path")
	}
	manifest.Normalize()
	var selected *contract.Service
	for i := range manifest.Services {
		s := &manifest.Services[i]
		if s.Application != nil && s.Domain == domain && s.Application.Name == app {
			if selected != nil {
				return Report{}, fmt.Errorf("add implementation: duplicate Application identity")
			}
			selected = s
		}
	}
	if selected == nil {
		return Report{}, fmt.Errorf("add implementation: Application %s not found in current canonical inputs", key)
	}
	if len(selected.Application.Requires) > 0 || len(selected.Application.Capabilities) > 0 {
		return Report{}, fmt.Errorf("add implementation: composed/infrastructure-dependent Applications require the later typed-dependency template; refusing to omit dependencies")
	}
	for _, m := range selected.Methods {
		if m.Operation == nil || len(m.Operation.RequiresOperations) > 0 || (m.Operation.Composition != "" && m.Operation.Composition != "none") {
			return Report{}, fmt.Errorf("add implementation: unsupported composed Operation %s", m.Name)
		}
	}
	for _, o := range selected.Application.Operations {
		if len(o.RequiresOperations) > 0 || (o.Composition != "" && o.Composition != "none") {
			return Report{}, fmt.Errorf("add implementation: unsupported composed Operation %s", o.ID)
		}
	}
	// Use the existing compiler/renderer's exact artifacts; do not reimplement its
	// single/multi-Application interface naming or PB Go type resolution.
	generated, err := contract.RenderC9ApplicationCode(manifest, contract.ApplicationCodeOptions{RootImport: project.GeneratedGoImport})
	if err != nil {
		return Report{}, err
	}
	port, err := selectPort(generated, *selected)
	if err != nil {
		return Report{}, err
	}
	directory := path.Join(project.GeneratedGoRoot, domain, "application", app)
	ownerImport := project.GoModule + "/" + directory
	if caller == ownerImport || strings.HasPrefix(caller, ownerImport+"/") {
		return Report{}, fmt.Errorf("add implementation: composition must be outside the new owner implementation")
	}
	contents, err := starterFiles(port, ownerImport, project.GeneratedGoImport+"/"+domain+"/application", key, caller)
	if err != nil {
		return Report{}, err
	}
	report := Report{SchemaVersion: 1, Mode: "plan", Application: key, Contract: project.GeneratedGoImport + "/" + domain + "/application." + port.name, ContractFile: path.Join(project.GeneratedGoRoot, port.path), ContractSHA256: digest(port.source), CompositionPackage: caller, Files: []File{}}
	seen := map[string]bool{}
	for name, data := range contents {
		if name == "README.md" {
			data = []byte(strings.ReplaceAll(string(data), "<owner-path>", directory))
		}
		relative := path.Join(directory, name)
		folded := strings.ToLower(relative)
		if seen[folded] {
			return Report{}, fmt.Errorf("add implementation: case-insensitive destination collision %s", relative)
		}
		seen[folded] = true
		report.Files = append(report.Files, File{Path: relative, SHA256: digest(data), Action: "create", Content: string(data)})
	}
	sort.Slice(report.Files, func(i, j int) bool { return report.Files[i].Path < report.Files[j].Path })
	return report, nil
}

func preflight(root *os.Root, report *Report) error {
	for i := range report.Files {
		f := &report.Files[i]
		if !filepath.IsLocal(f.Path) || filepath.ToSlash(filepath.Clean(f.Path)) != f.Path {
			return fmt.Errorf("add implementation: invalid target path %q", f.Path)
		}
		parts := strings.Split(f.Path, "/")
		for j := range parts {
			relative := filepath.Join(parts[:j+1]...)
			info, err := root.Lstat(relative)
			if os.IsNotExist(err) {
				break
			}
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("add implementation: symlink target or ancestor %s", relative)
			}
			if j < len(parts)-1 && !info.IsDir() {
				return fmt.Errorf("add implementation: non-directory ancestor %s", relative)
			}
			if j == len(parts)-1 {
				if !info.Mode().IsRegular() {
					return fmt.Errorf("add implementation: non-regular target %s", relative)
				}
				data, err := root.ReadFile(relative)
				if err != nil {
					return err
				}
				if string(data) != f.Content {
					return fmt.Errorf("add implementation: existing developer file differs; preserve it and edit explicitly: %s", f.Path)
				}
				f.Action = "unchanged"
			}
		}
	}
	return nil
}
