package main

import (
	"strings"

	"github.com/urfave/cli"
)

const (
	categoryDeveloperWorkflow = "Developer workflow"
	categoryStructural        = "Structural authoring"
	categoryDiagnostics       = "Diagnostics and inspection"
	categoryExpert            = "Expert architecture"
	categorySupplementary     = "Supplementary tooling"
)

var rootCommandCategories = map[string]string{
	"init":       categoryDeveloperWorkflow,
	"generate":   categoryDeveloperWorkflow,
	"check":      categoryDeveloperWorkflow,
	"dev":        categoryDeveloperWorkflow,
	"add":        categoryStructural,
	"advisor":    categoryDiagnostics,
	"audit":      categoryDiagnostics,
	"context":    categoryDiagnostics,
	"ownership":  categoryDiagnostics,
	"change":     categoryDiagnostics,
	"doctor":     categoryDiagnostics,
	"explain":    categoryDiagnostics,
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

Use add for explicit structural authoring without inferred business semantics. Applications, event DTOs, and declarative modules may be scaffolded directly. New Operations are plan-first: add operation --plan -> change set begin -> add operation -> generate -> change set check. The prospective plan and ChangeSet bound the mutation before the Operation is written. Existing single canonical Operations retain the AX7 protocol: change plan -> change begin -> change check -> change verify; multi-subject work may compose those contracts in a ChangeSet. For AI/automation, start with context for the read-only project and protocol contract.

Use audit for read-only deterministic framework-conformance evidence; findings report existing debt but do not block by default. When a ChangeSet is intended to remediate a proven finding, bind the exact finding before mutation with change set remediation bind and prove closure afterward with change set remediation check. Declaring a remediation target never proves it fixed: the check requires the target to appear in Audit fixed, rejects remaining targets, and rejects new proven debt. Use advisor to export deterministic evidence for external AI reasoning and to validate evidence-bound advisory responses; Yunka does not invoke an LLM or authorize mutations through advisor. Ownership is the lower-level mutation guard used by the change protocol. Use doctor/explain/inspect/graph for evidence and troubleshooting. Contract, assembly, module, domain, and dependency commands remain available as explicit expert architecture interfaces.`)
	for index := range app.Commands {
		if category, ok := rootCommandCategories[app.Commands[index].Name]; ok {
			app.Commands[index].Category = category
		}
	}
}
