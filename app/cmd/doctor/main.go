package doctor

import (
	"context"
	"fmt"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/pkg/devruntime"
	"yunka.io/pkg/diagnostic"
)

const AppName = "doctor"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "check the yunka developer environment without mutating it",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: "."},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
			cli.BoolFlag{Name: "strict", Usage: "treat warnings as failures"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format != "text" && format != "json" {
				text, err := diagnostic.RenderText([]diagnostic.Diagnostic{{
					Code:     "YUNKA-DX-DEV-001",
					Severity: diagnostic.SeverityError,
					Stage:    "cli",
					Summary:  "unsupported output format",
					Detail:   fmt.Sprintf("format %q is unsupported; use text or json", format),
				}})
				if err != nil {
					return err
				}
				fmt.Print(text)
				return cli.NewExitError("", 2)
			}

			report := devruntime.Doctor(context.Background(), devruntime.DoctorOptions{Root: c.String("root")})
			if format == "json" {
				contents, err := renderDoctorJSON(report, c.Bool("strict"))
				if err != nil {
					return err
				}
				fmt.Print(string(contents))
			} else {
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
