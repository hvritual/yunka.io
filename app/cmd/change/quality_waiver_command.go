package change

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/projectflow"
)

func qualityWaiverCommand() cli.Command {
	return cli.Command{
		Name:  "waiver",
		Usage: "create or validate exact-candidate engineering-quality waivers",
		Subcommands: []cli.Command{qualityWaiverCreateCommand(), qualityWaiverCheckCommand()},
	}
}

func qualityWaiverCreateCommand() cli.Command {
	return cli.Command{
		Name:  "create",
		Usage: "create a Git-private waiver set for selected current blocking new findings",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "contract", Value: DefaultChangeContractPath, Usage: "change contract path"},
			cli.StringFlag{Name: "output", Value: DefaultQualityWaiverPath, Usage: "Git-private waiver output path"},
			cli.StringSliceFlag{Name: "finding", Usage: "exact blocking new Audit finding ID; may be repeated"},
			cli.StringFlag{Name: "owner", Usage: "human owner accountable for the waiver"},
			cli.StringFlag{Name: "reason", Usage: "why the blocking debt is temporarily accepted"},
			cli.StringFlag{Name: "expires-at", Usage: "RFC3339 expiry timestamp"},
			cli.StringFlag{Name: "review-condition", Usage: "condition that requires explicit re-review"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: c.String("root")})
			if err != nil {
				return err
			}
			contractValue, _, err := LoadChangeContract(descriptor.Root, c.String("contract"))
			if err != nil {
				return err
			}
			headSHA, err := resolveGitBase(descriptor.Root, "HEAD")
			if err != nil {
				return err
			}
			report, err := audit.BuildWithBase(descriptor.Root, contractValue.BaseSHA)
			if err != nil {
				return err
			}
			if report.Debt == nil {
				return fmt.Errorf("quality waiver create: deterministic debt delta is unavailable")
			}
			now := time.Now().UTC()
			set, err := NewQualityWaiverSet(
				contractValue.BaseSHA,
				headSHA,
				c.String("owner"),
				c.String("reason"),
				c.String("expires-at"),
				c.String("review-condition"),
				report.Debt.New,
				c.StringSlice("finding"),
				now,
			)
			if err != nil {
				return err
			}
			path, err := WriteQualityWaiverSet(descriptor.Root, c.String("output"), set)
			if err != nil {
				return err
			}
			return printQualityWaiverSet(set, path, c.String("format"))
		},
	}
}

func qualityWaiverCheckCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "validate a waiver set against the exact current base/head and blocking new findings",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "contract", Value: DefaultChangeContractPath, Usage: "change contract path"},
			cli.StringFlag{Name: "waivers", Value: DefaultQualityWaiverPath, Usage: "waiver set path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: c.String("root")})
			if err != nil {
				return err
			}
			contractValue, _, err := LoadChangeContract(descriptor.Root, c.String("contract"))
			if err != nil {
				return err
			}
			headSHA, err := resolveGitBase(descriptor.Root, "HEAD")
			if err != nil {
				return err
			}
			report, err := audit.BuildWithBase(descriptor.Root, contractValue.BaseSHA)
			if err != nil {
				return err
			}
			if report.Debt == nil {
				return fmt.Errorf("quality waiver check: deterministic debt delta is unavailable")
			}
			blocking := blockingNewFindings(report.Debt.New)
			set, err := LoadQualityWaiverSet(descriptor.Root, c.String("waivers"), contractValue.BaseSHA, headSHA, blocking, time.Now().UTC())
			if err != nil {
				return err
			}
			if set == nil {
				return fmt.Errorf("quality waiver check: waiver set is missing")
			}
			return printQualityWaiverSet(*set, c.String("waivers"), c.String("format"))
		},
	}
}

func printQualityWaiverSet(set QualityWaiverSet, path, format string) error {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case FormatJSON, FormatAgentJSON:
		payload := struct {
			Path string           `json:"path"`
			Set  QualityWaiverSet `json:"waiverSet"`
		}{Path: path, Set: set}
		contents, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(contents))
		return nil
	case "", FormatText:
		fmt.Printf("quality waiver set %s sha256=%s waivers=%d\n", path, set.SetSHA256, len(set.Waivers))
		for _, waiver := range set.Waivers {
			fmt.Printf("  %s finding=%s owner=%s expires=%s review=%s\n", waiver.ID, waiver.Scope.FindingID, waiver.Owner, waiver.ExpiresAt, waiver.ReviewCondition)
		}
		return nil
	default:
		return fmt.Errorf("quality waiver: unsupported format %q; use text, json, or agent-json", format)
	}
}
