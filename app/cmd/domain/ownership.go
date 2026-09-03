package domain

import (
	"path/filepath"
	"sort"
)

// GeneratedPaths returns the exact project-code-root-relative files currently
// owned by the Domain generator. It derives the set from the same manifests,
// PO scan, and render path used by Regenerate/Check; no second ownership
// manifest is introduced.
func GeneratedPaths(root string) ([]string, error) {
	roots, err := managedDomainRoots(root)
	if err != nil {
		return nil, err
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, domainRoot := range roots {
		rawSpec, err := readManifest(domainRoot)
		if err != nil {
			return nil, err
		}
		spec := canonicalizeSpec(upgradeSpec(rawSpec))
		objects, err := scanPOObjects(domainRoot, spec.Objects, spec.TablePrefix, spec.Domain)
		if err != nil {
			return nil, err
		}
		spec.Objects = objects
		spec = canonicalizeSpec(spec)
		if err := validateSpec(spec); err != nil {
			return nil, err
		}
		moduleDirectory, goModule, err := findOwningGoModule(domainRoot)
		if err != nil {
			return nil, err
		}
		packageImport, err := importPath(moduleDirectory, goModule, domainRoot)
		if err != nil {
			return nil, err
		}
		files, err := renderComplete(spec, packageImport)
		if err != nil {
			return nil, err
		}
		domainRelative, err := filepath.Rel(absoluteRoot, domainRoot)
		if err != nil {
			return nil, err
		}
		result = append(result, filepath.ToSlash(filepath.Join(domainRelative, ManifestName)))
		for relative := range files {
			result = append(result, filepath.ToSlash(filepath.Join(domainRelative, filepath.FromSlash(relative))))
		}
	}
	sort.Strings(result)
	return result, nil
}
