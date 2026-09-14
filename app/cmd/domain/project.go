package domain

import (
	"errors"
	"fmt"
	"path/filepath"
)

// RegenerateAll discovers every managed domain directly below root and
// regenerates its persistence-only artifacts. Domain-like surfaces with no
// explicit ownership decision fail closed before generation starts.
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

// CheckAll validates Domain governance coverage and then validates every
// managed domain below root without mutating project files. The returned count
// includes only MANAGED domains; EXEMPT surfaces remain visible through
// InspectCoverage but are intentionally outside generator ownership.
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
	entries, err := ValidateCoverage(root)
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.State == CoverageManaged {
			roots = append(roots, entry.Root)
		}
	}
	return roots, nil
}
