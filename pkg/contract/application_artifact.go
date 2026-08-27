package contract

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var ErrApplicationOutputRequired = errors.New("contract application codegen: output root and import root are required for typed applications")

func HasTypedApplications(manifest Manifest) bool {
	for _, service := range manifest.Services {
		if service.Application != nil {
			return true
		}
	}
	return false
}

func WriteApplicationCode(root string, files []GeneratedApplicationFile) error {
	root = strings.TrimSpace(root)
	if root == "" {
		if len(files) == 0 {
			return nil
		}
		return ErrApplicationOutputRequired
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	expected, domains, err := applicationFileMap(files)
	if err != nil {
		return err
	}
	if err := cleanupStaleApplicationCode(absolute, expected, domains); err != nil {
		return err
	}
	paths := make([]string, 0, len(expected))
	for relative := range expected {
		paths = append(paths, relative)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		target, err := containedApplicationPath(absolute, relative)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := writeFileAtomic(target, expected[relative], 0o644); err != nil {
			return err
		}
	}
	return nil
}

func CheckApplicationCode(root string, files []GeneratedApplicationFile) ([]Drift, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		if len(files) == 0 {
			return nil, nil
		}
		return nil, ErrApplicationOutputRequired
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	expected, domains, err := applicationFileMap(files)
	if err != nil {
		return nil, err
	}
	var drift []Drift
	for relative, want := range expected {
		target, err := containedApplicationPath(absolute, relative)
		if err != nil {
			return nil, err
		}
		got, err := os.ReadFile(target)
		if err != nil {
			if os.IsNotExist(err) {
				drift = append(drift, Drift{File: relative, Reason: "generated application artifact is missing", Missing: true})
				continue
			}
			return nil, err
		}
		if !bytes.Equal(got, want) {
			drift = append(drift, Drift{File: relative, Reason: "generated application artifact differs from PB contract"})
		}
	}
	stale, err := staleApplicationCode(absolute, expected, domains)
	if err != nil {
		return nil, err
	}
	for _, relative := range stale {
		drift = append(drift, Drift{File: relative, Reason: "stale generated application artifact"})
	}
	sort.Slice(drift, func(i, j int) bool { return drift[i].File < drift[j].File })
	return drift, nil
}

func applicationFileMap(files []GeneratedApplicationFile) (map[string][]byte, []string, error) {
	expected := make(map[string][]byte, len(files))
	domainSet := map[string]struct{}{}
	for _, file := range files {
		relative := filepath.ToSlash(filepath.Clean(strings.TrimSpace(file.Path)))
		if relative == "." || relative == "" || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, "../") {
			return nil, nil, fmt.Errorf("contract application codegen: invalid generated path %q", file.Path)
		}
		parts := strings.Split(relative, "/")
		if len(parts) < 3 || !managedApplicationRelative(relative) {
			return nil, nil, fmt.Errorf("contract application codegen: path %q is outside managed application output", relative)
		}
		if _, duplicate := expected[relative]; duplicate {
			return nil, nil, fmt.Errorf("contract application codegen: duplicate generated path %q", relative)
		}
		if !bytes.HasPrefix(file.Content, []byte(GeneratedApplicationMarker)) {
			return nil, nil, fmt.Errorf("contract application codegen: generated marker missing from %q", relative)
		}
		expected[relative] = append([]byte(nil), file.Content...)
		domainSet[parts[0]] = struct{}{}
	}
	domains := make([]string, 0, len(domainSet))
	for domain := range domainSet {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	return expected, domains, nil
}

func managedApplicationRelative(relative string) bool {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) < 3 {
		return false
	}
	switch parts[1] {
	case "application", "policy":
		return len(parts) == 3 && strings.HasPrefix(parts[2], "zz_yunka_") && strings.HasSuffix(parts[2], "_gen.go")
	case "transport":
		return len(parts) == 4 && (parts[2] == "rpc" || parts[2] == "rest") && strings.HasPrefix(parts[3], "zz_yunka_") && strings.HasSuffix(parts[3], "_gen.go")
	default:
		return false
	}
}

func containedApplicationPath(root, relative string) (string, error) {
	target := filepath.Join(root, filepath.FromSlash(relative))
	cleanRoot := filepath.Clean(root)
	cleanTarget := filepath.Clean(target)
	rel, err := filepath.Rel(cleanRoot, cleanTarget)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("contract application codegen: path escapes output root: %s", relative)
	}
	return cleanTarget, nil
}

func cleanupStaleApplicationCode(root string, expected map[string][]byte, domains []string) error {
	stale, err := staleApplicationCode(root, expected, domains)
	if err != nil {
		return err
	}
	for _, relative := range stale {
		target, err := containedApplicationPath(root, relative)
		if err != nil {
			return err
		}
		contents, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(contents, []byte(GeneratedApplicationMarker)) {
			return fmt.Errorf("contract application codegen: refusing to delete developer-owned file %s", relative)
		}
		if err := os.Remove(target); err != nil {
			return err
		}
	}
	return nil
}

func staleApplicationCode(root string, expected map[string][]byte, domains []string) ([]string, error) {
	var stale []string
	for _, domain := range domains {
		for _, managed := range []string{
			filepath.Join(domain, "application"),
			filepath.Join(domain, "policy"),
			filepath.Join(domain, "transport", "rpc"),
			filepath.Join(domain, "transport", "rest"),
		} {
			directory, err := containedApplicationPath(root, filepath.ToSlash(managed))
			if err != nil {
				return nil, err
			}
			entries, err := os.ReadDir(directory)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return nil, err
			}
			for _, entry := range entries {
				if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
					continue
				}
				relative := filepath.ToSlash(filepath.Join(managed, entry.Name()))
				if _, keep := expected[relative]; keep {
					continue
				}
				contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
				if err != nil {
					return nil, err
				}
				if bytes.HasPrefix(contents, []byte(GeneratedApplicationMarker)) {
					stale = append(stale, relative)
				}
			}
		}
	}
	sort.Strings(stale)
	return stale, nil
}
