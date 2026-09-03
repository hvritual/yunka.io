package main

import (
	"bytes"
	"sort"
	"strings"
	"testing"

	"github.com/urfave/cli"
)

func TestC116ARootCommandTaxonomyIsCompleteAndStable(t *testing.T) {
	want := map[string]string{
		"add":        categoryStructural,
		"api":        categorySupplementary,
		"assembly":   categoryExpert,
		"change":     categoryDiagnostics,
		"check":      categoryDeveloperWorkflow,
		"context":    categoryDiagnostics,
		"contract":   categoryExpert,
		"dependency": categoryExpert,
		"dev":        categoryDeveloperWorkflow,
		"doc":        categorySupplementary,
		"doctor":     categoryDiagnostics,
		"domain":     categoryExpert,
		"explain":    categoryDiagnostics,
		"generate":   categoryDeveloperWorkflow,
		"graph":      categoryDiagnostics,
		"init":       categoryDeveloperWorkflow,
		"inspect":    categoryDiagnostics,
		"module":     categoryExpert,
		"ownership":  categoryDiagnostics,
	}
	if len(rootCommandCategories) != len(want) {
		t.Fatalf("root category inventory=%d want %d", len(rootCommandCategories), len(want))
	}
	for name, category := range want {
		if got := rootCommandCategories[name]; got != category {
			t.Fatalf("command %q category=%q want %q", name, got, category)
		}
	}
	for name := range rootCommandCategories {
		if _, ok := want[name]; !ok {
			t.Fatalf("unexpected categorized root command %q", name)
		}
	}
}

func TestC116ADeveloperWorkflowContainsOnlyFourHappyPathCommands(t *testing.T) {
	var got []string
	for name, category := range rootCommandCategories {
		if category == categoryDeveloperWorkflow {
			got = append(got, name)
		}
	}
	sort.Strings(got)
	want := []string{"check", "dev", "generate", "init"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("developer workflow=%v want %v", got, want)
	}
}

func TestC116AApplyDiscoverabilityPreservesCommandsAndRendersHappyPath(t *testing.T) {
	commands := make([]cli.Command, 0, len(rootCommandCategories))
	for name := range rootCommandCategories {
		commands = append(commands, cli.Command{Name: name, Usage: "test " + name})
	}
	sort.Sort(cli.CommandsByName(commands))
	before := make([]string, 0, len(commands))
	for _, command := range commands {
		before = append(before, command.Name)
	}

	app := cli.NewApp()
	app.Name = "yunka"
	app.Commands = commands
	var output bytes.Buffer
	app.Writer = &output
	applyDiscoverability(app)

	after := make([]string, 0, len(app.Commands))
	for _, command := range app.Commands {
		after = append(after, command.Name)
		wantCategory, ok := rootCommandCategories[command.Name]
		if !ok {
			t.Fatalf("uncategorized command %q", command.Name)
		}
		if command.Category != wantCategory {
			t.Fatalf("command %q category=%q want %q", command.Name, command.Category, wantCategory)
		}
	}
	if strings.Join(before, "\x00") != strings.Join(after, "\x00") {
		t.Fatalf("discoverability changed command inventory: before=%v after=%v", before, after)
	}
	if !strings.Contains(app.Description, "yunka init -> yunka generate -> yunka check -> yunka dev") {
		t.Fatalf("root description does not contain happy path: %q", app.Description)
	}
	if !strings.Contains(app.Description, "explicit structural authoring") {
		t.Fatalf("root description does not explain structural authoring: %q", app.Description)
	}
	for _, expected := range []string{"change plan", "change begin", "change check", "change verify"} {
		if !strings.Contains(app.Description, expected) {
			t.Fatalf("root description does not expose bounded change protocol %q: %q", expected, app.Description)
		}
	}
	if strings.Contains(strings.ToLower(app.Description), "deprecated") || strings.Contains(strings.ToLower(app.Description), "legacy") {
		t.Fatalf("root description introduced unsupported deprecation language: %q", app.Description)
	}

	// Exercise the same setup path as the real CLI. urfave/cli v1 builds its
	// command-category index during App.Run; calling ShowAppHelp directly would
	// bypass that setup and render an empty COMMANDS section.
	if err := app.Run([]string{"yunka", "--help"}); err != nil {
		t.Fatal(err)
	}
	help := output.String()
	for _, expected := range []string{
		"Developer workflow",
		"Structural authoring",
		"Diagnostics and inspection",
		"Expert architecture",
		"Supplementary tooling",
		"yunka init -> yunka generate -> yunka check -> yunka dev",
		"change plan -> change begin -> change check -> change verify",
		"add",
		"context",
		"ownership",
		"change",
		"explain",
	} {
		if !strings.Contains(help, expected) {
			t.Fatalf("root help missing %q:\n%s", expected, help)
		}
	}
	for name := range rootCommandCategories {
		if !strings.Contains(help, name) {
			t.Fatalf("root help no longer exposes command %q:\n%s", name, help)
		}
	}
}
