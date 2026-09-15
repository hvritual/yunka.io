package project

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli"
)

func TestInitCommandInstallsEngineeringQualityBaseline(t *testing.T) {
	root := t.TempDir()
	writeProjectTestFile(t, filepath.Join(root, "go.mod"), "module example.com/init-quality\n\ngo 1.25.0\n")
	app := cli.NewApp()
	app.Commands = []cli.Command{Command()}
	if err := app.Run([]string{"yunka", "init", "--root", root}); err != nil {
		t.Fatal(err)
	}
	for _, relative := range []string{
		EngineeringQualityBaselineRelativePath,
		EngineeringQualityRulesRelativePath,
		".yunka/engineering-quality.json",
		EngineeringQualityInstructionsPath,
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("init did not expose %s before source changes: %v", relative, err)
		}
	}
}
