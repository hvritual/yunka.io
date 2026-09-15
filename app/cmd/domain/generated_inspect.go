package domain

import (
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"yunka.io/app/cmd/project"
)

type GeneratedArtifactIssueKind string

const (
	GeneratedOwnershipConflict GeneratedArtifactIssueKind = "GENERATED_OWNERSHIP_CONFLICT"
	GeneratedStaleArtifact     GeneratedArtifactIssueKind = "STALE_GENERATED_ARTIFACT"
	GeneratedArtifactDrift     GeneratedArtifactIssueKind = "GENERATED_ARTIFACT_DRIFT"
)

// GeneratedArtifactIssue is a read-only projection of deterministic Domain
// compiler ownership/drift evidence. It is derived from the same domain.json,
// PO scanner and renderer used by Generate/Check; it is not a second registry.
type GeneratedArtifactIssue struct {
	Kind   GeneratedArtifactIssueKind
	Domain string
	Path   string
	Reason string
}

// InspectGeneratedArtifacts reports generator-owned path conflicts, stale
// generated files, missing generated files and byte drift without mutating the
// project. Only MANAGED domains (those with domain.json) participate; broader
// Domain coverage closure remains owned by ValidateCoverage.
func InspectGeneratedArtifacts(root string) ([]GeneratedArtifactIssue, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "internal"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absolute)
	if os.IsNotExist(err) {
		return []GeneratedArtifactIssue{}, nil
	}
	if err != nil {
		return nil, err
	}
	moduleDirectory, goModule, err := findOwningGoModule(absolute)
	if err != nil {
		return nil, err
	}
	projectConfig, err := project.LoadOrDefault(moduleDirectory)
	if err != nil {
		return nil, err
	}

	var issues []GeneratedArtifactIssue
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		domainRoot := filepath.Join(absolute, entry.Name())
		if _, err := os.Stat(filepath.Join(domainRoot, ManifestName)); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		rawSpec, err := readManifest(domainRoot)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		spec := canonicalizeSpec(upgradeSpec(rawSpec))
		if spec.TablePrefix != projectConfig.Database.TablePrefix {
			return nil, fmt.Errorf("%s: table prefix %q differs from project database prefix %q", entry.Name(), spec.TablePrefix, projectConfig.Database.TablePrefix)
		}
		if spec.Domain != entry.Name() {
			return nil, fmt.Errorf("%s: manifest domain=%q must match directory name", entry.Name(), spec.Domain)
		}
		if err := validateSpec(spec); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		scanned, err := scanPOObjects(domainRoot, spec.Objects, spec.TablePrefix, spec.Domain)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		expectedSpec := spec
		expectedSpec.Objects = scanned
		expectedSpec = canonicalizeSpec(expectedSpec)
		packageImport, err := importPath(moduleDirectory, goModule, domainRoot)
		if err != nil {
			return nil, err
		}
		expected, err := renderComplete(expectedSpec, packageImport)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}

		walkErr := filepath.WalkDir(domainRoot, func(path string, current os.DirEntry, walkErr error) error {
			if walkErr != nil || current.IsDir() {
				return walkErr
			}
			relative, err := filepath.Rel(domainRoot, path)
			if err != nil {
				return err
			}
			relative = filepath.ToSlash(relative)
			if _, known := expected[relative]; known {
				return nil
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !isFrameworkGenerated(relative, string(contents)) {
				return nil
			}
			projectPath, err := filepath.Rel(moduleDirectory, path)
			if err != nil {
				return err
			}
			issues = append(issues, GeneratedArtifactIssue{
				Kind: GeneratedStaleArtifact, Domain: entry.Name(), Path: filepath.ToSlash(projectPath),
				Reason: "generator-owned file is no longer present in the canonical Domain renderer output",
			})
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}

		paths := make([]string, 0, len(expected))
		for relative := range expected {
			paths = append(paths, relative)
		}
		sort.Strings(paths)
		for _, relative := range paths {
			expectedSource := expected[relative]
			generatedGo := strings.HasSuffix(relative, ".go") && strings.HasPrefix(expectedSource, generatedDomainMarker)
			if !generatedGo {
				continue
			}
			formatted, err := format.Source([]byte(expectedSource))
			if err != nil {
				return nil, fmt.Errorf("%s: format %s: %w", entry.Name(), relative, err)
			}
			expectedSource = string(formatted)
			actualPath := filepath.Join(domainRoot, filepath.FromSlash(relative))
			projectPath, err := filepath.Rel(moduleDirectory, actualPath)
			if err != nil {
				return nil, err
			}
			projectPath = filepath.ToSlash(projectPath)
			actual, err := os.ReadFile(actualPath)
			if os.IsNotExist(err) {
				issues = append(issues, GeneratedArtifactIssue{
					Kind: GeneratedArtifactDrift, Domain: entry.Name(), Path: projectPath,
					Reason: "canonical Domain renderer expects a generated file that is missing",
				})
				continue
			}
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(string(actual), generatedDomainMarker) {
				issues = append(issues, GeneratedArtifactIssue{
					Kind: GeneratedOwnershipConflict, Domain: entry.Name(), Path: projectPath,
					Reason: "canonical generator-owned path contains source without the Domain generated marker",
				})
				continue
			}
			if string(actual) != expectedSource {
				issues = append(issues, GeneratedArtifactIssue{
					Kind: GeneratedArtifactDrift, Domain: entry.Name(), Path: projectPath,
					Reason: "generator-owned file differs from the canonical Domain renderer output",
				})
			}
		}
	}

	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Path != issues[j].Path {
			return issues[i].Path < issues[j].Path
		}
		if issues[i].Kind != issues[j].Kind {
			return issues[i].Kind < issues[j].Kind
		}
		return issues[i].Domain < issues[j].Domain
	})
	if issues == nil {
		return []GeneratedArtifactIssue{}, nil
	}
	return issues, nil
}
