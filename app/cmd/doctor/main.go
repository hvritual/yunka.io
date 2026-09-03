package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli"
	"github.com/hvritual/yunka.io/pkg/devruntime"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

const AppName = "doctor"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "check the yunka developer environment without mutating it",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: "."},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text, json, or agent-json"},
			cli.BoolFlag{Name: "strict", Usage: "treat warnings as failures"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format != "text" && format != "json" && format != "agent-json" {
				item := diagnostic.MustDefinition(diagnostic.CodeUnsupportedOutputFormat).Diagnostic(diagnostic.SeverityError)
				item.Detail = fmt.Sprintf("format %q is unsupported; use text, json, or agent-json", format)
				text, err := diagnostic.RenderText([]diagnostic.Diagnostic{item})
				if err != nil {
					return err
				}
				fmt.Print(text)
				return cli.NewExitError("", 2)
			}

			report := devruntime.Doctor(context.Background(), devruntime.DoctorOptions{Root: c.String("root")})
			switch format {
			case "agent-json":
				contents, err := renderDoctorAgentJSON(report, c.Bool("strict"))
				if err != nil {
					return err
				}
				fmt.Print(string(contents))
			case "json":
				contents, err := renderDoctorJSON(report, c.Bool("strict"))
				if err != nil {
					return err
				}
				fmt.Print(string(contents))
			default:
				text, err := renderDoctorText(report)
				if err != nil {
					return err
				}
				fmt.Print(text)
			}
			if report.Failed(c.Bool("strict")) {
				return cli.NewExitError("", 1)
			}
			return nil
		},
	}
}
