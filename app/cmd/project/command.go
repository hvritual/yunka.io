package project

import (
	"fmt"

	"github.com/urfave/cli"
)

const AppName = "init"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "initialize or migrate the yunka developer project profile",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Usage: "project root", Value: "."},
			cli.StringFlag{Name: "db-prefix", Usage: "database table prefix; defaults to yk on first initialization"},
		},
		Action: func(context *cli.Context) error {
			config, err := Initialize(context.String("root"), context.String("db-prefix"))
			if err != nil {
				return err
			}
			fmt.Printf("yunka project initialized: version=%d db-prefix=%s\n", config.Version, config.Database.TablePrefix)
			if config.Workflow.Contract.Sources != "" {
				fmt.Printf("contract: sources=%s generated=%s\n", config.Workflow.Contract.Sources, config.Workflow.Contract.Generated)
			} else {
				fmt.Printf("contract: proto-root=%s generated=%s\n", config.Workflow.Contract.ProtoRoot, config.Workflow.Contract.Generated)
			}
			fmt.Printf("modules: root=%s\n", config.Workflow.Modules.Root)
			generatedImport := config.Workflow.GeneratedGo.Import
			if generatedImport == "" {
				generatedImport = "<derive-from-go.mod>"
			}
			fmt.Printf("generated-go: root=%s import=%s\n", config.Workflow.GeneratedGo.Root, generatedImport)
			fmt.Printf("dev: manifest=%s\n", config.Workflow.Dev.Manifest)
			return nil
		},
	}
}
