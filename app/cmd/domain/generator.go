package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/project"
)

func Generate(options Options) error {
	root := strings.TrimSpace(options.Root)
	if root == "" {
		root = "internal"
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	moduleDirectory, goModule, err := findOwningGoModule(absoluteRoot)
	if err != nil {
		return err
	}
	projectConfig, err := project.Ensure(moduleDirectory)
	if err != nil {
		return err
	}
	if requested := strings.TrimSpace(options.TablePrefix); requested != "" && requested != projectConfig.Database.TablePrefix {
		return fmt.Errorf("domain: table prefix %q differs from project database prefix %q", requested, projectConfig.Database.TablePrefix)
	}
	spec, root, err := newDomainSpec(options, projectConfig.Database.TablePrefix)
	if err != nil {
		return err
	}
	target := filepath.Join(root, spec.Domain)
	if err := ensureInside(moduleDirectory, target); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return err
	}

	if info, statErr := os.Stat(target); statErr == nil {
		if !info.IsDir() {
			return fmt.Errorf("domain: target %s is not a directory", target)
		}
		if _, manifestErr := os.Stat(filepath.Join(target, ManifestName)); manifestErr == nil {
			return fmt.Errorf("domain: target %s already contains %s", target, ManifestName)
		} else if !os.IsNotExist(manifestErr) {
			return manifestErr
		}
		return initializeExistingDomain(target, moduleDirectory, goModule, spec, options)
	} else if !os.IsNotExist(statErr) {
		return statErr
	}

	temporary, err := os.MkdirTemp(root, ".yunka-domain-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := ensureInitialPO(temporary, options); err != nil {
		return err
	}
	objects, err := scanPOObjects(temporary, nil, spec.TablePrefix, spec.Domain)
	if err != nil {
		return err
	}
	spec.Objects = objects
	spec = canonicalizeSpec(spec)
	if err := validateSpec(spec); err != nil {
		return err
	}
	packageImport, err := importPath(moduleDirectory, goModule, target)
	if err != nil {
		return err
	}
	files, err := renderComplete(spec, packageImport)
	if err != nil {
		return err
	}
	if err := writeGeneratedFiles(temporary, files); err != nil {
		return err
	}
	if err := writeManifest(temporary, spec); err != nil {
		return err
	}
	return os.Rename(temporary, target)
}

func initializeExistingDomain(target, moduleDirectory, goModule string, spec Spec, options Options) error {
	if err := ensureInitialPO(target, options); err != nil {
		return err
	}
	objects, err := scanPOObjects(target, nil, spec.TablePrefix, spec.Domain)
	if err != nil {
		return err
	}
	spec.Objects = objects
	spec = canonicalizeSpec(spec)
	if err := validateSpec(spec); err != nil {
		return err
	}
	packageImport, err := importPath(moduleDirectory, goModule, target)
	if err != nil {
		return err
	}
	files, err := renderComplete(spec, packageImport)
	if err != nil {
		return err
	}
	if err := cleanupStaleGenerated(target, files); err != nil {
		return err
	}
	if err := writeGeneratedFiles(target, files); err != nil {
		return err
	}
	return writeManifest(target, spec)
}

func Regenerate(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("domain: --path is required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rawSpec, err := readManifest(absolute)
	if err != nil {
		return err
	}
	spec := canonicalizeSpec(upgradeSpec(rawSpec))
	moduleDirectory, goModule, err := findOwningGoModule(absolute)
	if err != nil {
		return err
	}
	projectConfig, err := project.LoadOrDefault(moduleDirectory)
	if err != nil {
		return err
	}
	if spec.TablePrefix != projectConfig.Database.TablePrefix {
		return fmt.Errorf("domain: manifest table prefix %q differs from project database prefix %q", spec.TablePrefix, projectConfig.Database.TablePrefix)
	}
	if err := migrateLegacyPOFilename(absolute, rawSpec); err != nil {
		return err
	}
	objects, err := scanPOObjects(absolute, spec.Objects, spec.TablePrefix, spec.Domain)
	if err != nil {
		return err
	}
	spec.Objects = objects
	spec = canonicalizeSpec(spec)
	if err := validateSpec(spec); err != nil {
		return err
	}
	packageImport, err := importPath(moduleDirectory, goModule, absolute)
	if err != nil {
		return err
	}
	files, err := renderComplete(spec, packageImport)
	if err != nil {
		return err
	}
	if err := cleanupStaleGenerated(absolute, files); err != nil {
		return err
	}
	if err := writeGeneratedFiles(absolute, files); err != nil {
		return err
	}
	return writeManifest(absolute, spec)
}

func writeManifest(root string, spec Spec) error {
	spec = canonicalizeSpec(spec)
	contents, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(filepath.Join(root, ManifestName), contents, 0o640)
}

func readManifest(root string) (Spec, error) {
	contents, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		return Spec{}, err
	}
	var spec Spec
	if err := json.Unmarshal(contents, &spec); err != nil {
		return Spec{}, fmt.Errorf("domain: decode %s: %w", ManifestName, err)
	}
	return spec, nil
}

func renderComplete(spec Spec, packageImport string) (map[string]string, error) {
	spec = canonicalizeSpec(spec)
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	return renderMultiStructural(spec, packageImport), nil
}

func writeGeneratedFiles(root string, files map[string]string) error {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		contents := files[relative]
		if strings.HasSuffix(relative, ".go") && strings.HasPrefix(contents, generatedDomainMarker) {
			formatted, err := format.Source([]byte(contents))
			if err != nil {
				return fmt.Errorf("domain: format %s: %w", relative, err)
			}
			contents = string(formatted)
		}
		if err := os.WriteFile(path, []byte(contents), 0o640); err != nil {
			return err
		}
	}
	return nil
}

func cleanupStaleGenerated(root string, expected map[string]string) error {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if _, keep := expected[relative]; keep {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if isFrameworkGenerated(relative, string(contents)) {
			return os.Remove(path)
		}
		return nil
	})
}

func isFrameworkGenerated(relative, contents string) bool {
	if strings.HasPrefix(contents, generatedDomainMarker) {
		return true
	}
	// C8.4 cleanup explicitly recognizes historical generated protobuf output
	// so a V2 -> V3 regeneration removes stale transport artifacts.
	if strings.HasPrefix(relative, "transport/rpc/pb/") && strings.HasSuffix(relative, ".pb.go") {
		return strings.HasPrefix(contents, "// Code generated by protoc-gen-go")
	}
	return false
}

func Check(root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "internal"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(absolute)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	moduleDirectory, goModule, err := findOwningGoModule(absolute)
	if err != nil {
		return err
	}
	projectConfig, err := project.LoadOrDefault(moduleDirectory)
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		domainRoot := filepath.Join(absolute, entry.Name())
		if _, err := os.Stat(filepath.Join(domainRoot, ManifestName)); os.IsNotExist(err) {
			continue
		}
		rawSpec, err := readManifest(domainRoot)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		spec := canonicalizeSpec(upgradeSpec(rawSpec))
		if rawSpec.Version != SpecVersion || len(rawSpec.Objects) == 0 || rawSpec.REST != nil || rawSpec.RPC != nil {
			failures = append(failures, fmt.Errorf("%s: manifest requires persistence-only V3 upgrade; run yunka domain generate --path %s", entry.Name(), domainRoot))
		}
		if spec.TablePrefix != projectConfig.Database.TablePrefix {
			failures = append(failures, fmt.Errorf("%s: table prefix %q differs from project database prefix %q", entry.Name(), spec.TablePrefix, projectConfig.Database.TablePrefix))
		}
		if spec.Domain != entry.Name() {
			failures = append(failures, fmt.Errorf("%s: manifest domain=%q must match directory name", entry.Name(), spec.Domain))
		}
		if err := validateSpec(spec); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		scanned, scanErr := scanPOObjects(domainRoot, spec.Objects, spec.TablePrefix, spec.Domain)
		if scanErr != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Name(), scanErr))
			continue
		}
		for _, object := range scanned {
			if _, err := os.Stat(filepath.Join(domainRoot, "infrastructure", "persistence", object.File)); err != nil {
				failures = append(failures, fmt.Errorf("%s: PO object %s must be stored in snake_case file %s: %w", entry.Name(), object.Name, object.File, err))
			}
		}
		if !poContractEqual(spec.Objects, scanned) {
			failures = append(failures, fmt.Errorf("%s: PO object/field contract drift; run yunka domain generate --path %s", entry.Name(), domainRoot))
		}
		expectedSpec := spec
		expectedSpec.Objects = scanned
		expectedSpec = canonicalizeSpec(expectedSpec)
		packageImport, err := importPath(moduleDirectory, goModule, domainRoot)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		expected, err := renderComplete(expectedSpec, packageImport)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		_ = filepath.WalkDir(domainRoot, func(path string, current os.DirEntry, walkErr error) error {
			if walkErr != nil || current.IsDir() {
				return walkErr
			}
			relative, relErr := filepath.Rel(domainRoot, path)
			if relErr != nil {
				return relErr
			}
			relative = filepath.ToSlash(relative)
			if _, known := expected[relative]; known {
				return nil
			}
			contents, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			if isFrameworkGenerated(relative, string(contents)) {
				failures = append(failures, fmt.Errorf("%s: stale generated file %s", entry.Name(), relative))
			}
			return nil
		})
		for relative, source := range expected {
			if strings.HasSuffix(relative, ".go") && strings.HasPrefix(source, generatedDomainMarker) {
				formatted, formatErr := format.Source([]byte(source))
				if formatErr != nil {
					failures = append(failures, formatErr)
					continue
				}
				source = string(formatted)
			}
			actual, readErr := os.ReadFile(filepath.Join(domainRoot, filepath.FromSlash(relative)))
			if readErr != nil {
				failures = append(failures, fmt.Errorf("%s: missing generated file %s", entry.Name(), relative))
				continue
			}
			if string(actual) != source {
				failures = append(failures, fmt.Errorf("%s: generated drift in %s; run yunka domain generate --path %s", entry.Name(), relative, domainRoot))
			}
		}
	}
	return errors.Join(failures...)
}
