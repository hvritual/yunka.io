package domain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RegenerateAll discovers every managed domain directly below root and
// regenerates its persistence-only artifacts. A missing root is treated as an
// empty project so top-level project generation can remain zero-configuration.
func RegenerateAll(root string) (int, error) {
	roots, err := managedDomainRoots(root)
	if err != nil {
		return 0, err
	}
	var failures []error
	for _, domainRoot := range roots {
		if err := Regenerate(domainRoot); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", filepath.Base(domainRoot), err))
		}
	}
	return len(roots), errors.Join(failures...)
}

// CheckAll validates every managed domain below root without mutating project
// files and returns the number of discovered domains for workflow reporting.
func CheckAll(root string) (int, error) {
	roots, err := managedDomainRoots(root)
	if err != nil {
		return 0, err
	}
	if err := Check(root); err != nil {
		return len(roots), err
	}
	return len(roots), nil
}

func managedDomainRoots(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "internal"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absolute)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		domainRoot := filepath.Join(absolute, entry.Name())
		if _, err := os.Stat(filepath.Join(domainRoot, ManifestName)); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		roots = append(roots, domainRoot)
	}
	return roots, nil
}
