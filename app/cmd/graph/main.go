package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli"
	applicationgraph "yunka.io/pkg/applicationgraph"
	"yunka.io/pkg/contract"
	"yunka.io/pkg/operationplan"
)

const AppName = "graph"

func Command() cli.Command {
	return cli.Command{
		Name:        AppName,
		Usage:       "build and query the evidence-backed yunka application graph",
		Subcommands: []cli.Command{buildCommand(), inspectCommand(), findCommand(), impactCommand()},
	}
}

func buildCommand() cli.Command {
	return cli.Command{
		Name:  "build",
		Usage: "compile contract and optional runtime diagnostics into an application graph",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "manifest", Value: "contracts/generated/manifest.json"},
			cli.StringFlag{Name: "operation-plans", Value: "contracts/generated/operation-plans.json"},
			cli.StringFlag{Name: "diagnostics", Usage: "optional W07 diagnostics JSON file"},
			cli.StringFlag{Name: "application", Value: "yunka"},
			cli.StringFlag{Name: "out", Value: ".yunka/application-graph.json"},
		},
		Action: func(c *cli.Context) error {
			manifest, err := contract.LoadManifest(c.String("manifest"))
			if err != nil {
				return err
			}
			plans, err := operationplan.Load(c.String("operation-plans"))
			if err != nil {
				return err
			}
			builder := applicationgraph.NewBuilder()
			if err := applicationgraph.AddContract(builder, manifest); err != nil {
				return err
			}
			if err := applicationgraph.AddOperationPlans(builder, plans); err != nil {
				return err
			}
			if path := strings.TrimSpace(c.String("diagnostics")); path != "" {
				snapshot, err := loadRuntimeSnapshot(path)
				if err != nil {
					return err
				}
				if err := applicationgraph.AddRuntime(builder, snapshot, c.String("application")); err != nil {
					return err
				}
			}
			graph, err := builder.Build()
			if err != nil {
				return err
			}
			return writeGraph(c.String("out"), graph)
		},
	}
}

func inspectCommand() cli.Command {
	return cli.Command{
		Name:  "inspect",
		Usage: "summarize a compiled application graph",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "graph", Value: ".yunka/application-graph.json"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
		},
		Action: func(c *cli.Context) error {
			current, err := applicationgraph.Load(c.String("graph"))
			if err != nil {
				return err
			}
			if strings.EqualFold(c.String("format"), "json") {
				return applicationgraph.Encode(os.Stdout, current)
			}
			counts := make(map[applicationgraph.NodeKind]int)
			evidence := make(map[applicationgraph.EvidenceType]int)
			for _, node := range current.Nodes {
				counts[node.Kind]++
				for _, item := range node.Evidence {
					evidence[item.Type]++
				}
			}
			kinds := make([]string, 0, len(counts))
			for kind := range counts {
				kinds = append(kinds, string(kind))
			}
			sort.Strings(kinds)
			fmt.Printf("graph schema=%d nodes=%d edges=%d declared=%d observed=%d inferred=%d\n", current.SchemaVersion, len(current.Nodes), len(current.Edges), evidence[applicationgraph.EvidenceDeclared], evidence[applicationgraph.EvidenceObserved], evidence[applicationgraph.EvidenceInferred])
			for _, kind := range kinds {
				fmt.Printf("%-20s %d\n", kind, counts[applicationgraph.NodeKind(kind)])
			}
			return nil
		},
	}
}

func findCommand() cli.Command {
	return cli.Command{
		Name:  "find",
		Usage: "find graph nodes by ID, name or attribute",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "graph", Value: ".yunka/application-graph.json"},
			cli.StringFlag{Name: "query,q"},
			cli.StringFlag{Name: "kind"},
			cli.StringFlag{Name: "format", Value: "text"},
		},
		Action: func(c *cli.Context) error {
			current, err := applicationgraph.Load(c.String("graph"))
			if err != nil {
				return err
			}
			nodes := current.Find(c.String("query"), applicationgraph.NodeKind(strings.TrimSpace(c.String("kind"))))
			if strings.EqualFold(c.String("format"), "json") {
				return writeJSON(os.Stdout, nodes)
			}
			for _, node := range nodes {
				fmt.Printf("%-22s %s\n", node.Kind, node.ID)
			}
			if len(nodes) == 0 {
				return cli.NewExitError("no matching graph nodes", 1)
			}
			return nil
		},
	}
}

func impactCommand() cli.Command {
	return cli.Command{
		Name:  "impact",
		Usage: "show dependencies and dependents for one graph node",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "graph", Value: ".yunka/application-graph.json"},
			cli.StringFlag{Name: "id", Usage: "exact graph node ID"},
			cli.IntFlag{Name: "depth", Value: 3},
			cli.StringFlag{Name: "format", Value: "text"},
		},
		Action: func(c *cli.Context) error {
			if strings.TrimSpace(c.String("id")) == "" {
				return errors.New("graph impact: --id is required")
			}
			current, err := applicationgraph.Load(c.String("graph"))
			if err != nil {
				return err
			}
			report, err := current.Impact(c.String("id"), c.Int("depth"))
			if err != nil {
				return err
			}
			if strings.EqualFold(c.String("format"), "json") {
				return writeJSON(os.Stdout, report)
			}
			fmt.Printf("target %s [%s]\n", report.Target.ID, report.Target.Kind)
			printImpact("dependencies", report.Dependencies)
			printImpact("dependents", report.Dependents)
			return nil
		},
	}
}

func printImpact(title string, entries []applicationgraph.ImpactEntry) {
	fmt.Printf("%s:\n", title)
	if len(entries) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, entry := range entries {
		fmt.Printf("  d=%d via=%s %s\n", entry.Depth, entry.Via, entry.Node.ID)
	}
}

func loadRuntimeSnapshot(path string) (applicationgraph.RuntimeSnapshot, error) {
	var envelope struct {
		Core applicationgraph.RuntimeSnapshot `json:"core"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return applicationgraph.RuntimeSnapshot{}, err
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return applicationgraph.RuntimeSnapshot{}, err
	}
	return envelope.Core, nil
}

func writeGraph(path string, graph applicationgraph.Graph) error {
	path = strings.TrimSpace(path)
	if path == "" || path == "-" {
		return applicationgraph.Encode(os.Stdout, graph)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return applicationgraph.Encode(file, graph)
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
