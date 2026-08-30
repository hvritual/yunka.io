package dev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/urfave/cli"
	applicationgraph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/contract"
	"yunka.io/pkg/devruntime"
)

const AppName = "dev"

func Command() cli.Command {
	return cli.Command{
		Name:        AppName,
		Usage:       "plan, run, and inspect explicitly declared local yunka processes",
		Subcommands: []cli.Command{planCommand(), runCommand(), statusCommand()},
	}
}

func commonFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "root", Value: "."},
		cli.StringFlag{Name: "config", Value: ".yunka/dev.json"},
		cli.StringFlag{Name: "graph", Value: ".yunka/application-graph.json", Usage: "optional W09 graph for graphNode validation"},
		cli.StringSliceFlag{Name: "target,t"},
		cli.BoolFlag{Name: "closure", Usage: "require complete graph ownership and enable schema-v3 runtime artifacts"},
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
		Name: "run", Usage: "start and supervise the resolved process plan; commands are argv arrays and never executed through a shell", Flags: commonFlags(),
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

func statusCommand() cli.Command {
	flags := append(commonFlags(),
		cli.StringFlag{Name: "state", Value: devruntime.DefaultRuntimeStatePath},
		cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
	)
	return cli.Command{
		Name:  "status",
		Usage: "inspect the secret-free local runtime report and optionally require a complete live closure",
		Flags: flags,
		Action: func(c *cli.Context) error {
			root := strings.TrimSpace(c.String("root"))
			if root == "" {
				root = "."
			}
			statePath := strings.TrimSpace(c.String("state"))
			var closurePlan devruntime.Plan
			if c.Bool("closure") {
				plan, err := loadPlan(c)
				if err != nil {
					return err
				}
				if plan.Runtime == nil {
					return fmt.Errorf("dev status closure requires runtime configuration")
				}
				closurePlan = plan
				canonicalState := strings.TrimSpace(plan.Runtime.StatePath)
				if canonicalState == "" {
					canonicalState = devruntime.DefaultRuntimeStatePath
				}
				if statePath != "" && statePath != devruntime.DefaultRuntimeStatePath && filepath.Clean(statePath) != filepath.Clean(canonicalState) {
					return fmt.Errorf("dev status closure state path %q does not match runtime statePath %q", statePath, canonicalState)
				}
				statePath = canonicalState
			}
			if statePath == "" {
				statePath = devruntime.DefaultRuntimeStatePath
			}
			report, err := devruntime.LoadRuntimeReport(root, statePath)
			if err != nil {
				return err
			}
			if c.Bool("closure") {
				if err := devruntime.ValidateRuntimeClosure(closurePlan, report); err != nil {
					return err
				}
			}
			if strings.EqualFold(c.String("format"), "json") {
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetEscapeHTML(false)
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			}
			return devruntime.FormatRuntimeReport(os.Stdout, report)
		},
	}
}

func loadPlan(c *cli.Context) (devruntime.Plan, error) {
	root := strings.TrimSpace(c.String("root"))
	if root == "" {
		root = "."
	}
	manifest, err := devruntime.LoadDevManifest(cliPath(root, c.String("config")))
	if err != nil {
		return devruntime.Plan{}, err
	}
	var graph applicationgraph.Graph
	graphPath := strings.TrimSpace(c.String("graph"))
	if graphPath != "" {
		graphPath = cliPath(root, graphPath)
		if _, statErr := os.Stat(graphPath); statErr == nil {
			graph, err = applicationgraph.Load(graphPath)
			if err != nil {
				return devruntime.Plan{}, err
			}
		} else if !os.IsNotExist(statErr) {
			return devruntime.Plan{}, statErr
		}
	}

	closure := c.Bool("closure")
	if manifest.Runtime != nil && manifest.Runtime.Closure {
		closure = true
	}
	runtimeEnabled := manifest.SchemaVersion >= devruntime.RuntimeClosureSchemaVersion || closure
	if runtimeEnabled && len(graph.Nodes) == 0 {
		contractPath := devruntime.DefaultRuntimeContractManifest
		if manifest.Runtime != nil && strings.TrimSpace(manifest.Runtime.ContractManifest) != "" {
			contractPath = manifest.Runtime.ContractManifest
		}
		manifestContract, loadErr := contract.LoadManifest(cliPath(root, contractPath))
		if loadErr != nil {
			if closure {
				return devruntime.Plan{}, fmt.Errorf("dev runtime closure contract graph: %w", loadErr)
			}
		} else {
			builder := applicationgraph.NewBuilder()
			if err := applicationgraph.AddContract(builder, manifestContract); err != nil {
				return devruntime.Plan{}, err
			}
			graph, err = builder.Build()
			if err != nil {
				return devruntime.Plan{}, err
			}
		}
	}
	return devruntime.BuildPlanWithOptions(manifest, root, c.StringSlice("target"), graph, devruntime.PlanOptions{Closure: c.Bool("closure")})
}

func cliPath(root, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(root, value)
}
