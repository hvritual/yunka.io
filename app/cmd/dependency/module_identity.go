package dependency

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/moduleidentity"
)

func moduleIdentityCommand() cli.Command {
	return cli.Command{
		Name:  "module-identity",
		Usage: "inspect or explicitly migrate legacy Yunka Go module identities",
		Subcommands: []cli.Command{
			{
				Name:  "inspect",
				Usage: "report legacy yunka.io framework/gateway/pkg imports without mutating the project",
				Flags: moduleIdentityFlags(),
				Action: func(c *cli.Context) error {
					report, err := moduleidentity.Inspect(c.String("root"))
					if err != nil {
						return err
					}
					output, err := renderModuleIdentityReport(report, c.String("format"))
					if err != nil {
						return err
					}
					fmt.Print(output)
					if !report.Conformant {
						return cli.NewExitError("", 1)
					}
					return nil
				},
			},
			{
				Name:  "migrate",
				Usage: "mechanically migrate legacy Yunka module/import identities to the canonical GitHub module identity",
				Flags: moduleIdentityFlags(),
				Action: func(c *cli.Context) error {
					root, err := filepath.Abs(strings.TrimSpace(c.String("root")))
					if err != nil {
						return err
					}
					if err := requireCleanWorktreeIfGit(root); err != nil {
						return err
					}
					result, err := moduleidentity.Migrate(root)
					if err != nil {
						return err
					}
					output, err := renderModuleIdentityMigration(result, c.String("format"))
					if err != nil {
						return err
					}
					fmt.Print(output)
					return nil
				},
			},
		},
	}
}

func moduleIdentityFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "root", Value: ".", Usage: "Yunka consumer project root"},
		cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
	}
}

func renderModuleIdentityReport(report moduleidentity.Report, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "json", "agent-json":
		contents, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	case "", "text":
		var builder strings.Builder
		fmt.Fprintf(&builder, "module identity findings %d\n", len(report.Findings))
		for _, finding := range report.Findings {
			fmt.Fprintf(&builder, "%s:%d %s -> %s\n", finding.Path, finding.Line, finding.Legacy, finding.Canonical)
		}
		fmt.Fprintf(&builder, "conformant %t\n", report.Conformant)
		return builder.String(), nil
	default:
		return "", fmt.Errorf("dependency module-identity: unsupported format %q", format)
	}
}

func renderModuleIdentityMigration(result moduleidentity.MigrationResult, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "json", "agent-json":
		contents, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	case "", "text":
		var builder strings.Builder
		fmt.Fprintf(&builder, "module identity migrated %d file(s)\n", len(result.ChangedFiles))
		for _, path := range result.ChangedFiles {
			fmt.Fprintf(&builder, "%s\n", path)
		}
		fmt.Fprintf(&builder, "remaining %d\n", len(result.After))
		fmt.Fprintf(&builder, "conformant %t\n", result.Conformant)
		if len(result.ChangedFiles) > 0 {
			builder.WriteString("next: go mod tidy && yunka generate && yunka check\n")
		}
		return builder.String(), nil
	default:
		return "", fmt.Errorf("dependency module-identity: unsupported format %q", format)
	}
}

func requireCleanWorktreeIfGit(root string) error {
	probe := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	if err := probe.Run(); err != nil {
		return nil
	}
	command := exec.Command("git", "-C", root, "status", "--porcelain")
	output, err := command.Output()
	if err != nil {
		return fmt.Errorf("dependency module-identity migrate: inspect Git worktree: %w", err)
	}
	if strings.TrimSpace(string(output)) != "" {
		return errors.New("dependency module-identity migrate: Git worktree must be clean before project-wide module identity migration")
	}
	return nil
}
