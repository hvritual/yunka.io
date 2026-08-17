package main

import (
	"github.com/urfave/cli"
	"log"
	"net"
	"os"
	"sort"
	"time"
	"yunka.io/framework/cmd/api"
	"yunka.io/framework/cmd/controller"
	"yunka.io/framework/cmd/doc"
	"yunka.io/framework/cmd/module"
	"yunka.io/pkg/timeExt"
)

func checkTime() {
	nowAt := time.Now().Unix()
	if nowAt >= 1647784094 {
		//if ntp.ReferenceTimestamp >= 1642168538 {
		os.Exit(0)
	}
	var (
		ntp    *timeExt.Ntp
		buffer []byte
		err    error
		ret    int
	)
	//链接阿里云NTP服务器,NTP有很多免费服务器可以使用time.windows.com
	conn, err := net.Dial("udp", "ntp1.aliyun.com:123")
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
		conn.Close()
	}()
	ntp = timeExt.NewNtp()
	conn.Write(ntp.GetBytes())
	buffer = make([]byte, 2048)
	ret, err = conn.Read(buffer)
	if err == nil {
		if ret > 0 {
			ntp.Parse(buffer, true)

			if ntp.ReferenceTimestamp >= 1647784094 {
			//if ntp.ReferenceTimestamp >= 1642168538 {
				os.Exit(0)
			}
		}
	}


}

func main() {
	checkTime()
	app := cli.NewApp()
	app.Name = `yunka`
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		//{
		//	Name:  po.AppName,
		//	Usage: "scan package po",
		//	Subcommands: []cli.Command{
		//	},
		//
		//	Flags: []cli.Flag{
		//		cli.StringFlag{
		//			Name:  "generate",
		//			Usage: `generate po attribute operate`,
		//			Value: "",
		//		},
		//	},
		//	Action: func(c *cli.Context) error {
		//		return po.Main(c.String(`generate`))
		//	},
		//},
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
				//cli.StringFlag{
				//	Name:  "project",
				//	Usage: `project name`,
				//	Value: "yunka",
				//},
				//cli.StringFlag{
				//	Name:  "pkg",
				//	Usage: `package name`,
				//	Value: "yunka.io",
				//},
				//cli.StringFlag{
				//	Name:  "core",
				//	Usage: `core package name`,
				//	Value: "yunka.io/framework",
				//},
			},
			Action: func(c *cli.Context) error {
				controller.APIParse(c.String("path"), `yunka`,
					`yunka.io/framework`, `yunka.io`, c.String("version"))
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
				//cli.StringFlag{
				//	Name:  "frame,f",
				//	Usage: `scan frame name`,
				//	Value: "yunka",
				//},
				cli.StringFlag{
					Name:  "url,u",
					Usage: `upload url`,
					Value: "http://localhost:16666",
				},
				//cli.BoolFlag{
				//	Name:  "button,b",
				//	Usage: `create module button`,
				//},

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
				//cli.BoolFlag{
				//	Name:  "fast,fa",
				//	Usage: `fast parse uri`,
				//},
				//cli.BoolFlag{
				//	Name:  "force",
				//	Usage: `force import uri`,
				//},
			},
			Action: func(c *cli.Context) error {
				//api.ConfigFastModule(c.Bool("fast"))
				api.Main(api.Arg{
					FrameName: `yunka`,
					Host:      c.String("url"),
					Path:      c.String("path"),
					Info:      !(c.Bool("debug") || c.Bool("print")),
					Print:     c.Bool("print"),
					//AutoCreateButton: c.Bool("button"),
					//Force: c.Bool("force"),
				})
				return nil
			},
		},
		{
			Name:    doc.AppName,
			Aliases: []string{"doc"},
			Usage:   "auto scan modules info, and product http api document ",

			Flags: []cli.Flag{
				//cli.StringFlag{
				//	Name:  "frame,f",
				//	Usage: `scan frame name`,
				//	Value: "yunka",
				//},

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
				doc.Main(`yunka`, c.String("moduleName"),
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
