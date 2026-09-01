package projectflow

import (
	"fmt"
	"os"
	"path/filepath"

	projectcmd "yunka.io/app/cmd/project"
	contractcore "yunka.io/pkg/contract"
	"yunka.io/pkg/providerplan"
)

func validateProviderClosure(project resolvedProject) (int, bool, error) {
	modules, _, err := contractcore.DiscoverModuleSnapshot(project.ModuleRoot)
	if err != nil {
		return 0, false, fmt.Errorf("provider closure module snapshot: %w", err)
	}
	path := filepath.Join(project.Root, filepath.FromSlash(projectcmd.ProviderManifestRelativePath))
	manifest, err := providerplan.Load(path)
	if os.IsNotExist(err) {
		if providerplan.HasRequirements(modules) {
			return 0, false, fmt.Errorf("provider closure: %s is required by module capabilities; run `yunka init` then declare providers", projectcmd.ProviderManifestRelativePath)
		}
		return 0, false, nil
	}
	if err != nil {
		return 0, true, err
	}
	if err := providerplan.ValidateModules(manifest, modules); err != nil {
		return 0, true, err
	}
	return providerplan.BindingCount(manifest), true, nil
}
