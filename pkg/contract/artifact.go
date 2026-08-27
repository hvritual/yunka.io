package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	ManifestFilename   = "manifest.json"
	OpenAPIFilename    = "openapi.json"
	TypeScriptFilename = "client.ts"
)

type ArtifactOptions struct {
	OpenAPI    OpenAPIOptions
	TypeScript TypeScriptOptions
}

type Artifacts struct {
	Manifest   []byte
	OpenAPI    []byte
	TypeScript []byte
}

type Drift struct {
	File    string `json:"file"`
	Reason  string `json:"reason"`
	Missing bool   `json:"missing,omitempty"`
}

func RenderArtifacts(manifest Manifest, options ArtifactOptions) (Artifacts, error) {
	manifest.Normalize()
	manifestBytes, err := marshalJSON(manifest)
	if err != nil {
		return Artifacts{}, err
	}
	openAPI, err := GenerateOpenAPI(manifest, options.OpenAPI)
	if err != nil {
		return Artifacts{}, err
	}
	typeScript, err := GenerateTypeScript(manifest, options.TypeScript)
	if err != nil {
		return Artifacts{}, err
	}
	return Artifacts{Manifest: manifestBytes, OpenAPI: openAPI, TypeScript: typeScript}, nil
}

func WriteArtifacts(dir string, artifacts Artifacts) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	files := map[string][]byte{
		ManifestFilename:   artifacts.Manifest,
		OpenAPIFilename:    artifacts.OpenAPI,
		TypeScriptFilename: artifacts.TypeScript,
	}
	for name, data := range files {
		if err := writeFileAtomic(filepath.Join(dir, name), data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func CheckArtifacts(dir string, artifacts Artifacts) ([]Drift, error) {
	expected := map[string][]byte{
		ManifestFilename:   artifacts.Manifest,
		OpenAPIFilename:    artifacts.OpenAPI,
		TypeScriptFilename: artifacts.TypeScript,
	}
	var drift []Drift
	for name, want := range expected {
		path := filepath.Join(dir, name)
		got, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				drift = append(drift, Drift{File: name, Reason: "generated artifact is missing", Missing: true})
				continue
			}
			return nil, err
		}
		if !bytes.Equal(got, want) {
			drift = append(drift, Drift{File: name, Reason: "generated artifact differs from contract source"})
		}
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].File < drift[j].File })
	return drift, nil
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("contract: decode manifest %s: %w", path, err)
	}
	if manifest.SchemaVersion != 1 && manifest.SchemaVersion != ManifestVersion {
		return Manifest{}, fmt.Errorf("contract: unsupported manifest schemaVersion %d", manifest.SchemaVersion)
	}
	// V1 is a read-compatibility input. Normalization upgrades the in-memory
	// view; all newly written artifacts use ManifestVersion.
	manifest.Normalize()
	return manifest, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	file, err := os.CreateTemp(dir, ".yunka-contract-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
