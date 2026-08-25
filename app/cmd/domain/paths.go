package domain

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func findOwningGoModule(start string) (string, string, error) {
	current, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	for {
		if info, statErr := os.Stat(current); statErr == nil && !info.IsDir() {
			current = filepath.Dir(current)
		} else if os.IsNotExist(statErr) {
			current = filepath.Dir(current)
		} else if statErr != nil {
			return "", "", statErr
		}
		path := filepath.Join(current, "go.mod")
		if file, openErr := os.Open(path); openErr == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if strings.HasPrefix(line, "module ") {
					value := strings.TrimSpace(strings.TrimPrefix(line, "module "))
					if value == "" {
						return "", "", fmt.Errorf("domain: empty module directive in %s", path)
					}
					return current, value, nil
				}
			}
			return "", "", fmt.Errorf("domain: module directive not found in %s", path)
		} else if !os.IsNotExist(openErr) {
			return "", "", openErr
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", "", fmt.Errorf("domain: go.mod not found for %s", start)
		}
		current = parent
	}
}

func ensureInside(moduleDirectory, target string) error {
	relative, err := filepath.Rel(moduleDirectory, target)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("domain: target %s escapes Go module %s", target, moduleDirectory)
	}
	return nil
}

func importPath(moduleDirectory, goModule, target string) (string, error) {
	if err := ensureInside(moduleDirectory, target); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(moduleDirectory, target)
	if err != nil {
		return "", err
	}
	if relative == "." {
		return goModule, nil
	}
	return strings.TrimSuffix(goModule, "/") + "/" + filepath.ToSlash(relative), nil
}
