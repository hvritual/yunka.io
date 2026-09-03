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
	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/devruntime"
)

const AppName = "dev"

func Command() cli.Command {
	return cli.Command{
		Name:        AppName,
		Usage:       "start the declared local runtime, or use plan/run/status for explicit control",
		Flags:       runFlags(),
		Action:      runAction,
		Subcommands: []cli.Command{planCommand(), runCommand(), statusCommand()},
	}
}

func commonFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{Name: "root", Value: "."},
		cli.StringFlag{Name: "config", Value: defaultDevManifest},
		cli.StringFlag{Name: "graph", Value: ".yunka/application-graph.json", Usage: "optional W09 graph for graphNode validation"},
		cli.StringSliceFlag{Name: "target,t"},
		cli.BoolFlag{Name: "closure", Usage: "require complete graph ownership and enable schema-v3 runtime artifacts"},
	}
}

func runFlags() []cli.Flag {
	return append(commonFlags(), cli.StringFlag{
		Name:  "event-format",
		Value: eventFormatText,
		Usage: "runtime evidence output: text or jsonl; jsonl reserves stdout for machine events and sends child output to stderr",
	})
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
		Name: "run", Usage: "start and supervise the resolved process plan; commands are argv arrays and never executed through a shell", Flags: runFlags(),
		Action: runAction,
	}
}

func runAction(c *cli.Context) error {
	plan, err := loadPlan(c)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch strings.ToLower(strings.TrimSpace(c.String("event-format"))) {
	case "", eventFormatText:
		return runWithEvidence(ctx, plan, c.String("root"), os.Stderr, os.Stdout, os.Stderr)
	case eventFormatJSONL:
		return runWithEventStream(ctx, plan, c.String("root"), os.Stdout, os.Stderr)
	default:
		return fmt.Errorf("dev: unsupported event format %q; use text or jsonl", c.String("event-format"))
	}
}

func statusCommand() cli.Command {
	flags := append(commonFlags(),
		cli.StringFlag{Name: "state", Value: devruntime.DefaultRuntimeStatePath},
		cli.StringFlag{Name: "format", Value: "text", Usage: "text, json, or jsonl"},
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
			switch strings.ToLower(strings.TrimSpace(c.String("format"))) {
			case "", "text":
				return devruntime.FormatRuntimeReport(os.Stdout, report)
			case "json":
				encoder := json.NewEncoder(os.Stdout)
				encoder.SetEscapeHTML(false)
				encoder.SetIndent("", "  ")
				return encoder.Encode(report)
			case eventFormatJSONL:
				return renderRuntimeStatusEvents(os.Stdout, closurePlan, statePath, report, c.Bool("closure"))
			default:
				return fmt.Errorf("dev status: unsupported format %q; use text, json, or jsonl", c.String("format"))
			}
		},
	}
}

func loadPlan(c *cli.Context) (devruntime.Plan, error) {
	root := strings.TrimSpace(c.String("root"))
	if root == "" {
		root = "."
	}
	manifestPath, err := resolveDevManifestPath(root, c.String("config"), c.IsSet("config"))
	if err != nil {
		return devruntime.Plan{}, err
	}
	manifest, err := devruntime.LoadDevManifest(cliPath(root, manifestPath))
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
