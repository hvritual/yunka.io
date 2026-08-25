package project

import (
	"fmt"

	"github.com/urfave/cli"
)

const AppName = "init"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "initialize yunka project defaults",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Usage: "project root containing go.mod", Value: "."},
			cli.StringFlag{Name: "db-prefix", Usage: "database table prefix; defaults to yk on first initialization"},
		},
		Action: func(context *cli.Context) error {
			config, err := Initialize(context.String("root"), context.String("db-prefix"))
			if err != nil {
				return err
			}
			fmt.Printf("yunka project initialized: db-prefix=%s\n", config.Database.TablePrefix)
			return nil
		},
	}
}
