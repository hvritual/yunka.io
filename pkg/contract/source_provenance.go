package contract

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Dependencies are paths in the same namespace as Manifest.Files. External
// dependencies are descriptor import names, never local mutation authority.
func classifyProvenanceDependencies(manifest *Manifest) {
	files := make(map[string]bool, len(manifest.Files))
	for _, file := range manifest.Files {
		files[file.Name] = true
	}
	for i := range manifest.Files {
		file := &manifest.Files[i]
		imports := append(append([]string(nil), file.Dependencies...), file.ExternalDependencies...)
		file.Dependencies, file.ExternalDependencies = nil, nil
		for _, dependency := range imports {
			if files[dependency] {
				file.Dependencies = append(file.Dependencies, dependency)
			} else {
				file.ExternalDependencies = append(file.ExternalDependencies, dependency)
			}
		}
	}
}

// legacyManifestProjection keeps the established untyped V1 artifact byte
// contract. The compiler result still carries provenance; old persisted
// manifests must be recompiled before answering source-ownership queries.
func legacyManifestProjection(manifest Manifest) Manifest {
	manifest.Files = append([]File(nil), manifest.Files...)
	for i := range manifest.Files {
		manifest.Files[i].Dependencies = nil
		manifest.Files[i].ExternalDependencies = nil
	}
	manifest.Messages = append([]Message(nil), manifest.Messages...)
	for i := range manifest.Messages {
		manifest.Messages[i].SourceFile = ""
	}
	manifest.Enums = append([]Enum(nil), manifest.Enums...)
	for i := range manifest.Enums {
		manifest.Enums[i].SourceFile = ""
	}
	manifest.Services = append([]Service(nil), manifest.Services...)
	for i := range manifest.Services {
		service := &manifest.Services[i]
		service.SourceFile = ""
		service.Methods = append([]Method(nil), service.Methods...)
		for j := range service.Methods {
			service.Methods[j].SourceFile = ""
		}
	}
	return manifest
}

type preparedSourceSet struct {
	name, rootRel, root string
	files, protoPaths   []string
}

// Preparation performs the existing inventory/discovery checks before protoc
// runs, and binds physical source ownership once for the entire inventory.
// Same-named files in different source roots remain distinct identities.
func prepareProvenanceSources(root string, sources []SourceSet) ([]preparedSourceSet, map[string]string, error) {
	prepared := make([]preparedSourceSet, 0, len(sources))
	owners := map[string]string{}
	for _, source := range sources {
		rootRel, sourceRoot, err := repositoryPath(root, source.Root)
		if err != nil {
			return nil, nil, fmt.Errorf("contract: source set %s root: %w", source.Name, err)
		}
		files, err := normalizeInventoryFiles(source.Files)
		if err != nil {
			return nil, nil, fmt.Errorf("contract: source set %s: %w", source.Name, err)
		}
		discovered, err := discoverProtoFiles(sourceRoot)
		if err != nil {
			return nil, nil, fmt.Errorf("contract: source set %s discover: %w", source.Name, err)
		}
		if !equalSourcePaths(discovered, files) {
			return nil, nil, fmt.Errorf("contract: source set %s inventory drift: listed=%v discovered=%v", source.Name, files, discovered)
		}
		item := preparedSourceSet{name: source.Name, rootRel: rootRel, root: sourceRoot, files: files}
		for _, name := range files {
			canonical := filepath.ToSlash(filepath.Join(rootRel, name))
			_, physical, err := repositoryPath(root, canonical)
			if err != nil {
				return nil, nil, fmt.Errorf("contract: source set %s file %s: %w", source.Name, name, err)
			}
			info, err := os.Stat(physical)
			if err != nil {
				return nil, nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, nil, fmt.Errorf("contract: source file %s is not regular", canonical)
			}
			if isDSLSupportFile(name) {
				continue
			}
			if previous, exists := owners[physical]; exists {
				return nil, nil, fmt.Errorf("contract: duplicate physical source ownership: %s and %s", previous, canonical)
			}
			owners[physical] = canonical
		}
		for _, path := range source.ProtoPaths {
			_, absolute, err := repositoryPath(root, path)
			if err != nil {
				return nil, nil, fmt.Errorf("contract: source set %s protoPath %q: %w", source.Name, path, err)
			}
			item.protoPaths = append(item.protoPaths, absolute)
		}
		prepared = append(prepared, item)
	}
	return prepared, owners, nil
}

func equalSourcePaths(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type provenanceInclude struct {
	path     string
	external bool
}

// Rebase all declarations and imports together. Imports are resolved using
// exactly Compile's include order, not by descriptor basename or set name.
func rebaseInventoryProvenance(manifest *Manifest, root string, source preparedSourceSet, owners map[string]string, protoc string) error {
	includes := []provenanceInclude{{path: source.root}}
	if standard := standardProtoInclude(protoc); standard != "" && filepath.Clean(standard) != filepath.Clean(source.root) {
		includes = append(includes, provenanceInclude{path: standard, external: true})
	}
	for _, path := range source.protoPaths {
		includes = append(includes, provenanceInclude{path: path})
	}
	mapping := map[string]string{}
	for _, file := range manifest.Files {
		if !validProvenancePath(file.Name) {
			return fmt.Errorf("contract: invalid source provenance %q", file.Name)
		}
		physical, err := filepath.EvalSymlinks(filepath.Join(source.root, filepath.FromSlash(file.Name)))
		if err != nil {
			return err
		}
		canonical, ok := owners[physical]
		if !ok {
			return fmt.Errorf("contract: source %s has no inventory owner", file.Name)
		}
		mapping[file.Name] = canonical
	}
	translate := func(name string) (string, error) {
		value, ok := mapping[name]
		if !ok {
			return "", fmt.Errorf("contract: declaration source %q is not owned by source set %s", name, source.name)
		}
		return value, nil
	}
	for i := range manifest.Files {
		file := &manifest.Files[i]
		imports := stableStrings(append(append([]string(nil), file.Dependencies...), file.ExternalDependencies...))
		file.Dependencies, file.ExternalDependencies = nil, nil
		for _, dependency := range imports {
			physical, err := resolveProvenanceImport(root, includes, dependency)
			if err != nil {
				return fmt.Errorf("contract: source set %s import %s: %w", source.name, dependency, err)
			}
			if canonical, ok := owners[physical]; ok {
				file.Dependencies = append(file.Dependencies, canonical)
			} else {
				file.ExternalDependencies = append(file.ExternalDependencies, dependency)
			}
		}
		file.Name = mapping[file.Name]
	}
	for i := range manifest.Messages {
		name, err := translate(manifest.Messages[i].SourceFile)
		if err != nil {
			return err
		}
		manifest.Messages[i].SourceFile = name
	}
	for i := range manifest.Enums {
		name, err := translate(manifest.Enums[i].SourceFile)
		if err != nil {
			return err
		}
		manifest.Enums[i].SourceFile = name
	}
	for i := range manifest.Services {
		service := &manifest.Services[i]
		name, err := translate(service.SourceFile)
		if err != nil {
			return err
		}
		service.SourceFile = name
		for j := range service.Methods {
			name, err := translate(service.Methods[j].SourceFile)
			if err != nil {
				return err
			}
			service.Methods[j].SourceFile = name
		}
	}
	return nil
}

func resolveProvenanceImport(root string, includes []provenanceInclude, name string) (string, error) {
	if !validProvenancePath(name) {
		return "", fmt.Errorf("invalid descriptor import path %q", name)
	}
	for _, include := range includes {
		path := filepath.Join(include.path, filepath.FromSlash(name))
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("import %s is not a regular file", name)
		}
		physical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", err
		}
		boundary := root
		if include.external {
			boundary, err = filepath.EvalSymlinks(include.path)
			if err != nil {
				return "", err
			}
		}
		relative, err := filepath.Rel(boundary, physical)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("import %s escapes its source boundary", name)
		}
		return physical, nil
	}
	return "", fmt.Errorf("descriptor import %s has no resolved source", name)
}

func validProvenancePath(value string) bool {
	return value != "" && value != "." && !strings.ContainsAny(value, "\\:\x00") && !filepath.IsAbs(value) && filepath.ToSlash(filepath.Clean(value)) == value && value != ".." && !strings.HasPrefix(value, "../")
}
