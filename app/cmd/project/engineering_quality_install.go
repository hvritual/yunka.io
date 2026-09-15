package project

import (
	"os"
	"path/filepath"
)

// EnsureEngineeringQualityBaseline installs the versioned consumer baseline for
// a Go project without overwriting project-owned edits. The canonical policy
// identity is framework-defined; existing project configuration remains an
// enforcement input and is never silently replaced.
func EnsureEngineeringQualityBaseline(root string) (EngineeringQualityInstallReport, error) {
	absolute, err := absoluteRoot(root)
	if err != nil {
		return EngineeringQualityInstallReport{}, err
	}
	hasGoModule := true
	if _, err := os.Stat(filepath.Join(absolute, "go.mod")); err != nil {
		if !os.IsNotExist(err) {
			return EngineeringQualityInstallReport{}, err
		}
		hasGoModule = false
	}
	return ensureEngineeringQualityBaseline(absolute, hasGoModule)
}
