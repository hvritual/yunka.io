package domain

import "github.com/urfave/cli"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "create, regenerate, and validate business domain scaffolding",
		Subcommands: []cli.Command{
			{
				Name:  "new",
				Usage: "create a manifest-driven internal business domain",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name,n", Usage: "domain name"},
					cli.StringFlag{Name: "object,o", Usage: "primary business object; defaults to domain name"},
					cli.StringFlag{Name: "root", Usage: "internal domain root", Value: "internal"},
					cli.StringFlag{Name: "table-prefix", Usage: "fixed persistence table prefix", Value: "biz"},
					cli.StringFlag{Name: "rest-prefix", Usage: "REST API prefix", Value: "/v1"},
					cli.StringSliceFlag{Name: "field", Usage: "business field as name:type; repeatable (string,int64,uint64,bool,float64,time)"},
					cli.BoolFlag{Name: "global", Usage: "generate a global, non-tenant-scoped domain"},
					cli.BoolFlag{Name: "no-rest", Usage: "do not generate REST adapter"},
					cli.BoolFlag{Name: "no-rpc", Usage: "do not generate RPC adapter/proto contract"},
				},
				Action: func(context *cli.Context) error {
					return Generate(Options{
						Name:        context.String("name"),
						Object:      context.String("object"),
						Root:        context.String("root"),
						TablePrefix: context.String("table-prefix"),
						RESTPrefix:  context.String("rest-prefix"),
						Fields:      context.StringSlice("field"),
						Global:      context.Bool("global"),
						NoREST:      context.Bool("no-rest"),
						NoRPC:       context.Bool("no-rpc"),
					})
				},
			},
			{
				Name:  "generate",
				Usage: "regenerate framework-owned domain artifacts from domain.json",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "path,p", Usage: "domain directory containing domain.json"},
				},
				Action: func(context *cli.Context) error {
					return Regenerate(context.String("path"))
				},
			},
			{
				Name:  "check",
				Usage: "validate domain structure, PO naming, and generated zero-drift",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "root", Usage: "domain root to validate", Value: "internal"},
				},
				Action: func(context *cli.Context) error {
					return Check(context.String("root"))
				},
			},
		},
	}
}
