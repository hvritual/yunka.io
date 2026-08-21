package dependencypolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const SchemaVersion = 3

type ModuleRule struct {
	Path    string `json:"path"`
	Version string `json:"version"`
}

type WorkspaceModuleRule struct {
	Path      string `json:"path"`
	Directory string `json:"directory"`
}

type LegacyImportRule struct {
	Path            string   `json:"path"`
	AllowedFiles    []string `json:"allowedFiles,omitempty"`
	AllowedPrefixes []string `json:"allowedPrefixes,omitempty"`
}

type LocalReplacementRule struct {
	Path      string   `json:"path"`
	Version   string   `json:"version"`
	Directory string   `json:"directory"`
	Files     []string `json:"files"`
}

type Policy struct {
	SchemaVersion             int                    `json:"schemaVersion"`
	ModuleFiles               []string               `json:"moduleFiles"`
	ForbiddenModules          []string               `json:"forbiddenModules"`
	RequiredModules           []ModuleRule           `json:"requiredModules"`
	WorkspaceModules          []WorkspaceModuleRule  `json:"workspaceModules"`
	ForbidExternalReplaces    bool                   `json:"forbidExternalReplaces"`
	ForbiddenReplaceModules   []string               `json:"forbiddenReplaceModules"`
	RequiredLocalReplacements []LocalReplacementRule `json:"requiredLocalReplacements"`
	LegacyImports             []LegacyImportRule     `json:"legacyImports"`
}

type Module struct {
	Path    string  `json:"Path"`
	Version string  `json:"Version"`
	Main    bool    `json:"Main"`
	Dir     string  `json:"Dir"`
	GoMod   string  `json:"GoMod"`
	Replace *Module `json:"Replace,omitempty"`
}

type Diagnostic struct {
	Path    string
	Message string
}

func Load(path string) (Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Policy{}, err
	}
	var policy Policy
	if err := json.Unmarshal(data, &policy); err != nil {
		return Policy{}, fmt.Errorf("dependency policy: decode %s: %w", path, err)
	}
	if policy.SchemaVersion != SchemaVersion {
		return Policy{}, fmt.Errorf("dependency policy: unsupported schemaVersion %d", policy.SchemaVersion)
	}
	if len(policy.ModuleFiles) == 0 {
		return Policy{}, errors.New("dependency policy: moduleFiles is required")
	}
	return policy, nil
}

func Check(ctx context.Context, repositoryRoot, policyPath, goBinary string) ([]Diagnostic, error) {
	root, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return nil, err
	}
	policy, err := Load(policyPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(goBinary) == "" {
		goBinary = "go"
	}
	modules, err := listModules(ctx, root, goBinary)
	if err != nil {
		return nil, err
	}

	diagnostics := checkModuleGraph(root, modules, policy)
	replaceDiagnostics, err := checkReplaces(root, policy)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, replaceDiagnostics...)
	importDiagnostics, err := checkLegacyImports(root, policy)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, importDiagnostics...)
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Path == diagnostics[j].Path {
			return diagnostics[i].Message < diagnostics[j].Message
		}
		return diagnostics[i].Path < diagnostics[j].Path
	})
	return diagnostics, nil
}

func listModules(ctx context.Context, root, goBinary string) ([]Module, error) {
	command := exec.CommandContext(ctx, goBinary, "list", "-m", "-json", "all")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("dependency policy: go list -m failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("dependency policy: go list -m: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	var modules []Module
	for {
		var module Module
		err := decoder.Decode(&module)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("dependency policy: decode module graph: %w", err)
		}
		modules = append(modules, module)
	}
	return modules, nil
}

func checkModuleGraph(root string, modules []Module, policy Policy) []Diagnostic {
	index := make(map[string]Module, len(modules))
	for _, module := range modules {
		index[module.Path] = module
	}
	var diagnostics []Diagnostic
	for _, path := range policy.ForbiddenModules {
		if module, ok := index[path]; ok {
			version := module.Version
			if module.Replace != nil {
				version += " replaced-by=" + module.Replace.Path + "@" + module.Replace.Version
			}
			diagnostics = append(diagnostics, Diagnostic{
				Path:    "module." + path,
				Message: "forbidden module selected" + formatVersion(version),
			})
		}
	}
	for _, required := range policy.RequiredModules {
		module, ok := index[required.Path]
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{Path: "module." + required.Path, Message: "required module is not selected"})
			continue
		}
		actual := module.Version
		if module.Replace != nil {
			actual = module.Replace.Version
		}
		if required.Version != "" && actual != required.Version {
			diagnostics = append(diagnostics, Diagnostic{
				Path:    "module." + required.Path,
				Message: fmt.Sprintf("version=%s want=%s", actual, required.Version),
			})
		}
	}
	for _, required := range policy.WorkspaceModules {
		module, ok := index[required.Path]
		if !ok {
			diagnostics = append(diagnostics, Diagnostic{Path: "module." + required.Path, Message: "required workspace compatibility module is not selected"})
			continue
		}
		expectedDir, err := containedPath(root, required.Directory)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Path: "module." + required.Path, Message: err.Error()})
			continue
		}
		expectedDir, _ = filepath.Abs(expectedDir)
		actualDir, _ := filepath.Abs(module.Dir)
		switch {
		case !module.Main:
			diagnostics = append(diagnostics, Diagnostic{Path: "module." + required.Path, Message: "compatibility module must be a workspace main module"})
		case module.Replace != nil:
			diagnostics = append(diagnostics, Diagnostic{Path: "module." + required.Path, Message: "compatibility module must not be supplied through replace"})
		case filepath.Clean(actualDir) != filepath.Clean(expectedDir):
			diagnostics = append(diagnostics, Diagnostic{
				Path:    "module." + required.Path,
				Message: fmt.Sprintf("directory=%s want=%s", filepath.ToSlash(actualDir), filepath.ToSlash(expectedDir)),
			})
		case filepath.Clean(module.GoMod) != filepath.Join(filepath.Clean(expectedDir), "go.mod"):
			diagnostics = append(diagnostics, Diagnostic{Path: "module." + required.Path, Message: "compatibility module go.mod is not repository-owned"})
		}
	}
	return diagnostics
}

func checkReplaces(root string, policy Policy) ([]Diagnostic, error) {
	type replacement struct {
		modulePath    string
		moduleVersion string
		targetPath    string
	}

	byFile := make(map[string][]replacement, len(policy.ModuleFiles))
	var diagnostics []Diagnostic
	for _, relative := range policy.ModuleFiles {
		path, err := containedPath(root, relative)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(data), "\n")
		for lineNumber, line := range lines {
			if comment := strings.Index(line, "//"); comment >= 0 {
				line = line[:comment]
			}
			modulePath, moduleVersion, targetPath, ok := parseReplacement(line)
			if !ok {
				continue
			}
			byFile[filepath.ToSlash(relative)] = append(byFile[filepath.ToSlash(relative)], replacement{
				modulePath: modulePath, moduleVersion: moduleVersion, targetPath: targetPath,
			})
			if policy.ForbidExternalReplaces && !isLocalReplacementTarget(targetPath) {
				diagnostics = append(diagnostics, Diagnostic{
					Path:    fmt.Sprintf("%s:%d", filepath.ToSlash(relative), lineNumber+1),
					Message: "external dependency replace is forbidden: " + modulePath + " => " + targetPath,
				})
			}
			for _, module := range policy.ForbiddenReplaceModules {
				if modulePath == module {
					diagnostics = append(diagnostics, Diagnostic{
						Path:    fmt.Sprintf("%s:%d", filepath.ToSlash(relative), lineNumber+1),
						Message: "forbidden dependency replace for " + module,
					})
				}
			}
		}
	}

	for _, rule := range policy.RequiredLocalReplacements {
		expectedDir, err := containedPath(root, rule.Directory)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Path: "replace." + rule.Path, Message: err.Error()})
			continue
		}
		expectedDir, _ = filepath.Abs(expectedDir)
		for _, relative := range rule.Files {
			relative = filepath.ToSlash(relative)
			moduleFile, err := containedPath(root, relative)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{Path: relative, Message: err.Error()})
				continue
			}
			matches := 0
			for _, candidate := range byFile[relative] {
				if candidate.modulePath != rule.Path || candidate.moduleVersion != rule.Version {
					continue
				}
				matches++
				if !isLocalReplacementTarget(candidate.targetPath) {
					diagnostics = append(diagnostics, Diagnostic{Path: relative, Message: "required compatibility replacement target is not local"})
					continue
				}
				actualDir := filepath.Clean(filepath.Join(filepath.Dir(moduleFile), filepath.FromSlash(candidate.targetPath)))
				actualDir, _ = filepath.Abs(actualDir)
				if filepath.Clean(actualDir) != filepath.Clean(expectedDir) {
					diagnostics = append(diagnostics, Diagnostic{
						Path:    relative,
						Message: fmt.Sprintf("replacement %s@%s target=%s want=%s", rule.Path, rule.Version, filepath.ToSlash(actualDir), filepath.ToSlash(expectedDir)),
					})
				}
			}
			switch {
			case matches == 0:
				diagnostics = append(diagnostics, Diagnostic{Path: relative, Message: fmt.Sprintf("required local replacement missing: %s %s", rule.Path, rule.Version)})
			case matches > 1:
				diagnostics = append(diagnostics, Diagnostic{Path: relative, Message: fmt.Sprintf("required local replacement appears %d times: %s %s", matches, rule.Path, rule.Version)})
			}
		}
	}
	return diagnostics, nil
}

func parseReplacement(line string) (modulePath, moduleVersion, targetPath string, ok bool) {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "replace ")
	fields := strings.Fields(line)
	arrow := -1
	for index, field := range fields {
		if field == "=>" {
			arrow = index
			break
		}
	}
	if arrow < 1 || arrow+1 >= len(fields) {
		return "", "", "", false
	}
	modulePath = fields[0]
	if arrow == 2 {
		moduleVersion = fields[1]
	} else if arrow != 1 {
		return "", "", "", false
	}
	return modulePath, moduleVersion, fields[arrow+1], true
}

func isLocalReplacementTarget(target string) bool {
	return target == "." || strings.HasPrefix(target, "./") || strings.HasPrefix(target, "../")
}

func checkLegacyImports(root string, policy Policy) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && skipDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("dependency policy: parse %s: %w", relative, err)
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("dependency policy: parse import %s: %w", relative, err)
			}
			for _, rule := range policy.LegacyImports {
				if importPath != rule.Path && !strings.HasPrefix(importPath, rule.Path+"/") {
					continue
				}
				if !legacyImportAllowed(relative, rule) {
					diagnostics = append(diagnostics, Diagnostic{
						Path:    relative,
						Message: "legacy import outside approved compatibility island: " + importPath,
					})
				}
			}
		}
		return nil
	})
	return diagnostics, err
}

func legacyImportAllowed(relative string, rule LegacyImportRule) bool {
	for _, allowed := range rule.AllowedFiles {
		if relative == filepath.ToSlash(allowed) {
			return true
		}
	}
	for _, prefix := range rule.AllowedPrefixes {
		prefix = strings.TrimSuffix(filepath.ToSlash(prefix), "/") + "/"
		if strings.HasPrefix(relative, prefix) {
			return true
		}
	}
	return false
}

func skipDirectory(name string) bool {
	switch name {
	case ".git", ".yunka", "node_modules", "vendor", "var", "patch-workspaces":
		return true
	default:
		return false
	}
}

func containedPath(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("dependency policy: absolute module file path %q", relative)
	}
	path := filepath.Clean(filepath.Join(root, relative))
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("dependency policy: path escapes repository: %q", relative)
	}
	return path, nil
}

func formatVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return ""
	}
	return " (" + version + ")"
}
