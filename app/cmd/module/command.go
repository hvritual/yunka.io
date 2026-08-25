package module

import "github.com/urfave/cli"

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
				Name:  "new",
				Usage: "create a deterministic typed module",
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
				Usage: "validate typed module and autoload structure",
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
