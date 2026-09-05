package moduleidentity

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const SchemaVersion = 1

type FindingKind string

const (
	FindingGoImport FindingKind = "go_import"
	FindingGoMod    FindingKind = "go_mod"
	FindingGoWork   FindingKind = "go_work"
)

type Mapping struct {
	Legacy    string `json:"legacy"`
	Canonical string `json:"canonical"`
}

var mappings = []Mapping{
	{Legacy: "yunka.io/framework", Canonical: "github.com/hvritual/yunka.io/framework"},
	{Legacy: "yunka.io/gateway", Canonical: "github.com/hvritual/yunka.io/gateway"},
	{Legacy: "yunka.io/pkg", Canonical: "github.com/hvritual/yunka.io/pkg"},
}

type Finding struct {
	Kind      FindingKind `json:"kind"`
	Path      string      `json:"path"`
	Line      int         `json:"line,omitempty"`
	Legacy    string      `json:"legacy"`
	Canonical string      `json:"canonical"`
}

type Report struct {
	SchemaVersion int       `json:"schemaVersion"`
	Findings      []Finding `json:"findings"`
	Conformant    bool      `json:"conformant"`
}

type MigrationResult struct {
	SchemaVersion int       `json:"schemaVersion"`
	Before        []Finding `json:"before"`
	ChangedFiles  []string  `json:"changedFiles"`
	After         []Finding `json:"after"`
	Conformant    bool      `json:"conformant"`
}

type replacement struct {
	start int
	end   int
	value string
}

func Mappings() []Mapping {
	result := make([]Mapping, len(mappings))
	copy(result, mappings)
	return result
}

func Canonicalize(value string) (string, bool) {
	value = strings.TrimSpace(value)
	for _, mapping := range mappings {
		if value == mapping.Legacy {
			return mapping.Canonical, true
		}
		if strings.HasPrefix(value, mapping.Legacy+"/") {
			return mapping.Canonical + strings.TrimPrefix(value, mapping.Legacy), true
		}
	}
	return value, false
}

func Inspect(root string) (Report, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return Report{}, err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return Report{}, err
	}
	if !info.IsDir() {
		return Report{}, fmt.Errorf("module identity: root %s is not a directory", absolute)
	}

	var findings []Finding
	err = walkProjectFiles(absolute, func(path string, entry fs.DirEntry) error {
		relative, relErr := filepath.Rel(absolute, path)
		if relErr != nil {
			return relErr
		}
		relative = filepath.ToSlash(relative)
		switch {
		case strings.HasSuffix(entry.Name(), ".go"):
			items, inspectErr := inspectGoFile(path, relative)
			if inspectErr != nil {
				return inspectErr
			}
			findings = append(findings, items...)
		case entry.Name() == "go.mod":
			items, inspectErr := inspectModuleFile(path, relative, FindingGoMod)
			if inspectErr != nil {
				return inspectErr
			}
			findings = append(findings, items...)
		case entry.Name() == "go.work":
			items, inspectErr := inspectModuleFile(path, relative, FindingGoWork)
			if inspectErr != nil {
				return inspectErr
			}
			findings = append(findings, items...)
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	findings = normalizeFindings(findings)
	if findings == nil {
		findings = []Finding{}
	}
	return Report{SchemaVersion: SchemaVersion, Findings: findings, Conformant: len(findings) == 0}, nil
}

func Migrate(root string) (MigrationResult, error) {
	absolute, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return MigrationResult{}, err
	}
	before, err := Inspect(absolute)
	if err != nil {
		return MigrationResult{}, err
	}
	changed := make([]string, 0)
	err = walkProjectFiles(absolute, func(path string, entry fs.DirEntry) error {
		var didChange bool
		var migrateErr error
		switch {
		case strings.HasSuffix(entry.Name(), ".go"):
			didChange, migrateErr = migrateGoFile(path)
		case entry.Name() == "go.mod" || entry.Name() == "go.work":
			didChange, migrateErr = migrateModuleFile(path)
		default:
			return nil
		}
		if migrateErr != nil {
			return migrateErr
		}
		if didChange {
			relative, relErr := filepath.Rel(absolute, path)
			if relErr != nil {
				return relErr
			}
			changed = append(changed, filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		return MigrationResult{}, err
	}
	sort.Strings(changed)
	after, err := Inspect(absolute)
	if err != nil {
		return MigrationResult{}, err
	}
	result := MigrationResult{
		SchemaVersion: SchemaVersion,
		Before:        before.Findings,
		ChangedFiles:  changed,
		After:         after.Findings,
		Conformant:    after.Conformant,
	}
	if result.ChangedFiles == nil {
		result.ChangedFiles = []string{}
	}
	if !result.Conformant {
		return result, fmt.Errorf("module identity migration left %d legacy reference(s)", len(result.After))
	}
	return result, nil
}

func inspectGoFile(path, relative string) ([]Finding, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, path, contents, parser.ImportsOnly|parser.AllErrors)
	if file == nil {
		return nil, nil
	}
	result := make([]Finding, 0)
	for _, item := range file.Imports {
		if item.Path == nil {
			continue
		}
		value, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			continue
		}
		canonical, changed := Canonicalize(value)
		if !changed {
			continue
		}
		line := fset.PositionFor(item.Path.Pos(), false).Line
		result = append(result, Finding{Kind: FindingGoImport, Path: relative, Line: line, Legacy: value, Canonical: canonical})
	}
	return result, nil
}

func inspectModuleFile(path, relative string, kind FindingKind) ([]Finding, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := make([]Finding, 0)
	for index, line := range strings.Split(string(contents), "\n") {
		code := line
		if comment := strings.Index(code, "//"); comment >= 0 {
			code = code[:comment]
		}
		for _, field := range strings.Fields(code) {
			field = strings.Trim(field, "()")
			canonical, changed := Canonicalize(field)
			if changed {
				result = append(result, Finding{Kind: kind, Path: relative, Line: index + 1, Legacy: field, Canonical: canonical})
			}
		}
	}
	return result, nil
}

func migrateGoFile(path string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	file, _ := parser.ParseFile(fset, path, contents, parser.ImportsOnly|parser.AllErrors)
	if file == nil {
		return false, nil
	}
	replacements := make([]replacement, 0)
	for _, item := range file.Imports {
		if item.Path == nil {
			continue
		}
		value, unquoteErr := strconv.Unquote(item.Path.Value)
		if unquoteErr != nil {
			continue
		}
		canonical, changed := Canonicalize(value)
		if !changed {
			continue
		}
		start := fset.PositionFor(item.Path.Pos(), false).Offset
		end := fset.PositionFor(item.Path.End(), false).Offset
		literal := strconv.Quote(canonical)
		if strings.HasPrefix(item.Path.Value, "`") {
			literal = "`" + canonical + "`"
		}
		replacements = append(replacements, replacement{start: start, end: end, value: literal})
	}
	if len(replacements) == 0 {
		return false, nil
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	updated := append([]byte(nil), contents...)
	for _, item := range replacements {
		if item.start < 0 || item.end < item.start || item.end > len(updated) {
			return false, fmt.Errorf("module identity: invalid import replacement bounds in %s", path)
		}
		updated = append(updated[:item.start], append([]byte(item.value), updated[item.end:]...)...)
	}
	return true, writeAtomic(path, updated)
}

func migrateModuleFile(path string) (bool, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	lines := strings.SplitAfter(string(contents), "\n")
	changed := false
	for index, line := range lines {
		newline := ""
		body := line
		if strings.HasSuffix(body, "\n") {
			newline = "\n"
			body = strings.TrimSuffix(body, "\n")
		}
		code := body
		comment := ""
		if offset := strings.Index(code, "//"); offset >= 0 {
			comment = code[offset:]
			code = code[:offset]
		}
		for _, field := range strings.Fields(code) {
			trimmed := strings.Trim(field, "()")
			canonical, rewrite := Canonicalize(trimmed)
			if !rewrite {
				continue
			}
			code = strings.ReplaceAll(code, trimmed, canonical)
			changed = true
		}
		lines[index] = code + comment + newline
	}
	if !changed {
		return false, nil
	}
	return true, writeAtomic(path, []byte(strings.Join(lines, "")))
}

func writeAtomic(path string, contents []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".yunka-module-identity-*")
	if err != nil {
		return err
	}
	tmp := temporary.Name()
	defer os.Remove(tmp)
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Chmod(info.Mode().Perm()); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func walkProjectFiles(root string, visit func(path string, entry fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			switch entry.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			default:
				return nil
			}
		}
		return visit(path, entry)
	})
}

func normalizeFindings(values []Finding) []Finding {
	seen := make(map[string]Finding, len(values))
	for _, value := range values {
		key := string(value.Kind) + "\x00" + value.Path + "\x00" + strconv.Itoa(value.Line) + "\x00" + value.Legacy + "\x00" + value.Canonical
		seen[key] = value
	}
	result := make([]Finding, 0, len(seen))
	for _, value := range seen {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		if result[i].Line != result[j].Line {
			return result[i].Line < result[j].Line
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Legacy < result[j].Legacy
	})
	return result
}
