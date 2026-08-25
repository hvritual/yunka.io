package domain

import "github.com/urfave/cli"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "compile PO-first business domains into persistence, Service, REST, and pinned gRPC code",
		Subcommands: []cli.Command{
			{
				Name:  "new",
				Usage: "create/adopt a domain, auto-scan one snake_case file per *PO object, and compile the full stack",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name,n", Usage: "domain name"},
					cli.StringFlag{Name: "object,o", Usage: "optional bootstrap PO object when no PO files exist; defaults to domain name"},
					cli.StringFlag{Name: "root", Usage: "internal domain root", Value: "internal"},
					cli.StringFlag{Name: "rest-prefix", Usage: "REST API prefix", Value: "/v1"},
					cli.StringSliceFlag{Name: "field", Usage: "optional bootstrap PO field name:type; normal development scans fields from PO files"},
					cli.BoolFlag{Name: "global", Usage: "generate a global, non-tenant-scoped domain"},
					cli.BoolFlag{Name: "no-rest", Usage: "do not generate REST adapter"},
					cli.BoolFlag{Name: "no-rpc", Usage: "do not generate domain.proto, pinned protobuf/gRPC code, or typed registration bridge"},
				},
				Action: func(context *cli.Context) error {
					return Generate(Options{
						Name:       context.String("name"),
						Object:     context.String("object"),
						Root:       context.String("root"),
						RESTPrefix: context.String("rest-prefix"),
						Fields:     context.StringSlice("field"),
						Global:     context.Bool("global"),
						NoREST:     context.Bool("no-rest"),
						NoRPC:      context.Bool("no-rpc"),
					})
				},
			},
			{
				Name:  "generate",
				Usage: "rescan PO objects/fields and regenerate Service, REST, domain.proto, pinned pb/grpc, bridge/register, and wiring",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "path,p", Usage: "domain directory containing domain.json"},
				},
				Action: func(context *cli.Context) error {
					return Regenerate(context.String("path"))
				},
			},
			{
				Name:  "check",
				Usage: "validate PO scan contract, snake_case files, project naming, pinned protobuf/gRPC zero-drift, and all generated artifacts",
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
