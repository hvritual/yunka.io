package explain

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

const (
	AppName       = "explain"
	SchemaVersion = 1
	FormatText    = "text"
	FormatJSON    = "json"
)

type Result struct {
	Output   string
	ExitCode int
}

type envelope struct {
	SchemaVersion int                    `json:"schemaVersion"`
	OK            bool                   `json:"ok"`
	Code          string                 `json:"code"`
	Definition    *diagnostic.Definition `json:"definition,omitempty"`
	Error         string                 `json:"error,omitempty"`
}

func Command() cli.Command {
	return cli.Command{
		Name:      AppName,
		Usage:     "explain a stable YUNKA-DX diagnostic code",
		ArgsUsage: "<CODE>",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text or json"},
		},
		Action: func(c *cli.Context) error {
			if len(c.Args()) != 1 {
				fmt.Print("explain: exactly one diagnostic code is required\n")
				return cli.NewExitError("", 2)
			}
			result, err := Build(c.Args().First(), c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(result.Output)
			if result.ExitCode != 0 {
				return cli.NewExitError("", result.ExitCode)
			}
			return nil
		},
	}
}

func Build(code, format string) (Result, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = FormatText
	}
	if format != FormatText && format != FormatJSON {
		item := diagnostic.MustDefinition(diagnostic.CodeUnsupportedOutputFormat).Diagnostic(diagnostic.SeverityError)
		item.Detail = fmt.Sprintf("format %q is unsupported; use text or json", format)
		text, err := diagnostic.RenderText([]diagnostic.Diagnostic{item})
		if err != nil {
			return Result{}, err
		}
		return Result{Output: text, ExitCode: 2}, nil
	}

	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	if normalizedCode == "" {
		return Result{Output: "explain: diagnostic code is required\n", ExitCode: 2}, nil
	}
	definition, ok := diagnostic.LookupDefinition(normalizedCode)
	if !ok {
		if format == FormatJSON {
			return renderJSON(envelope{
				SchemaVersion: SchemaVersion,
				OK:            false,
				Code:          normalizedCode,
				Error:         "unknown diagnostic code",
			}, 1)
		}
		return Result{Output: fmt.Sprintf("unknown diagnostic code: %s\n", normalizedCode), ExitCode: 1}, nil
	}

	if format == FormatJSON {
		copy := definition
		return renderJSON(envelope{
			SchemaVersion: SchemaVersion,
			OK:            true,
			Code:          definition.Code,
			Definition:    &copy,
		}, 0)
	}
	return Result{Output: renderText(definition)}, nil
}

func renderJSON(value envelope, exitCode int) (Result, error) {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Result{}, err
	}
	return Result{Output: string(append(contents, '\n')), ExitCode: exitCode}, nil
}

func renderText(definition diagnostic.Definition) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s\n", definition.Code)
	fmt.Fprintf(&builder, "  stage:    %s\n", definition.Stage)
	fmt.Fprintf(&builder, "  meaning:  %s\n", definition.Meaning)
	if definition.Location != "" {
		fmt.Fprintf(&builder, "  location: %s\n", definition.Location)
	}
	for _, action := range definition.Actions {
		label := strings.TrimSpace(action.Label)
		if label == "" {
			label = string(action.Kind)
		}
		fmt.Fprintf(&builder, "  action:   %s: %s\n", label, action.Value)
	}
	return builder.String()
}
