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
			setRemediationCommand(),
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

func setRemediationCommand() cli.Command {
	return cli.Command{
		Name:  "remediation",
		Usage: "bind proven Audit findings to an exact ChangeSet and prove they are actually fixed",
		Subcommands: []cli.Command{
			setRemediationBindCommand(),
			setRemediationCheckCommand(),
		},
	}
}

func setRemediationBindCommand() cli.Command {
	return cli.Command{
		Name:  "bind",
		Usage: "bind one or more currently proven Audit finding IDs to the active ChangeSet before mutation",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
			cli.StringSliceFlag{Name: "finding", Usage: "exact proven Audit finding ID to remediate; repeatable"},
			cli.StringFlag{Name: "output", Value: DefaultRemediationBindingPath, Usage: "remediation binding path, relative to project root unless absolute"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format == "" {
				format = FormatText
			}
			if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
				return fmt.Errorf("change remediation bind: unsupported format %q", format)
			}
			descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: c.String("root")})
			if err != nil {
				return printFailure("yunka change set remediation bind", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation bind: resolve project: %w", err)}), 1)
			}
			value, _, err := LoadChangeSet(descriptor.Root, c.String("set"))
			if err != nil {
				return printFailure("yunka change set remediation bind", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation bind: load ChangeSet: %w", err)}), 1)
			}
			binding, err := BuildRemediationBinding(descriptor.Root, value, c.StringSlice("finding"))
			if err != nil {
				return printFailure("yunka change set remediation bind", format, Diagnose(err), 1)
			}
			path, err := WriteRemediationBinding(descriptor.Root, c.String("output"), binding)
			if err != nil {
				return printFailure("yunka change set remediation bind", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation bind: persist: %w", err)}), 1)
			}
			output, err := RenderRemediationBinding(binding, path, format)
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func setRemediationCheckCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "prove the bound Audit findings are fixed without introducing new proven architecture debt",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "set", Value: DefaultChangeSetPath, Usage: "ChangeSet path"},
			cli.StringFlag{Name: "binding", Value: DefaultRemediationBindingPath, Usage: "remediation binding path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format == "" {
				format = FormatText
			}
			if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
				return fmt.Errorf("change remediation check: unsupported format %q", format)
			}
			descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: c.String("root")})
			if err != nil {
				return printFailure("yunka change set remediation check", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation check: resolve project: %w", err)}), 1)
			}
			value, _, err := LoadChangeSet(descriptor.Root, c.String("set"))
			if err != nil {
				return printFailure("yunka change set remediation check", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation check: load ChangeSet: %w", err)}), 1)
			}
			binding, _, err := LoadRemediationBinding(descriptor.Root, c.String("binding"))
			if err != nil {
				return printFailure("yunka change set remediation check", format, Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change remediation check: load binding: %w", err)}), 1)
			}
			report, err := ReconcileRemediation(descriptor.Root, value, binding)
			if err != nil {
				return printFailure("yunka change set remediation check", format, Diagnose(&Failure{Kind: FailureEvidence, Err: err}), 1)
			}
			output, err := RenderRemediationCheck(report, format)
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
