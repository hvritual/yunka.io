package change

import (
	"fmt"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

func setCommand() cli.Command {
	return cli.Command{
		Name:  "set",
		Usage: "compose and reconcile a bounded multi-subject ChangeSet",
		Subcommands: []cli.Command{
			setBeginCommand(),
			setCheckCommand(),
		},
	}
}

func setBeginCommand() cli.Command {
	return cli.Command{
		Name:  "begin",
		Usage: "compose existing-operation contracts and create-operation plans on one Git baseline",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "base", Value: "HEAD", Usage: "Git commit/ref used as the authoritative ChangeSet baseline"},
			cli.StringSliceFlag{Name: "contract", Usage: "existing Operation Change Contract v1; repeatable"},
			cli.StringSliceFlag{Name: "create-plan", Usage: "T4.1 `yunka add operation --plan --format agent-json` output; repeatable"},
			cli.StringFlag{Name: "output", Value: DefaultChangeSetPath, Usage: "ChangeSet path, relative to project root unless absolute"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format == "" {
				format = FormatText
			}
			if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
				return fmt.Errorf("change set begin: unsupported format %q", format)
			}
			value, root, err := BuildChangeSet(c.String("root"), c.String("base"), c.StringSlice("contract"), c.StringSlice("create-plan"))
			if err != nil {
				return printFailure("yunka change set begin", format, Diagnose(err), 1)
			}
			path, err := WriteChangeSet(root, c.String("output"), value)
			if err != nil {
				return printFailure("yunka change set begin", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: persist: %w", err)}), 1)
			}
			output, err := RenderChangeSet(value, path, format)
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func setCheckCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "reconcile actual Git delta and canonical semantic readback with the active ChangeSet",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: c.String("root")})
			if err != nil {
				return printFailure("yunka change set check", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set check: resolve project: %w", err)}), 1)
			}
			value, _, err := LoadChangeSet(descriptor.Root, c.String("set"))
			if err != nil {
				return printFailure("yunka change set check", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set check: load: %w", err)}), 1)
			}
			report, err := ReconcileChangeSet(descriptor.Root, value)
			if err != nil {
				return printFailure("yunka change set check", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: err}), 1)
			}
			output, err := RenderChangeSetCheck(report, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			if !report.Conformant {
				return cli.NewExitError("", 1)
			}
			return nil
		},
	}
}
