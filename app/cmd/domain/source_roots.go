package domain

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// discoverDomainSurfaces traverses grouping directories until a canonical
// manifest or domain/ anchor owns a source tree. Its children belong to that
// domain, not to a second inventory inferred from ordinary package names.
// Logical identity remains the domain directory name at either depth.
func discoverDomainSurfaces(root string) ([]CoverageEntry, error) {
	var surfaces []CoverageEntry
	identities := make(map[string]string)
	var visit func(string) error
	visit = func(parent string) error {
		entries, err := os.ReadDir(parent)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			path := filepath.Join(parent, entry.Name())
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("DOMAIN_SOURCE_ALIAS: discovery does not follow symbolic links: %s", path)
			}
			if !entry.IsDir() {
				continue
			}
			for _, anchor := range []string{ManifestName, domainTopologyAnchor, "go.mod"} {
				info, err := os.Lstat(filepath.Join(path, anchor))
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				if err == nil && info.Mode()&os.ModeSymlink != 0 {
					return fmt.Errorf("DOMAIN_SOURCE_ALIAS: discovery anchor is a symbolic link: %s", filepath.Join(path, anchor))
				}
				if err == nil && anchor == "go.mod" {
					return fmt.Errorf("DOMAIN_SOURCE_MODULE: nested Go module requires its own domain check: %s", path)
				}
			}
			_, manifestErr := os.Stat(filepath.Join(path, ManifestName))
			managed := manifestErr == nil
			if manifestErr != nil && !os.IsNotExist(manifestErr) {
				return manifestErr
			}
			signals, err := domainLikeSignals(path)
			if err != nil {
				return err
			}
			if !managed && len(signals) == 0 {
				if err := visit(path); err != nil {
					return err
				}
				continue
			}
			if previous, exists := identities[entry.Name()]; exists {
				return fmt.Errorf("DOMAIN_IDENTITY_CONFLICT: domain %q exists at both %s and %s", entry.Name(), previous, path)
			}
			identities[entry.Name()] = path
			state := CoverageUnknown
			if managed {
				state = CoverageManaged
			}
			surfaces = append(surfaces, CoverageEntry{Domain: entry.Name(), Root: path, State: state, Signals: signals})
		}
		return nil
	}
	if err := visit(root); err != nil {
		return nil, err
	}
	return surfaces, nil
}
