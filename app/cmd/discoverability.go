package main

import (
	"strings"

	"github.com/urfave/cli"
)

const (
	categoryDeveloperWorkflow = "Developer workflow"
	categoryDiagnostics       = "Diagnostics and inspection"
	categoryExpert            = "Expert architecture"
	categorySupplementary     = "Supplementary tooling"
)

var rootCommandCategories = map[string]string{
	"init":       categoryDeveloperWorkflow,
	"generate":   categoryDeveloperWorkflow,
	"check":      categoryDeveloperWorkflow,
	"dev":        categoryDeveloperWorkflow,
	"doctor":     categoryDiagnostics,
	"inspect":    categoryDiagnostics,
	"graph":      categoryDiagnostics,
	"contract":   categoryExpert,
	"assembly":   categoryExpert,
	"module":     categoryExpert,
	"domain":     categoryExpert,
	"dependency": categoryExpert,
	"api":        categorySupplementary,
	"doc":        categorySupplementary,
}

func applyDiscoverability(app *cli.App) {
	if app == nil {
		return
	}
	app.Usage = "correct-by-default Yunka application development"
	app.Description = strings.TrimSpace(`Start here with the developer workflow:

  yunka init -> yunka generate -> yunka check -> yunka dev

Use doctor/inspect/graph for evidence and troubleshooting. Contract, assembly, module, domain, and dependency commands remain available as explicit expert architecture interfaces.`)
	for index := range app.Commands {
		if category, ok := rootCommandCategories[app.Commands[index].Name]; ok {
			app.Commands[index].Category = category
		}
	}
}
