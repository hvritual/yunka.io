package module

import (
	"fmt"

	"github.com/urfave/cli"
)

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "create and validate typed modules",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "name", Usage: "compatibility module name", Value: "org"},
		},
		Action: func(context *cli.Context) error {
			return Generate(context.String("name"))
		},
		Subcommands: []cli.Command{
			{
				Name:  "add",
				Usage: "declare a module without generated consumer boilerplate",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name,n", Usage: "module name"},
					cli.StringFlag{Name: "root", Usage: "module directory", Value: "modules"},
					cli.StringFlag{Name: "version", Usage: "module contract version", Value: "v0.1.0"},
					cli.StringFlag{Name: "config-key", Usage: "declared configuration key"},
					cli.BoolFlag{Name: "logger", Usage: "require logger capability"},
					cli.StringSliceFlag{Name: "database", Usage: "declared named GORM database; repeatable"},
					cli.BoolFlag{Name: "event-bus", Usage: "require the application event bus"},
					cli.StringSliceFlag{Name: "rpc", Usage: "declared named gRPC connection; repeatable"},
					cli.StringSliceFlag{Name: "depends-on", Usage: "declared module dependency; repeatable"},
				},
				Action: func(context *cli.Context) error {
					return AddSpec(SpecOptions{
						Name:      context.String("name"),
						Root:      context.String("root"),
						Version:   context.String("version"),
						ConfigKey: context.String("config-key"),
						Logger:    context.Bool("logger"),
						Databases: context.StringSlice("database"),
						EventBus:  context.Bool("event-bus"),
						RPC:       context.StringSlice("rpc"),
						DependsOn: context.StringSlice("depends-on"),
					})
				},
			},
			{
				Name:  "require",
				Usage: "add a capability requirement to a declarative module",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "root", Usage: "module directory", Value: "modules"},
				},
				Action: func(context *cli.Context) error {
					return RequireSpec(context.String("root"), context.Args().Get(0), context.Args().Get(1), context.Args().Get(2))
				},
			},
			{
				Name:  "show",
				Usage: "show the declarative module contract in developer-facing terms",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "root", Usage: "module directory", Value: "modules"},
				},
				Action: func(context *cli.Context) error {
					output, err := ShowSpec(context.String("root"), context.Args().Get(0))
					if err != nil {
						return err
					}
					fmt.Print(output)
					return nil
				},
			},
			{
				Name:  "new",
				Usage: "create a legacy generated typed module with a custom runtime Build",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name,n", Usage: "module/package name"},
					cli.StringFlag{Name: "root", Usage: "module directory owned by a Go module", Value: "modules"},
					cli.StringFlag{Name: "config-key", Usage: "declared configuration key; defaults to modules.<name>"},
					cli.BoolFlag{Name: "no-config", Usage: "generate without configuration capability"},
					cli.BoolFlag{Name: "no-logger", Usage: "generate without logger capability"},
					cli.StringSliceFlag{Name: "database", Usage: "declared named GORM database; repeatable"},
					cli.BoolFlag{Name: "event-bus", Usage: "declare the application event bus"},
					cli.StringSliceFlag{Name: "rpc", Usage: "declared named gRPC connection; repeatable"},
					cli.StringSliceFlag{Name: "depends-on", Usage: "declared module dependency; repeatable"},
				},
				Action: func(context *cli.Context) error {
					return GenerateWithOptions(Options{
						Name:      context.String("name"),
						Root:      context.String("root"),
						ConfigKey: context.String("config-key"),
						NoConfig:  context.Bool("no-config"),
						Logger:    !context.Bool("no-logger"),
						Databases: context.StringSlice("database"),
						EventBus:  context.Bool("event-bus"),
						RPC:       context.StringSlice("rpc"),
						DependsOn: context.StringSlice("depends-on"),
					})
				},
			},
			{
				Name:  "check",
				Usage: "validate declarative or legacy typed modules",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "root", Usage: "module directory to validate", Value: "modules"},
				},
				Action: func(context *cli.Context) error {
					return Check(context.String("root"))
				},
			},
		},
	}
}
