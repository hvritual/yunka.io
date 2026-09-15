package project

import (
	"errors"
	"os"
	"path/filepath"
)

// EnsureEngineeringQualityBaseline installs the versioned consumer baseline for
// a Go project without overwriting project-owned edits. A canonical project
// profile is sufficient project identity for monorepo roots whose Go module
// lives below the profile root. Existing project configuration remains an
// enforcement input and is never silently replaced.
func EnsureEngineeringQualityBaseline(root string) (EngineeringQualityInstallReport, error) {
	absolute, err := absoluteRoot(root)
	if err != nil {
		return EngineeringQualityInstallReport{}, err
	}
	hasGoProject := true
	if _, err := os.Stat(filepath.Join(absolute, "go.mod")); err != nil {
		if !os.IsNotExist(err) {
			return EngineeringQualityInstallReport{}, err
		}
		if _, profileErr := Load(absolute); profileErr != nil {
			if !errors.Is(profileErr, os.ErrNotExist) {
				return EngineeringQualityInstallReport{}, profileErr
			}
			hasGoProject = false
		}
	}
	return ensureEngineeringQualityBaseline(absolute, hasGoProject)
}
