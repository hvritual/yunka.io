package dev

import (
	"errors"
	"fmt"
	"os"
	"strings"

	projectcmd "yunka.io/app/cmd/project"
)

const defaultDevManifest = ".yunka/dev.json"

// resolveDevManifestPath resolves only the developer-owned manifest location.
// The manifest itself remains the sole owner of process, dependency, target,
// readiness, and runtime-closure facts.
func resolveDevManifestPath(root, configured string, explicit bool) (string, error) {
	configured = strings.TrimSpace(configured)
	if explicit {
		return configured, nil
	}
	if configured == "" {
		configured = defaultDevManifest
	}

	profile, err := projectcmd.Load(root)
	if err == nil {
		return profile.Workflow.Dev.Manifest, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return configured, nil
	}
	return "", fmt.Errorf("dev project profile: %w", err)
}
