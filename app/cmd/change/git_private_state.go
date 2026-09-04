package change

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func resolveGitPrivateStatePath(root, requested, defaultLogical string) (string, string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", fmt.Errorf("resolve project root: %w", err)
	}

	requested = strings.TrimSpace(requested)
	if requested == "" {
		requested = defaultLogical
	}
	normalizedRequested := filepath.ToSlash(filepath.Clean(filepath.FromSlash(requested)))
	if normalizedRequested == defaultLogical {
		gitRelative := strings.TrimPrefix(defaultLogical, ".git/")
		if gitRelative == defaultLogical || strings.TrimSpace(gitRelative) == "" {
			return "", "", fmt.Errorf("default Git-private path %q is invalid", defaultLogical)
		}
		command := exec.Command("git", "-C", absoluteRoot, "rev-parse", "--git-path", gitRelative)
		output, err := command.CombinedOutput()
		if err != nil {
			return "", "", fmt.Errorf("resolve Git-private path %s: %w: %s", gitRelative, err, strings.TrimSpace(string(output)))
		}
		path := strings.TrimSpace(string(output))
		if path == "" {
			return "", "", fmt.Errorf("resolve Git-private path %s: git returned an empty path", gitRelative)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(absoluteRoot, filepath.FromSlash(path))
		}
		return filepath.Clean(path), defaultLogical, nil
	}

	path := requested
	if !filepath.IsAbs(path) {
		path = filepath.Join(absoluteRoot, filepath.FromSlash(path))
	}
	path = filepath.Clean(path)
	display := filepath.ToSlash(path)
	if relative, err := filepath.Rel(absoluteRoot, path); err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		display = filepath.ToSlash(relative)
	}
	return path, display, nil
}
