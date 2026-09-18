package boundary

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

const AppName = "boundary"

func Command() cli.Command {
	return cli.Command{
		Name:  AppName,
		Usage: "inspect canonical Service Boundary evidence without mutating the project",
		Subcommands: []cli.Command{
			inspectCommand(),
		},
	}
}

func inspectCommand() cli.Command {
	return cli.Command{
		Name:  "inspect",
		Usage: "compile current canonical protobuf sources and project one Application boundary",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "application", Usage: "canonical Application identity in <domain>/<application> form"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary used to compile current canonical sources"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protobuf include path; may be repeated"},
			cli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			application := strings.TrimSpace(c.String("application"))
			if application == "" {
				return fmt.Errorf("boundary inspect: --application is required")
			}
			inspection, err := Build(context.Background(), projectflow.Options{
				Root:       c.String("root"),
				Protoc:     c.String("protoc"),
				ProtoPaths: append([]string(nil), c.StringSlice("proto-path")...),
			}, application)
			if err != nil {
				return err
			}
			output, err := Render(inspection, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

// Build is a read-only adapter over the canonical source compiler and
// boundarycore.Inspect. It never reads generated manifest artifacts and never
// persists the inspection.
func Build(ctx context.Context, options projectflow.Options, application string) (boundarycore.Inspection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	snapshot, err := projectflow.DescribeContractSourceSnapshot(ctx, options)
	if err != nil {
		return boundarycore.Inspection{}, fmt.Errorf("boundary inspect: compile current canonical source: %w", err)
	}
	inspection, err := boundarycore.Inspect(snapshot.Manifest, application)
	if err != nil {
		return boundarycore.Inspection{}, err
	}
	return inspection, nil
}

func Render(inspection boundarycore.Inspection, format string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return renderText(inspection), nil
	case "json", "agent-json":
		contents, err := json.MarshalIndent(inspection, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	default:
		return "", fmt.Errorf("boundary inspect: unsupported format %q; use text, json, or agent-json", format)
	}
}

func renderText(inspection boundarycore.Inspection) string {
	fingerprint := inspection.Fingerprint
	var builder strings.Builder
	fmt.Fprintf(&builder, "BOUNDARY authority=%s application=%s service=%s source=%s\n",
		inspection.Authority, fingerprint.Application, fingerprint.Service, fingerprint.ServiceSource)
	fmt.Fprintf(&builder, "DIGEST fingerprint=%s operationPlans=%s\n",
		inspection.FingerprintDigest, inspection.OperationPlansDigest)
	fmt.Fprintf(&builder, "INTENT state=%s declared=%d unknown=%d contexts=%s aggregates=%s\n",
		inspection.IntentCoverage.State,
		len(inspection.IntentCoverage.DeclaredOperations),
		len(inspection.IntentCoverage.UnknownOperations),
		joinOrDash(inspection.IntentCoverage.Contexts),
		joinOrDash(inspection.IntentCoverage.Aggregates),
	)
	fmt.Fprintf(&builder, "OPERATIONS %d\n", len(fingerprint.Operations))
	for _, operation := range fingerprint.Operations {
		contextValue, aggregateValue := operationIntent(operation)
		fmt.Fprintf(&builder, "  %s context=%q aggregate=%q transaction=%s idempotency=%s public=%t\n",
			operation.Plan.OperationID,
			contextValue,
			aggregateValue,
			operation.Plan.Execution.Transaction,
			operation.Plan.Execution.Idempotency,
			operation.Plan.Security.Public,
		)
	}
	fmt.Fprintf(&builder, "DEPENDENCIES %d\n", len(fingerprint.DependencyOperations))
	for _, operation := range fingerprint.DependencyOperations {
		contextValue, aggregateValue := operationIntent(operation)
		fmt.Fprintf(&builder, "  %s context=%q aggregate=%q application=%s/%s\n",
			operation.Plan.OperationID,
			contextValue,
			aggregateValue,
			operation.Plan.Domain,
			operation.Plan.Application,
		)
	}
	notEvaluated := append([]string(nil), inspection.NotEvaluated...)
	sort.Strings(notEvaluated)
	fmt.Fprintf(&builder, "NOT_EVALUATED %s\n", joinOrDash(notEvaluated))
	return builder.String()
}

func operationIntent(operation boundarycore.OperationEvidence) (string, string) {
	if operation.Boundary == nil {
		return "<unknown>", "<unknown>"
	}
	aggregate := strings.TrimSpace(operation.Boundary.Aggregate)
	if aggregate == "" {
		aggregate = "n/a:" + strings.TrimSpace(operation.Boundary.AggregateNotApplicableReason)
	}
	return strings.TrimSpace(operation.Boundary.Context), aggregate
}

func joinOrDash(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ",")
}
