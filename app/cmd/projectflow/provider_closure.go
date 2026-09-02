package projectflow

import (
	"fmt"
	"os"
	"path/filepath"

	projectcmd "yunka.io/app/cmd/project"
	contractcore "github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/providerplan"
)

func validateProviderClosure(project resolvedProject) (int, bool, error) {
	modules, _, err := contractcore.DiscoverModuleSnapshot(project.ModuleRoot)
	if err != nil {
		return 0, false, fmt.Errorf("provider closure module snapshot: %w", err)
	}
	path := filepath.Join(project.Root, filepath.FromSlash(projectcmd.ProviderManifestRelativePath))
	manifest, err := providerplan.Load(path)
	if os.IsNotExist(err) {
		// Backward-compatibility boundary: projects that predate the declarative
		// provider manifest may continue to assemble platform.Provider explicitly
		// in consumer code. `yunka init` creates the manifest for new/adopted
		// projects; once it exists, module capability closure is strict below.
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
