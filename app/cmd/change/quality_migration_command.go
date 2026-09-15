package change

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli"
)

func qualityMigrationCommand() cli.Command {
	return cli.Command{
		Name:  "migration",
		Usage: "plan and verify bounded engineering-quality structural migrations",
		Subcommands: []cli.Command{
			qualityMigrationPlanCommand(),
			qualityMigrationCheckCommand(),
		},
	}
}

func qualityMigrationPlanCommand() cli.Command {
	return cli.Command{
		Name:  "plan",
		Usage: "inventory exact baseline debt and declare a bounded structural migration before mutation",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "base", Value: "HEAD", Usage: "clean immutable Git baseline"},
			cli.StringSliceFlag{Name: "recipe", Usage: "migration recipe; may be repeated"},
			cli.StringSliceFlag{Name: "path", Usage: "exact existing or planned touched path; may be repeated"},
			cli.StringFlag{Name: "problem", Usage: "reviewability problem being repaired"},
			cli.StringSliceFlag{Name: "current-concept", Usage: "current responsibility/concept; may be repeated"},
			cli.StringSliceFlag{Name: "desired-ownership", Usage: "desired responsibility/ownership; may be repeated"},
			cli.StringFlag{Name: "why", Usage: "why the migration is needed"},
			cli.StringFlag{Name: "what", Usage: "what structural change is planned"},
			cli.StringFlag{Name: "boundary", Usage: "what behavior/API/persistence/generated scope must not change"},
			cli.StringSliceFlag{Name: "invariant", Usage: "affected invariant; may be repeated"},
			cli.StringSliceFlag{Name: "risk", Usage: "review risk; may be repeated"},
			cli.StringSliceFlag{Name: "unresolved", Usage: "unresolved semantic concern; may be repeated"},
			cli.StringFlag{Name: "output", Value: DefaultQualityMigrationPlanPath, Usage: "Git-private migration plan path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			plan, root, err := BuildQualityMigrationPlan(context.Background(), sourceCompilerOptions(c), c.String("base"), c.StringSlice("recipe"), c.StringSlice("path"), ReviewNarrative{
				Problem: c.String("problem"), CurrentConcepts: c.StringSlice("current-concept"), DesiredOwnership: c.StringSlice("desired-ownership"),
				Why: c.String("why"), What: c.String("what"), Boundary: c.String("boundary"),
				AffectedInvariants: c.StringSlice("invariant"), Risks: c.StringSlice("risk"), UnresolvedFindings: c.StringSlice("unresolved"),
			})
			if err != nil {
				return err
			}
			path, err := WriteQualityMigrationPlan(root, c.String("output"), plan)
			if err != nil {
				return err
			}
			output, err := RenderQualityMigrationPlan(plan, path, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func qualityMigrationCheckCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "build the exact-candidate migration review packet and reject scope, coverage, debt, or semantic drift",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "plan", Value: DefaultQualityMigrationPlanPath, Usage: "Git-private migration plan path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			packet, err := CheckQualityMigration(context.Background(), sourceCompilerOptions(c), c.String("plan"))
			if err != nil {
				return err
			}
			output, err := RenderQualityMigrationReview(packet, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			if !packet.Conformant {
				return cli.NewExitError("", 1)
			}
			return nil
		},
	}
}

func RenderQualityMigrationPlan(plan QualityMigrationPlan, path, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		payload := struct {
			Path string               `json:"path"`
			Plan QualityMigrationPlan `json:"plan"`
		}{Path: path, Plan: plan}
		contents, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("quality migration plan: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "quality migration plan %s\n", path)
	fmt.Fprintf(&builder, "base      %s\n", plan.BaseSHA)
	fmt.Fprintf(&builder, "recipes   %s\n", strings.Join(plan.Recipes, ","))
	fmt.Fprintf(&builder, "paths     %d\n", len(plan.TouchedPaths))
	fmt.Fprintf(&builder, "findings  %d\n", len(plan.BaselineFindings))
	fmt.Fprintf(&builder, "coverage  %s policy=%s\n", plan.Coverage.Status, plan.Coverage.PolicySHA256)
	fmt.Fprintf(&builder, "plan      %s\n", plan.PlanSHA256)
	fmt.Fprintf(&builder, "WHY       %s\nWHAT      %s\nBOUNDARY  %s\n", plan.Narrative.Why, plan.Narrative.What, plan.Narrative.Boundary)
	return builder.String(), nil
}

func RenderQualityMigrationReview(packet QualityMigrationReviewPacket, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		contents, err := json.MarshalIndent(packet, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("quality migration review: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "quality migration review\n")
	fmt.Fprintf(&builder, "base      %s\nhead      %s\n", packet.BaseSHA, packet.HeadSHA)
	fmt.Fprintf(&builder, "recipes   %s\n", strings.Join(packet.Recipes, ","))
	fmt.Fprintf(&builder, "changed   %d/%d declared paths\n", len(packet.ChangedPaths), len(packet.TouchedPaths))
	fmt.Fprintf(&builder, "behavior    %s\n", packet.BehaviorChange.State)
	fmt.Fprintf(&builder, "api         %s\n", packet.PublicAPIChange.State)
	fmt.Fprintf(&builder, "persistence %s\n", packet.PersistenceChange.State)
	fmt.Fprintf(&builder, "generated   %s\n", packet.GeneratedCodeChange.State)
	if packet.QualityDebt != nil {
		fmt.Fprintf(&builder, "debt existing=%d new=%d fixed=%d blocking-new=%d\n", packet.QualityDebt.DeterministicExisting, packet.QualityDebt.DeterministicNew, packet.QualityDebt.DeterministicFixed, packet.QualityDebt.BlockingNew)
	}
	fmt.Fprintf(&builder, "coverage  %s\n", packet.Coverage.Status)
	fmt.Fprintf(&builder, "WHY       %s\nWHAT      %s\nBOUNDARY  %s\n", packet.Projection.Why, packet.Projection.What, packet.Projection.Boundary)
	builder.WriteString("PROOF\n")
	for _, proof := range packet.Projection.Proof {
		fmt.Fprintf(&builder, "  %s\n", proof)
	}
	for _, violation := range packet.Violations {
		fmt.Fprintf(&builder, "VIOLATION %s\n", violation)
	}
	fmt.Fprintf(&builder, "conformant %t\n", packet.Conformant)
	return builder.String(), nil
}
