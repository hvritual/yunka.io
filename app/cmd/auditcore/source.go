package auditcore

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type GoSourceFile struct {
	Path      string   `json:"path"`
	Package   string   `json:"package"`
	Test      bool     `json:"test"`
	Generated bool     `json:"generated"`
	Imports   []string `json:"imports"`
}

type SourceSnapshot struct {
	SourceRoot string         `json:"sourceRoot"`
	Files      []GoSourceFile `json:"files"`
}

func CollectGoSource(projectRoot, sourceRoot string) (SourceSnapshot, error) {
	project, source, relativeRoot, err := resolveSourceRoot(projectRoot, sourceRoot)
	if err != nil {
		return SourceSnapshot{}, err
	}
	snapshot := SourceSnapshot{SourceRoot: relativeRoot, Files: []GoSourceFile{}}
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return SourceSnapshot{}, fmt.Errorf("audit source: stat %s: %w", relativeRoot, err)
	}
	if !info.IsDir() {
		return SourceSnapshot{}, fmt.Errorf("audit source: %s is not a directory", relativeRoot)
	}

	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != source && entry.IsDir() && excludedSourceDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.EqualFold(filepath.Ext(entry.Name()), ".go") {
			return nil
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly|parser.ParseComments)
		if parseErr != nil {
			relative, _ := filepath.Rel(project, path)
			return fmt.Errorf("audit source: parse %s: %w", filepath.ToSlash(relative), parseErr)
		}
		relative, relErr := filepath.Rel(project, path)
		if relErr != nil || outsideRoot(relative) {
			return fmt.Errorf("audit source: file %s escaped project root", path)
		}
		imports, importErr := importPaths(file)
		if importErr != nil {
			return fmt.Errorf("audit source: imports %s: %w", filepath.ToSlash(relative), importErr)
		}
		snapshot.Files = append(snapshot.Files, GoSourceFile{
			Path:      filepath.ToSlash(relative),
			Package:   strings.TrimSpace(file.Name.Name),
			Test:      strings.HasSuffix(strings.ToLower(entry.Name()), "_test.go"),
			Generated: ast.IsGenerated(file),
			Imports:   imports,
		})
		return nil
	})
	if err != nil {
		return SourceSnapshot{}, err
	}
	NormalizeSource(&snapshot)
	return snapshot, nil
}

func NormalizeSource(snapshot *SourceSnapshot) {
	if snapshot == nil {
		return
	}
	snapshot.SourceRoot = cleanSlash(snapshot.SourceRoot)
	for index := range snapshot.Files {
		file := &snapshot.Files[index]
		file.Path = cleanSlash(file.Path)
		file.Package = strings.TrimSpace(file.Package)
		file.Imports = uniqueStrings(file.Imports)
	}
	sort.Slice(snapshot.Files, func(i, j int) bool {
		if snapshot.Files[i].Path != snapshot.Files[j].Path {
			return snapshot.Files[i].Path < snapshot.Files[j].Path
		}
		return snapshot.Files[i].Package < snapshot.Files[j].Package
	})
	if snapshot.Files == nil {
		snapshot.Files = []GoSourceFile{}
	}
}

func resolveSourceRoot(projectRoot, sourceRoot string) (string, string, string, error) {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		projectRoot = "."
	}
	project, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", "", "", fmt.Errorf("audit source: project root: %w", err)
	}
	sourceRoot = strings.TrimSpace(sourceRoot)
	if sourceRoot == "" {
		sourceRoot = "."
	}
	source := sourceRoot
	if !filepath.IsAbs(source) {
		source = filepath.Join(project, filepath.FromSlash(source))
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", "", "", fmt.Errorf("audit source: source root: %w", err)
	}
	relative, err := filepath.Rel(project, source)
	if err != nil || outsideRoot(relative) {
		return "", "", "", fmt.Errorf("audit source: source root %s is outside project root", sourceRoot)
	}
	return project, source, cleanSlash(relative), nil
}

func importPaths(file *ast.File) ([]string, error) {
	if file == nil {
		return []string{}, nil
	}
	values := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		if spec == nil || spec.Path == nil {
			continue
		}
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return uniqueStrings(values), nil
}

func excludedSourceDirectory(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case ".git", ".yunka", "vendor":
		return true
	default:
		return false
	}
}

func outsideRoot(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func cleanSlash(value string) string {
	value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
	if value == "." {
		return "."
	}
	return strings.TrimPrefix(value, "./")
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	if result == nil {
		return []string{}
	}
	return result
}
