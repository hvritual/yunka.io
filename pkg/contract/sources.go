package contract

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const SourceInventoryVersion = 1

var sourceSetNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

type SourceInventory struct {
	SchemaVersion int         `json:"schemaVersion"`
	SourceSets    []SourceSet `json:"sourceSets"`
}

type SourceSet struct {
	Name       string   `json:"name"`
	Root       string   `json:"root"`
	Files      []string `json:"files"`
	ProtoPaths []string `json:"protoPaths,omitempty"`
}

type InventoryCompileOptions struct {
	RepositoryRoot string
	InventoryPath  string
	Protoc         string
}

func LoadSourceInventory(path string) (SourceInventory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SourceInventory{}, err
	}
	var inventory SourceInventory
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&inventory); err != nil {
		return SourceInventory{}, fmt.Errorf("contract: decode source inventory %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return SourceInventory{}, fmt.Errorf("contract: source inventory %s contains multiple JSON values", path)
		}
		return SourceInventory{}, fmt.Errorf("contract: decode source inventory %s: %w", path, err)
	}
	if inventory.SchemaVersion != SourceInventoryVersion {
		return SourceInventory{}, fmt.Errorf("contract: unsupported source inventory schemaVersion %d", inventory.SchemaVersion)
	}
	if len(inventory.SourceSets) == 0 {
		return SourceInventory{}, fmt.Errorf("contract: source inventory must contain at least one source set")
	}
	return inventory, nil
}

func CompileInventory(ctx context.Context, options InventoryCompileOptions) (CompileResult, error) {
	root, err := filepath.Abs(strings.TrimSpace(options.RepositoryRoot))
	if err != nil {
		return CompileResult{}, err
	}
	if root == "" {
		return CompileResult{}, fmt.Errorf("contract: repository root is required")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return CompileResult{}, fmt.Errorf("contract: resolve repository root: %w", err)
	}
	inventory, err := LoadSourceInventory(options.InventoryPath)
	if err != nil {
		return CompileResult{}, err
	}

	sources := append([]SourceSet(nil), inventory.SourceSets...)
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	seenSourceNames := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		if !sourceSetNameRE.MatchString(source.Name) {
			return CompileResult{}, fmt.Errorf("contract: invalid source set name %q", source.Name)
		}
		if _, duplicate := seenSourceNames[source.Name]; duplicate {
			return CompileResult{}, fmt.Errorf("contract: duplicate source set name %q", source.Name)
		}
		seenSourceNames[source.Name] = struct{}{}
	}
	merged := Manifest{SchemaVersion: ManifestVersion}
	var allFiles []string
	digest := sha256.New()

	prepared, owners, err := prepareProvenanceSources(root, sources)
	if err != nil {
		return CompileResult{}, err
	}
	for _, source := range prepared {
		rootRel, sourceRoot, expectedFiles, protoPaths := source.rootRel, source.root, source.files, source.protoPaths
		result, err := Compile(ctx, CompileOptions{
			Dir:        sourceRoot,
			ProtoPaths: protoPaths,
			Files:      expectedFiles,
			Protoc:     options.Protoc,
		})
		if err != nil {
			return CompileResult{}, fmt.Errorf("contract: source set %s: %w", source.name, err)
		}
		protoc, err := resolveProtoc(options.Protoc)
		if err != nil {
			return CompileResult{}, err
		}
		if err := rebaseInventoryProvenance(&result.Manifest, root, source, owners, protoc); err != nil {
			return CompileResult{}, err
		}
		if err := mergeSourceManifest(&merged, source.name, result.Manifest); err != nil {
			return CompileResult{}, err
		}
		for _, file := range expectedFiles {
			allFiles = append(allFiles, filepath.ToSlash(filepath.Join(rootRel, file)))
		}
		_, _ = digest.Write([]byte(source.name))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(rootRel))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(result.DescriptorSHA))
		_, _ = digest.Write([]byte{0})
	}
	merged.Normalize()
	sort.Strings(allFiles)
	return CompileResult{
		Manifest:      merged,
		DescriptorSHA: hex.EncodeToString(digest.Sum(nil)),
		Files:         allFiles,
	}, nil
}

func normalizeInventoryFiles(files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("files must be explicit and non-empty")
	}
	seen := make(map[string]struct{}, len(files))
	result := make([]string, 0, len(files))
	for _, file := range files {
		file = filepath.ToSlash(filepath.Clean(strings.TrimSpace(file)))
		if file == "." || file == "" || filepath.IsAbs(file) || file == ".." || strings.HasPrefix(file, "../") {
			return nil, fmt.Errorf("invalid proto file path %q", file)
		}
		if !strings.EqualFold(filepath.Ext(file), ".proto") {
			return nil, fmt.Errorf("source file %q is not a .proto file", file)
		}
		if _, duplicate := seen[file]; duplicate {
			return nil, fmt.Errorf("duplicate proto file %q", file)
		}
		seen[file] = struct{}{}
		result = append(result, file)
	}
	sort.Strings(result)
	return result, nil
}

func repositoryPath(repositoryRoot, configured string) (string, string, error) {
	configured = strings.TrimSpace(configured)
	if configured == "" || filepath.IsAbs(configured) {
		return "", "", fmt.Errorf("path must be repository-relative")
	}
	clean := filepath.Clean(configured)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("path escapes repository: %q", configured)
	}
	joined := filepath.Join(repositoryRoot, clean)
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(repositoryRoot, resolved)
	if err != nil {
		return "", "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", "", fmt.Errorf("resolved path escapes repository: %q", configured)
	}
	return filepath.ToSlash(clean), resolved, nil
}

func mergeSourceManifest(target *Manifest, sourceName string, incoming Manifest) error {
	fileNames := make(map[string]struct{}, len(target.Files))
	messageNames := make(map[string]struct{}, len(target.Messages))
	enumNames := make(map[string]struct{}, len(target.Enums))
	serviceNames := make(map[string]struct{}, len(target.Services))
	for _, item := range target.Files {
		fileNames[item.Name] = struct{}{}
	}
	for _, item := range target.Messages {
		messageNames[item.FullName] = struct{}{}
	}
	for _, item := range target.Enums {
		enumNames[item.FullName] = struct{}{}
	}
	for _, item := range target.Services {
		serviceNames[item.FullName] = struct{}{}
	}
	for _, item := range incoming.Files {
		if _, duplicate := fileNames[item.Name]; duplicate {
			return fmt.Errorf("contract: source set %s duplicates canonical file %s", sourceName, item.Name)
		}
		fileNames[item.Name] = struct{}{}
		target.Files = append(target.Files, item)
	}
	for _, item := range incoming.Messages {
		if _, duplicate := messageNames[item.FullName]; duplicate {
			return fmt.Errorf("contract: source set %s duplicates message %s", sourceName, item.FullName)
		}
		messageNames[item.FullName] = struct{}{}
		target.Messages = append(target.Messages, item)
	}
	for _, item := range incoming.Enums {
		if _, duplicate := enumNames[item.FullName]; duplicate {
			return fmt.Errorf("contract: source set %s duplicates enum %s", sourceName, item.FullName)
		}
		enumNames[item.FullName] = struct{}{}
		target.Enums = append(target.Enums, item)
	}
	for _, item := range incoming.Services {
		if _, duplicate := serviceNames[item.FullName]; duplicate {
			return fmt.Errorf("contract: source set %s duplicates service %s", sourceName, item.FullName)
		}
		serviceNames[item.FullName] = struct{}{}
		target.Services = append(target.Services, item)
	}
	return nil
}
