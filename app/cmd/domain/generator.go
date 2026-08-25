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
	options.TablePrefix = projectConfig.Database.TablePrefix

	spec, root, err := normalizeOptions(options)
	if err != nil {
		return err
	}
	target := filepath.Join(root, spec.Domain)
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("domain: target %s already exists", target)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := ensureInside(moduleDirectory, target); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(root, ".yunka-domain-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := writeManifest(temporary, spec); err != nil {
		return err
	}
	packageImport, err := importPath(moduleDirectory, goModule, target)
	if err != nil {
		return err
	}
	if err := writeRendered(temporary, spec, packageImport); err != nil {
		return err
	}
	if err := ensureDeveloperScaffold(temporary, spec); err != nil {
		return err
	}
	return os.Rename(temporary, target)
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
	spec, err := readManifest(absolute)
	if err != nil {
		return err
	}
	if err := validateSpec(spec); err != nil {
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
	if spec.TablePrefix != projectConfig.Database.TablePrefix {
		return fmt.Errorf("domain: manifest table prefix %q differs from project database prefix %q", spec.TablePrefix, projectConfig.Database.TablePrefix)
	}
	packageImport, err := importPath(moduleDirectory, goModule, absolute)
	if err != nil {
		return err
	}
	if err := writeRendered(absolute, spec, packageImport); err != nil {
		return err
	}
	return ensureDeveloperScaffold(absolute, spec)
}

func writeManifest(root string, spec Spec) error {
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

func writeRendered(root string, spec Spec, packageImport string) error {
	files, err := render(spec, packageImport)
	if err != nil {
		return err
	}
	if err := cleanupStaleGenerated(root, files); err != nil {
		return err
	}
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
		if strings.HasSuffix(relative, ".go") {
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

func ensureDeveloperScaffold(root string, spec Spec) error {
	path := filepath.Join(root, "infrastructure", "persistence", "po.go")
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	formatted, err := format.Source([]byte(developerPOTemplate(spec)))
	if err != nil {
		return fmt.Errorf("domain: format editable PO scaffold: %w", err)
	}
	return os.WriteFile(path, formatted, 0o640)
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
		if strings.HasPrefix(string(contents), "// Code generated by yunka domain; DO NOT EDIT.") {
			return os.Remove(path)
		}
		return nil
	})
}

func render(spec Spec, packageImport string) (map[string]string, error) {
	if err := validateSpec(spec); err != nil {
		return nil, err
	}
	files := map[string]string{
		"domain/zz_yunka_entity_gen.go":                              entityTemplate(spec),
		"application/zz_yunka_contract_gen.go":                       applicationTemplate(spec, packageImport),
		"ports/zz_yunka_repository_gen.go":                           repositoryPortTemplate(spec, packageImport),
		"infrastructure/persistence/zz_yunka_po_base_gen.go":         poBaseTemplate(spec, packageImport),
		"infrastructure/persistence/zz_yunka_repository_gen.go":      persistenceRepositoryTemplate(spec, packageImport),
		"wire/zz_yunka_wiring_gen.go":                                wireTemplate(spec, packageImport),
	}
	if spec.REST.Enabled {
		files["transport/rest/zz_yunka_rest_gen.go"] = restTemplate(spec, packageImport)
	}
	if spec.RPC.Enabled {
		files["transport/rpc/zz_yunka_rpc_gen.go"] = rpcTemplate(spec, packageImport)
		files["transport/rpc/"+spec.Object+".proto"] = protoTemplate(spec, packageImport)
	}
	return files, nil
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
		spec, err := readManifest(domainRoot)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		if err := validateSpec(spec); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		if spec.TablePrefix != projectConfig.Database.TablePrefix {
			failures = append(failures, fmt.Errorf("%s: table prefix %q differs from project database prefix %q", entry.Name(), spec.TablePrefix, projectConfig.Database.TablePrefix))
		}
		if spec.Domain != entry.Name() {
			failures = append(failures, fmt.Errorf("%s: manifest domain=%q must match directory name", entry.Name(), spec.Domain))
		}
		if _, err := os.Stat(filepath.Join(domainRoot, "infrastructure", "persistence", "po.go")); err != nil {
			failures = append(failures, fmt.Errorf("%s: editable persistence PO scaffold is missing: %w", entry.Name(), err))
		}
		packageImport, err := importPath(moduleDirectory, goModule, domainRoot)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		expected, err := render(spec, packageImport)
		if err != nil {
			failures = append(failures, err)
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
			if strings.HasPrefix(string(contents), "// Code generated by yunka domain; DO NOT EDIT.") {
				failures = append(failures, fmt.Errorf("%s: stale generated file %s", entry.Name(), relative))
			}
			return nil
		})
		for relative, source := range expected {
			if strings.HasSuffix(relative, ".go") {
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
