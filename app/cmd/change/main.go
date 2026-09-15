package change

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hvritual/yunka.io/pkg/diagnostic"
	"github.com/urfave/cli"
)

const AppName = "change"

const (
	FailureUsage    = "usage"
	FailureEvidence = "evidence"
	FailurePolicy   = "policy"
	FailureNoop     = "noop"
)

type Failure struct {
	Kind string
	Err  error
}

func (failure *Failure) Error() string {
	if failure == nil || failure.Err == nil {
		return "change plan failed"
	}
	return failure.Err.Error()
}

func (failure *Failure) Unwrap() error {
	if failure == nil {
		return nil
	}
	return failure.Err
}

func Command() cli.Command {
	return cli.Command{
		Name:        AppName,
		Usage:       "plan, constrain, verify, and review evidence-backed bounded changes",
		Subcommands: []cli.Command{planCommand(), beginCommand(), checkCommand(), verifyCommand(), reviewCommand(), qualityWaiverCommand(), setCommand()},
	}
}

func planCommand() cli.Command {
	return cli.Command{
		Name:  "plan",
		Usage: "derive impact, mutation targets, generated effects, and verification gates without changing the project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			sourceProtocFlag(), sourceIncludesFlag(),
			cli.StringFlag{Name: "operation", Usage: "exact canonical operation ID or operation:<ID> graph node ID"},
			cli.StringFlag{Name: "intent", Value: IntentBoth, Usage: "change intent: contract, implementation, or both"},
			cli.IntFlag{Name: "depth", Value: 3, Usage: "maximum static graph impact depth"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format == "" {
				format = FormatText
			}
			if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
				item := diagnostic.MustDefinition(diagnostic.CodeUnsupportedOutputFormat).Diagnostic(diagnostic.SeverityError)
				item.Detail = fmt.Sprintf("format %q is unsupported; use text, json, or agent-json", format)
				return printFailure("yunka change plan", format, item, 2)
			}
			plan, err := BuildPlan(Options{
				Root:          c.String("root"),
				Operation:     c.String("operation"),
				Intent:        c.String("intent"),
				Depth:         c.Int("depth"),
				Protoc:        c.String("protoc"),
				ProtoPaths:    c.StringSlice("proto-path"),
				ProtoPathFile: c.String("proto-path-file"),
			})
			if err != nil {
				item := Diagnose(err)
				code := 1
				var failure *Failure
				if errors.As(err, &failure) && failure.Kind == FailureUsage {
					code = 2
				}
				return printFailure("yunka change plan", format, item, code)
			}
			if err := Render(plan, format, os.Stdout); err != nil {
				item := diagnostic.MustDefinition(diagnostic.CodeChangeEvidence).Diagnostic(diagnostic.SeverityError)
				item.Stage = "render"
				item.Detail = err.Error()
				return printFailure("yunka change plan", format, item, 1)
			}
			if !plan.Conformant {
				return cli.NewExitError("", 1)
			}
			return nil
		},
	}
}

func Render(plan Plan, format string, writer io.Writer) error {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = FormatText
	}
	switch format {
	case FormatJSON, FormatAgentJSON:
		contents, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(writer, string(contents))
		return err
	case FormatText:
		_, err := fmt.Fprint(writer, renderText(plan))
		return err
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func renderText(plan Plan) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "CHANGE PLAN operation=%s intent=%s base=%s conformant=%t\n", plan.Operation.OperationID, plan.Intent, plan.BaseSHA, plan.Conformant)
	fmt.Fprintf(&builder, "IMPACT depth=%d nodes=%d\n", plan.Impact.Depth, len(plan.Impact.Nodes))
	for _, node := range plan.Impact.Nodes {
		fmt.Fprintf(&builder, "  %s %s distance=%d via=%s\n", node.Kind, node.ID, node.Distance, node.Via)
	}
	fmt.Fprintf(&builder, "EDITABLE paths=%d scopes=%d\n", len(plan.Mutation.EditablePaths), len(plan.Mutation.EditableScopes))
	for _, path := range plan.Mutation.EditablePaths {
		fmt.Fprintf(&builder, "  path  %s\n", path)
	}
	for _, scope := range plan.Mutation.EditableScopes {
		fmt.Fprintf(&builder, "  scope %s\n", scope)
	}
	fmt.Fprintf(&builder, "GENERATED paths=%d scopes=%d\n", len(plan.Generated.GeneratedPaths), len(plan.Generated.GeneratedScopes))
	for _, path := range plan.Generated.GeneratedPaths {
		fmt.Fprintf(&builder, "  path  %s\n", path)
	}
	for _, scope := range plan.Generated.GeneratedScopes {
		fmt.Fprintf(&builder, "  scope %s\n", scope)
	}
	fmt.Fprintf(&builder, "FORBIDDEN %d\n", len(plan.Forbidden))
	for _, forbidden := range plan.Forbidden {
		fmt.Fprintf(&builder, "  %s\n", forbidden)
	}
	fmt.Fprintf(&builder, "VERIFY %d\n", len(plan.Verification))
	for _, gate := range plan.Verification {
		fmt.Fprintf(&builder, "  %s — %s\n", gate.Command, gate.Reason)
	}
	if len(plan.Blockers) > 0 {
		fmt.Fprintf(&builder, "BLOCKERS %d\n", len(plan.Blockers))
		for _, blocker := range plan.Blockers {
			fmt.Fprintf(&builder, "  %s %s — %s\n", blocker.Code, blocker.Subject, blocker.Detail)
		}
	}
	return builder.String()
}

func printFailure(tool, format string, item diagnostic.Diagnostic, code int) error {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = FormatText
	}
	if format == FormatJSON || format == FormatAgentJSON {
		contents, err := json.MarshalIndent(struct {
			Tool        string                `json:"tool"`
			Conformant  bool                  `json:"conformant"`
			Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
		}{Tool: tool, Conformant: false, Diagnostics: []diagnostic.Diagnostic{item}}, "", "  ")
		if err == nil {
			fmt.Fprintln(os.Stderr, string(contents))
		}
	} else {
		fmt.Fprintf(os.Stderr, "%s: %s — %s\n", tool, item.Code, item.Detail)
	}
	return cli.NewExitError("", code)
}
