// Package boundary exposes read-only canonical service-boundary evidence.
package boundary

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "boundary"

type SourcePath struct {
	Canonical   string `json:"canonical"`
	ProjectPath string `json:"projectPath"`
}

type Report struct {
	boundarycore.Inspection
	Project projectflow.ProjectDescriptor `json:"project"`
	// Fingerprint paths retain the compiler's canonical namespace. This explicit
	// mapping reuses projectflow's physical source identity, including inventory.
	Sources []SourcePath `json:"sources"`
}

func Build(ctx context.Context, options projectflow.Options, application string) (Report, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	snapshot, err := projectflow.DescribeContractSourceSnapshot(ctx, options)
	if err != nil {
		return Report{}, err
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	inspection, err := boundarycore.Inspect(snapshot.Manifest, application)
	if err != nil {
		return Report{}, err
	}
	report := Report{Inspection: inspection, Project: snapshot.Project, Sources: []SourcePath{}}
	for _, file := range inspection.Fingerprint.Files {
		path, ok := snapshot.Paths[file.Name]
		if !ok {
			return Report{}, fmt.Errorf("boundary inspect: missing project path for %s", file.Name)
		}
		report.Sources = append(report.Sources, SourcePath{Canonical: file.Name, ProjectPath: path})
	}
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	return report, nil
}

type protoPaths []string

func (p *protoPaths) Set(value string) error { *p = append(*p, value); return nil }
func (p *protoPaths) String() string         { return strings.Join(*p, ", ") }

func Command() cli.Command {
	return cli.Command{
		Name: AppName, Usage: "inspect canonical Service Boundary evidence without granting mutation authority",
		Action: func(c *cli.Context) error {
			return fmt.Errorf("boundary: use inspect [options] <domain>/<application>")
		},
		Subcommands: []cli.Command{{
			Name: "inspect", Usage: "derive a read-only ServiceFingerprint from current protobuf sources", ArgsUsage: "[options] <domain>/<application>",
			Flags: []cli.Flag{
				cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
				cli.StringFlag{Name: "protoc", Usage: "explicit protobuf compiler"},
				cli.GenericFlag{Name: "proto-path", Value: new(protoPaths), Usage: "additional protobuf include directory; repeatable, proto-root projects only"},
				cli.StringFlag{Name: "format", Value: "text", Usage: "text or json"},
			},
			Action: func(c *cli.Context) error {
				if c.NArg() != 1 || strings.TrimSpace(c.Args().First()) == "" {
					return fmt.Errorf("boundary inspect: exactly one <domain>/<application> is required; flags precede the application")
				}
				format := strings.TrimSpace(c.String("format"))
				if format != "json" && format != "text" {
					return fmt.Errorf("boundary inspect: format must be text or json")
				}
				if strings.TrimSpace(c.String("root")) == "" {
					return fmt.Errorf("boundary inspect: root must not be blank")
				}
				if c.IsSet("protoc") && strings.TrimSpace(c.String("protoc")) == "" {
					return fmt.Errorf("boundary inspect: protoc must not be blank")
				}
				paths, ok := c.Generic("proto-path").(*protoPaths)
				if !ok || paths == nil {
					return fmt.Errorf("boundary inspect: missing include flag")
				}
				report, err := Build(context.Background(), projectflow.Options{Root: c.String("root"), Protoc: c.String("protoc"), ProtoPaths: append([]string(nil), (*paths)...)}, c.Args().First())
				if err != nil {
					return err
				}
				if format == "json" {
					data, err := json.MarshalIndent(report, "", "  ")
					if err != nil {
						return err
					}
					_, err = fmt.Fprintln(c.App.Writer, string(data))
					return err
				}
				_, err = fmt.Fprint(c.App.Writer, FormatText(report))
				return err
			},
		}},
	}
}

func FormatText(report Report) string {
	var b strings.Builder
	f := report.Fingerprint
	fmt.Fprintf(&b, "APPLICATION %s\nSERVICE %s\nAUTHORITY %s\nFINGERPRINT %s\n", f.Application, f.Service, report.Authority, report.FingerprintDigest)
	fmt.Fprintf(&b, "OPERATIONS %d\nINTENT COVERAGE %s declared=%d unknown=%d\n", len(f.Operations), report.IntentCoverage.State, len(report.IntentCoverage.DeclaredOperations), len(report.IntentCoverage.UnknownOperations))
	for _, op := range f.Operations {
		fmt.Fprintf(&b, "OPERATION %s", op.Plan.OperationID)
		if op.Boundary == nil {
			b.WriteString(" boundary=unknown\n")
			continue
		}
		fmt.Fprintf(&b, " context=%s", op.Boundary.Context)
		if op.Boundary.Aggregate != "" {
			fmt.Fprintf(&b, " aggregate=%s\n", op.Boundary.Aggregate)
		} else {
			fmt.Fprintf(&b, " aggregate=not_applicable reason=%q\n", op.Boundary.AggregateNotApplicableReason)
		}
	}
	for _, source := range report.Sources {
		fmt.Fprintf(&b, "SOURCE %s\n", source.ProjectPath)
	}
	b.WriteString("No boundary decision or mutation authorization is issued.\n")
	return b.String()
}
