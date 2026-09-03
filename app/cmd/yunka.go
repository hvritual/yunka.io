package main

import (
	"fmt"
	"github.com/urfave/cli"
	"log"
	"os"
	"sort"
	"yunka.io/app/cmd/agentcontext"
	"yunka.io/app/cmd/api"
	"yunka.io/app/cmd/assembly"
	"yunka.io/app/cmd/check"
	"yunka.io/app/cmd/contract"
	"yunka.io/app/cmd/dependency"
	"yunka.io/app/cmd/dev"
	"yunka.io/app/cmd/doc"
	"yunka.io/app/cmd/doctor"
	"yunka.io/app/cmd/domain"
	"yunka.io/app/cmd/explain"
	"yunka.io/app/cmd/generate"
	"yunka.io/app/cmd/graph"
	"yunka.io/app/cmd/inspect"
	"yunka.io/app/cmd/module"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/project"
)

func main() {
	app := cli.NewApp()
	app.Name = `yunka`
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		agentcontext.Command(),
		assembly.Command(),
		check.Command(),
		contract.Command(),
		dependency.Command(),
		dev.Command(),
		doctor.Command(),
		domain.Command(),
		explain.Command(),
		generate.Command(),
		graph.Command(),
		project.Command(),
		inspect.Command(),
		module.Command(),
		ownership.Command(),
		{
			Name:    api.AppName,
			Aliases: []string{"a"},
			Usage:   "auto scan module info, and import api to system, ",

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "frame,f",
					Usage: `scan frame name`,
					Value: "yunka",
				},
				cli.StringFlag{
					Name:  "url,u",
					Usage: `upload url`,
					Value: "http://localhost:16666",
				},
				cli.StringFlag{
					Name:   "api-key",
					Usage:  `32-byte API authentication key`,
					EnvVar: "YUNKA_API_KEY",
				},
				cli.BoolFlag{
					Name:  "button,b",
					Usage: `create module button`,
				},

				cli.StringFlag{
					Name:  "path",
					Usage: `scan path`,
					Value: "./",
				},

				cli.BoolFlag{
					Name:  "debug,d",
					Usage: `output debug info`,
				},
				cli.BoolFlag{
					Name:  "print,p",
					Usage: `output print info`,
				},
				cli.BoolFlag{
					Name:  "fast,fa",
					Usage: `fast parse uri`,
				},
				cli.BoolFlag{
					Name:  "force",
					Usage: `force import uri`,
				},
			},
			Action: func(c *cli.Context) error {
				api.ConfigFastModule(c.Bool("fast"))
				api.Main(api.Arg{
					FrameName:        c.String("frame"),
					Host:             c.String("url"),
					Path:             c.String("path"),
					Info:             !(c.Bool("debug") || c.Bool("print")),
					Print:            c.Bool("print"),
					AutoCreateButton: c.Bool("button"),
					Force:            c.Bool("force"),
					APIKey:           c.String("api-key"),
				})
				return nil
			},
		},
		{
			Name:    doc.AppName,
			Aliases: []string{"doc"},
			Usage:   "auto scan modules info, and product http api document ",

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "frame,f",
					Usage: `scan frame name`,
					Value: "yunka",
				},

				cli.StringFlag{
					Name:  "path",
					Usage: `scan path`,
					Value: "./",
				},
				cli.StringFlag{
					Name:  "moduleName",
					Usage: `doc store directory name`,
					Value: "modules",
				},
				cli.StringFlag{
					Name:  "apiVersion,av",
					Usage: `api doc version`,
					Value: "1.0.1",
				},
				cli.BoolFlag{
					Name:  "print,p",
					Usage: `output print info`,
				},

				cli.BoolFlag{
					Name:  "fast,fa",
					Usage: `fast parse uri`,
				},
			},
			Action: func(c *cli.Context) error {
				doc.Main(c.String("frame"), c.String("moduleName"),
					c.String("path"),
					c.String("apiVersion"),
					c.Bool("print"))
				return nil
			},
		},
	}

	sort.Sort(cli.CommandsByName(app.Commands))
	applyDiscoverability(app)

	err := app.Run(os.Args)
	if err != nil {
		if message, code, ok := classifyCLIExit(err); ok {
			if message != "" {
				fmt.Fprintln(os.Stderr, message)
			}
			os.Exit(code)
		}
		log.Fatal(err)
	}
}
