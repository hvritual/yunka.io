package check

import (
	"context"
	"fmt"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/dxoutput"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "check"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "run the read-only fast structural and generated-artifact checks for the current project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary; defaults to PATH"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protoc import path; expert escape hatch, may be repeated"},
			cli.StringFlag{Name: "format", Value: dxoutput.FormatText, Usage: "output format: text or json"},
			cli.BoolFlag{Name: "full", Usage: "ignore fast-feedback evidence and run the canonical full check"},
		},
		Action: func(c *cli.Context) error {
			report, workflowErr := projectflow.CheckWithFastFeedback(context.Background(), projectflow.Options{
				Root:       c.String("root"),
				Protoc:     c.String("protoc"),
				ProtoPaths: c.StringSlice("proto-path"),
			}, c.Bool("full"))
			result, err := dxoutput.Build("yunka check", c.String("format"), report, workflowErr)
			if err != nil {
				return err
			}
			fmt.Print(result.Output)
			if result.ExitCode != 0 {
				return cli.NewExitError("", result.ExitCode)
			}
			return nil
		},
	}
}
