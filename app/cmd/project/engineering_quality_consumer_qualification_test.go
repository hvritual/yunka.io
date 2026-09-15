package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEngineeringQualityBaselineConsumerQualification(t *testing.T) {
	roots := []struct {
		name string
		env  string
	}{
		{name: "biz", env: "YUNKA_QUALITY_CONSUMER_BIZ"},
		{name: "iot", env: "YUNKA_QUALITY_CONSUMER_IOT"},
	}
	for _, item := range roots {
		consumerRoot := strings.TrimSpace(os.Getenv(item.env))
		if consumerRoot == "" {
			return
		}
		t.Run(item.name, func(t *testing.T) {
			goMod, err := os.ReadFile(filepath.Join(consumerRoot, "go.mod"))
			if err != nil {
				t.Fatalf("read real consumer go.mod: %v", err)
			}
			if !strings.Contains(string(goMod), "module ") {
				t.Fatalf("real consumer %s has no module identity", consumerRoot)
			}

			root := t.TempDir()
			writeProjectTestFile(t, filepath.Join(root, "go.mod"), string(goMod))
			consumerPolicy := filepath.Join(consumerRoot, ".yunka", "engineering-quality.json")
			if contents, err := os.ReadFile(consumerPolicy); err == nil {
				writeProjectTestFile(t, filepath.Join(root, ".yunka", "engineering-quality.json"), string(contents))
			} else if !os.IsNotExist(err) {
				t.Fatal(err)
			}

			first, err := EnsureEngineeringQualityBaseline(root)
			if err != nil {
				t.Fatal(err)
			}
			if first.PolicyIdentity != engineeringQualityPolicyIdentity() || first.PolicyVersion != EngineeringQualityPolicyVersion {
				t.Fatalf("consumer %s identity=%#v", item.name, first)
			}
			baselineBytes, err := os.ReadFile(filepath.Join(root, EngineeringQualityBaselineRelativePath))
			if err != nil {
				t.Fatal(err)
			}
			baseline, err := decodeEngineeringQualityBaseline(baselineBytes)
			if err != nil {
				t.Fatal(err)
			}
			if baseline.PolicyIdentity != engineeringQualityPolicyIdentity() || baseline.PolicyVersion != EngineeringQualityPolicyVersion {
				t.Fatalf("consumer %s baseline=%#v", item.name, baseline)
			}

			tracked := []string{EngineeringQualityBaselineRelativePath, EngineeringQualityRulesRelativePath, ".yunka/engineering-quality.json", EngineeringQualityInstructionsPath}
			before := make(map[string]string, len(tracked))
			for _, relative := range tracked {
				contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
				if err != nil {
					t.Fatal(err)
				}
				before[relative] = string(contents)
			}
			if _, err := EnsureEngineeringQualityBaseline(root); err != nil {
				t.Fatal(err)
			}
			for _, relative := range tracked {
				contents, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
				if err != nil {
					t.Fatal(err)
				}
				if string(contents) != before[relative] {
					t.Fatalf("consumer %s re-run mutated %s", item.name, relative)
				}
			}
		})
	}
}
