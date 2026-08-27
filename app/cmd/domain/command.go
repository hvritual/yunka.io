package domain

import "github.com/urfave/cli"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "compile developer-owned PO structs into entities and basic repository CRUD code",
		Subcommands: []cli.Command{
			{
				Name:  "new",
				Usage: "create/adopt a domain, scan one snake_case file per *PO object, and generate persistence-only domain artifacts",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "name,n", Usage: "domain name"},
					cli.StringFlag{Name: "object,o", Usage: "optional bootstrap PO object when no PO files exist; defaults to domain name"},
					cli.StringFlag{Name: "root", Usage: "internal domain root", Value: "internal"},
					cli.StringSliceFlag{Name: "field", Usage: "optional bootstrap PO field name:type; normal development scans fields from PO files"},
					cli.BoolFlag{Name: "global", Usage: "generate a global, non-tenant-scoped domain"},
				},
				Action: func(context *cli.Context) error {
					return Generate(Options{
						Name:   context.String("name"),
						Object: context.String("object"),
						Root:   context.String("root"),
						Fields: context.StringSlice("field"),
						Global: context.Bool("global"),
					})
				},
			},
			{
				Name:  "generate",
				Usage: "rescan PO objects/fields and regenerate Entity plus basic Repository CRUD artifacts only",
				Flags: []cli.Flag{
					cli.StringFlag{Name: "path,p", Usage: "domain directory containing domain.json"},
				},
				Action: func(context *cli.Context) error {
					return Regenerate(context.String("path"))
				},
			},
			{
				Name:  "check",
				Usage: "validate PO scan contract, persistence-only Domain Manifest V3, project naming, and generated repository artifacts",
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
