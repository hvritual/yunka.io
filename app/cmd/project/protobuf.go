package project

import "path/filepath"

const ProtobufGoManifestRelativePath = ".yunka/protobuf-go.json"

var emptyProtobufGoManifest = []byte("{\n  \"schemaVersion\": 1,\n  \"files\": []\n}\n")

// EnsureProtobufGoManifest explicitly adopts strict protobuf Go output
// ownership for new or migrated projects. Legacy projects without this marker
// remain compatible with their existing generated-output ownership model.
func EnsureProtobufGoManifest(root string) (string, error) {
	absolute, err := absoluteRoot(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(absolute, filepath.FromSlash(ProtobufGoManifestRelativePath))
	if err := writeIfMissing(path, emptyProtobufGoManifest); err != nil {
		return "", err
	}
	return ProtobufGoManifestRelativePath, nil
}
