package main

import (
	"github.com/urfave/cli"
	"log"
	"os"
	"sort"
	"yunka.io/app/cmd/api"
	"yunka.io/app/cmd/contract"
	"yunka.io/app/cmd/controller"
	"yunka.io/app/cmd/doc"
	"yunka.io/app/cmd/graph"
	"yunka.io/app/cmd/inspect"
	"yunka.io/app/cmd/module"
	"yunka.io/app/cmd/po"
)

func main() {
	app := cli.NewApp()
	app.Name = `yunka`
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		contract.Command(),
		graph.Command(),
		inspect.Command(),
		{
			Name:        po.AppName,
			Usage:       "scan package po",
			Subcommands: []cli.Command{},

			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "generate",
					Usage: `generate po attribute operate`,
					Value: "",
				},
			},
			Action: func(c *cli.Context) error {
				return po.Main(c.String(`generate`))
			},
		},
		{
			Name:  controller.AppName,
			Usage: "auto scan module info, produce router&register file",
			Flags: []cli.Flag{

				cli.StringFlag{
					Name:  "path",
					Usage: `scan path`,
					Value: "./",
				},
				cli.StringFlag{
					Name:  "version",
					Usage: `url version`,
					Value: "v1",
				},
				cli.StringFlag{
					Name:  "project",
					Usage: `project name`,
					Value: "yunka",
				},
				cli.StringFlag{
					Name:  "pkg",
					Usage: `package name`,
					Value: "yunka.io",
				},
				cli.StringFlag{
					Name:  "core",
					Usage: `core package name`,
					Value: "yunka.io/framework",
				},
			},
			Action: func(c *cli.Context) error {
				controller.APIParse(c.String("path"), c.String("project"),
					c.String("core"), c.String("pkg"), c.String("version"))
				return nil
			},
		},
		{
			Name:  module.AppName,
			Usage: "module layout",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name",
					Usage: `module name`,
					Value: "org",
				},
			},
			Action: func(c *cli.Context) error {
				return module.Generate(c.String("name"))
			},
		},
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

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
