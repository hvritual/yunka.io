package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeDefaultsToYK(t *testing.T) {
	root := t.TempDir()
	config, err := Initialize(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.TablePrefix != "yk" {
		t.Fatalf("prefix=%q want yk", config.Database.TablePrefix)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ConfigRelativePath))); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeAcceptsExplicitPrefixAndIsStable(t *testing.T) {
	root := t.TempDir()
	config, err := Initialize(root, "biz")
	if err != nil {
		t.Fatal(err)
	}
	if config.Database.TablePrefix != "biz" {
		t.Fatalf("prefix=%q want biz", config.Database.TablePrefix)
	}
	if _, err := Initialize(root, "biz"); err != nil {
		t.Fatal(err)
	}
	if _, err := Initialize(root, "iot"); err == nil {
		t.Fatal("expected prefix mutation to be rejected")
	}
}
