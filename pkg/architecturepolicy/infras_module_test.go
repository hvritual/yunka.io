package architecturepolicy

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfrasModuleBoundary(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.work")); os.IsNotExist(err) {
		t.Skip("repository root not available")
	}

	goWork := readArchitectureFile(t, filepath.Join(root, "go.work"))
	if !strings.Contains(goWork, "./infras") {
		t.Fatal("go.work must include the separately versioned infras module")
	}
	infrasMod := readArchitectureFile(t, filepath.Join(root, "infras", "go.mod"))
	if !strings.HasPrefix(infrasMod, "module github.com/hvritual/yunka.io/infras\n") {
		t.Fatal("infras/go.mod must declare github.com/hvritual/yunka.io/infras")
	}
	frameworkMod := readArchitectureFile(t, filepath.Join(root, "framework", "go.mod"))
	if strings.Contains(frameworkMod, "github.com/hvritual/yunka.io/infras") {
		t.Fatal("framework must not depend on the optional infras distribution module")
	}

	forbidden := []byte("github.com/hvritual/yunka.io/infras")
	err = filepath.WalkDir(filepath.Join(root, "framework"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(content, forbidden) {
			relative, _ := filepath.Rel(root, path)
			t.Errorf("framework -> infras dependency is forbidden: %s", filepath.ToSlash(relative))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInfrasAutoloadPackagesRemainDescriptorOnly(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "infras", "modules"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("infras module not available")
	}
	var found int
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || filepath.Base(filepath.Dir(path)) != "autoload" || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		found++
		diagnostics, err := CheckAutoloadFile(path)
		if err != nil {
			return err
		}
		if len(diagnostics) != 0 {
			relative, _ := filepath.Rel(root, path)
			t.Errorf("%s diagnostics=%#v", filepath.ToSlash(relative), diagnostics)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("expected at least one infras autoload plugin")
	}
}

func readArchitectureFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
