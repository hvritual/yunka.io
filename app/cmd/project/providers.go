package project

import (
	"path/filepath"

	"github.com/hvritual/yunka.io/pkg/providerplan"
)

const ProviderManifestRelativePath = ".yunka/providers.json"

func EnsureProviderManifest(root string) (string, error) {
	absolute, err := absoluteRoot(root)
	if err != nil {
		return "", err
	}
	contents, err := providerplan.Marshal(providerplan.Empty())
	if err != nil {
		return "", err
	}
	path := filepath.Join(absolute, filepath.FromSlash(ProviderManifestRelativePath))
	if err := writeIfMissing(path, contents); err != nil {
		return "", err
	}
	return ProviderManifestRelativePath, nil
}
