package auditcore

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func ResolveGitCommit(root, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("audit debt: base Git ref is required")
	}
	output, err := runGitBytes(root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("audit debt: resolve base %q: %w", ref, err)
	}
	sha := strings.TrimSpace(string(output))
	if sha == "" {
		return "", fmt.Errorf("audit debt: Git ref %q resolved to an empty commit", ref)
	}
	return sha, nil
}

func ReadGitFileAtCommit(root, commitSHA, path string) ([]byte, error) {
	commitSHA = strings.TrimSpace(commitSHA)
	path = cleanSlash(path)
	if commitSHA == "" || path == "" || path == "." || outsideSlashRoot(path) {
		return nil, fmt.Errorf("audit debt: commit SHA and project-relative file path are required")
	}
	output, err := runGitBytes(root, "show", commitSHA+":"+path)
	if err != nil {
		return nil, fmt.Errorf("audit debt: read %s at %s: %w", path, commitSHA, err)
	}
	return output, nil
}

func CollectGoSourceAtCommit(projectRoot, sourceRoot, commitSHA string) (SourceSnapshot, error) {
	_, _, relativeRoot, err := resolveSourceRoot(projectRoot, sourceRoot)
	if err != nil {
		return SourceSnapshot{}, err
	}
	commitSHA = strings.TrimSpace(commitSHA)
	if commitSHA == "" {
		return SourceSnapshot{}, fmt.Errorf("audit debt: base commit SHA is required")
	}

	args := []string{"ls-tree", "-r", "-z", "--full-tree", commitSHA, "--"}
	if relativeRoot != "." {
		args = append(args, relativeRoot)
	}
	output, err := runGitBytes(projectRoot, args...)
	if err != nil {
		return SourceSnapshot{}, fmt.Errorf("audit debt: list source at %s: %w", commitSHA, err)
	}

	snapshot := SourceSnapshot{SourceRoot: relativeRoot, Files: []GoSourceFile{}}
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		entry := string(raw)
		tab := strings.IndexByte(entry, '\t')
		if tab < 0 {
			return SourceSnapshot{}, fmt.Errorf("audit debt: malformed git ls-tree entry %q", entry)
		}
		meta, filePath := entry[:tab], cleanSlash(entry[tab+1:])
		fields := strings.Fields(meta)
		if len(fields) < 3 || fields[1] != "blob" || fields[0] == "120000" {
			continue
		}
		if !strings.EqualFold(filepath.Ext(filePath), ".go") || excludedGitSourcePath(filePath, relativeRoot) {
			continue
		}
		contents, err := ReadGitFileAtCommit(projectRoot, commitSHA, filePath)
		if err != nil {
			return SourceSnapshot{}, err
		}
		file, err := parseGoSourceBytes(filePath, contents)
		if err != nil {
			return SourceSnapshot{}, err
		}
		snapshot.Files = append(snapshot.Files, file)
	}
	NormalizeSource(&snapshot)
	return snapshot, nil
}

func parseGoSourceBytes(path string, contents []byte) (GoSourceFile, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, contents, parser.ImportsOnly|parser.ParseComments)
	if err != nil {
		return GoSourceFile{}, fmt.Errorf("audit debt: parse %s: %w", path, err)
	}
	imports, err := importPaths(file)
	if err != nil {
		return GoSourceFile{}, fmt.Errorf("audit debt: imports %s: %w", path, err)
	}
	return GoSourceFile{
		Path:      cleanSlash(path),
		Package:   strings.TrimSpace(file.Name.Name),
		Test:      strings.HasSuffix(strings.ToLower(path), "_test.go"),
		Generated: ast.IsGenerated(file),
		Imports:   imports,
	}, nil
}

func excludedGitSourcePath(filePath, sourceRoot string) bool {
	filePath = cleanSlash(filePath)
	sourceRoot = cleanSlash(sourceRoot)
	relative := filePath
	if sourceRoot != "." {
		prefix := strings.TrimSuffix(sourceRoot, "/") + "/"
		if !strings.HasPrefix(filePath, prefix) {
			return true
		}
		relative = strings.TrimPrefix(filePath, prefix)
	}
	for _, part := range strings.Split(relative, "/") {
		if excludedSourceDirectory(part) {
			return true
		}
	}
	return false
}

func outsideSlashRoot(path string) bool {
	path = strings.TrimSpace(path)
	return path == ".." || strings.HasPrefix(path, "../") || strings.HasPrefix(path, "/")
}

func runGitBytes(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.Bytes(), nil
}

func sortGitFiles(values []GoSourceFile) {
	sort.Slice(values, func(i, j int) bool { return values[i].Path < values[j].Path })
}
