package gitproject

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Paths is a transient adapter between Yunka's project-relative path domain and
// Git's repository-relative path domain. It is derived from the current
// checkout through Git and is never persisted as another project source of
// truth.
type Paths struct {
	ProjectRoot    string
	RepositoryRoot string
	ProjectPrefix  string
}

// Resolve derives the repository root that owns projectRoot and the
// repository-relative prefix of that project. projectRoot may itself be the
// repository root.
func Resolve(projectRoot string) (Paths, error) {
	projectRoot = strings.TrimSpace(projectRoot)
	if projectRoot == "" {
		projectRoot = "."
	}
	projectAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return Paths{}, fmt.Errorf("git project paths: project root: %w", err)
	}
	repositoryBytes, err := runGitBytes(projectAbs, "rev-parse", "--show-toplevel")
	if err != nil {
		return Paths{}, fmt.Errorf("git project paths: resolve repository root: %w", err)
	}
	repositoryRoot := strings.TrimSpace(string(repositoryBytes))
	if repositoryRoot == "" {
		return Paths{}, fmt.Errorf("git project paths: Git returned an empty repository root")
	}
	repositoryAbs, err := filepath.Abs(repositoryRoot)
	if err != nil {
		return Paths{}, fmt.Errorf("git project paths: repository root: %w", err)
	}
	prefix, err := filepath.Rel(repositoryAbs, projectAbs)
	if err != nil || outsideRoot(prefix) {
		return Paths{}, fmt.Errorf("git project paths: project root %s is outside repository root %s", projectAbs, repositoryAbs)
	}
	return Paths{
		ProjectRoot:    filepath.Clean(projectAbs),
		RepositoryRoot: filepath.Clean(repositoryAbs),
		ProjectPrefix:  cleanSlash(prefix),
	}, nil
}

// ToRepository converts one project-relative path into the repository-relative
// path required by Git tree APIs.
func (paths Paths) ToRepository(projectRelative string) (string, error) {
	projectRelative = cleanSlash(projectRelative)
	if projectRelative == "" || outsideSlashRoot(projectRelative) {
		return "", fmt.Errorf("git project paths: project-relative path is required")
	}
	if paths.ProjectPrefix == "." {
		return projectRelative, nil
	}
	if projectRelative == "." {
		return paths.ProjectPrefix, nil
	}
	return cleanSlash(filepath.Join(filepath.FromSlash(paths.ProjectPrefix), filepath.FromSlash(projectRelative))), nil
}

// ToProject converts a repository-relative Git path back into the canonical
// project-relative namespace. The bool is false when the path is outside this
// project but still inside the repository.
func (paths Paths) ToProject(repositoryRelative string) (string, bool, error) {
	repositoryRelative = cleanSlash(repositoryRelative)
	if repositoryRelative == "" || outsideSlashRoot(repositoryRelative) {
		return "", false, fmt.Errorf("git project paths: repository-relative path is required")
	}
	if paths.ProjectPrefix == "." {
		return repositoryRelative, true, nil
	}
	if repositoryRelative == paths.ProjectPrefix {
		return ".", true, nil
	}
	prefix := strings.TrimSuffix(paths.ProjectPrefix, "/") + "/"
	if !strings.HasPrefix(repositoryRelative, prefix) {
		return "", false, nil
	}
	projectRelative := cleanSlash(strings.TrimPrefix(repositoryRelative, prefix))
	if projectRelative == "" || projectRelative == "." || outsideSlashRoot(projectRelative) {
		return "", false, fmt.Errorf("git project paths: invalid project-relative result for %s", repositoryRelative)
	}
	return projectRelative, true, nil
}

func cleanSlash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if value == "." {
		return "."
	}
	return strings.TrimPrefix(value, "./")
}

func outsideRoot(relative string) bool {
	return relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func outsideSlashRoot(path string) bool {
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
