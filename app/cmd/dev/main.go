package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/urfave/cli"
	applicationgraph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/devruntime"
)

const AppName = "dev"

func Command() cli.Command {
	return cli.Command{
		Name:        AppName,
		Usage:       "plan and run explicitly declared local yunka processes",
		Subcommands: []cli.Command{planCommand(), runCommand()},
	}
}

func commonFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "root", Value: "."},
		cli.StringFlag{Name: "config", Value: ".yunka/dev.json"},
		cli.StringFlag{Name: "graph", Value: ".yunka/application-graph.json", Usage: "optional W09 graph for graphNode validation"},
		cli.StringSliceFlag{Name: "target,t"},
	}
}

func planCommand() cli.Command {
	return cli.Command{
		Name: "plan", Usage: "resolve target processes and dependencies without starting them", Flags: append(commonFlags(), cli.StringFlag{Name: "format", Value: "text"}),
		Action: func(c *cli.Context) error {
			plan, err := loadPlan(c)
			if err != nil {
				return err
			}
			if strings.EqualFold(c.String("format"), "json") {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				return encoder.Encode(plan)
			}
			for index, process := range plan.Processes {
				fmt.Printf("%02d %-24s %s\n", index+1, process.Name, strings.Join(process.Command, " "))
			}
			return nil
		},
	}
}

func runCommand() cli.Command {
	return cli.Command{
		Name: "run", Usage: "start the resolved process plan; commands are argv arrays and never executed through a shell", Flags: commonFlags(),
		Action: func(c *cli.Context) error {
			plan, err := loadPlan(c)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return devruntime.Run(ctx, plan, devruntime.RunOptions{Root: c.String("root")})
		},
	}
}

func loadPlan(c *cli.Context) (devruntime.Plan, error) {
	manifest, err := devruntime.LoadDevManifest(c.String("config"))
	if err != nil {
		return devruntime.Plan{}, err
	}
	var graph applicationgraph.Graph
	if path := strings.TrimSpace(c.String("graph")); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			graph, err = applicationgraph.Load(path)
			if err != nil {
				return devruntime.Plan{}, err
			}
		} else if !os.IsNotExist(statErr) {
			return devruntime.Plan{}, statErr
		}
	}
	return devruntime.BuildPlan(manifest, c.String("root"), c.StringSlice("target"), graph)
}
