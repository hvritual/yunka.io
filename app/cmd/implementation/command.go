// Package implementation materializes developer-owned starter code from the
// canonical renderer. It never modifies generated code or executes an Application.
package implementation

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

// Command is exposed under the existing yunka add authoring command.
func Command() cli.Command {
	return cli.Command{
		Name: "implementation", Usage: "plan or create a sealed leaf Application starter from its existing canonical contract",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "canonical project root"},
			cli.StringFlag{Name: "protoc", Usage: "explicit protobuf compiler"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional canonical protobuf include path"},
			cli.StringFlag{Name: "composition-package", Usage: "required exact Go package permitted to call the owner factory"},
			cli.BoolFlag{Name: "apply", Usage: "create missing planned files exclusively; default is read-only plan"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text, json or agent-json"},
		},
		Action: func(c *cli.Context) error {
			if c.NArg() != 1 {
				return fmt.Errorf("add implementation: one domain/application key is required")
			}
			if c.String("format") != "text" && c.String("format") != "json" && c.String("format") != "agent-json" {
				return fmt.Errorf("add implementation: unsupported format")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			options := Options{Project: projectflow.Options{Root: c.String("root"), Protoc: c.String("protoc"), ProtoPaths: c.StringSlice("proto-path")}, Application: c.Args().First(), CompositionPackage: c.String("composition-package")}
			report, err := Run(ctx, options, c.Bool("apply"))
			if err != nil {
				return err
			}
			if c.String("format") != "text" {
				data, err := json.MarshalIndent(report, "", "  ")
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(c.App.Writer, string(data))
				return err
			}
			if _, err = fmt.Fprintf(c.App.Writer, "IMPLEMENTATION %s application=%s contract=%s\n", report.Mode, report.Application, report.Contract); err != nil {
				return err
			}
			for _, file := range report.Files {
				if _, err = fmt.Fprintf(c.App.Writer, "%s %s sha256=%s\n", file.Action, file.Path, file.SHA256); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintln(c.App.Writer, "Editable starter only: handlers return explicit not-implemented errors. No runtime registration, permissions or persistence behavior was generated.")
			return err
		},
	}
}
