package generate

import (
	"context"
	"fmt"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "generate"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "generate canonical contract, application, module, and runtime assembly artifacts for the current project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary; defaults to PATH"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protoc import path; expert escape hatch, may be repeated"},
		},
		Action: func(c *cli.Context) error {
			report, err := projectflow.Generate(context.Background(), projectflow.Options{
				Root:       c.String("root"),
				Protoc:     c.String("protoc"),
				ProtoPaths: c.StringSlice("proto-path"),
			})
			if err != nil {
				return err
			}
			fmt.Print(projectflow.Format(report))
			return nil
		},
	}
}
