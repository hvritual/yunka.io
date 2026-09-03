package change

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/projectflow"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
)

const (
	ChangeAttestationSchemaVersion = 1
	DefaultChangeAttestationPath   = ".yunka/change-attestation.json"
)

type GateResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ChangeAttestation struct {
	SchemaVersion int                    `json:"schemaVersion"`
	BaseSHA       string                 `json:"baseSha"`
	HeadSHA       string                 `json:"headSha"`
	OperationID   string                 `json:"operationId"`
	Reconciliation Reconciliation        `json:"reconciliation"`
	Semantic      SemanticReport         `json:"semantic"`
	Gates         []GateResult           `json:"gates"`
	Diagnostics   []diagnostic.Diagnostic `json:"diagnostics"`
	Conformant    bool                   `json:"conformant"`
}

func verifyCommand() cli.Command {
	return cli.Command{
		Name:  "verify",
		Usage: "run final change-contract reconciliation and canonical Yunka verification",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "contract", Value: DefaultChangeContractPath, Usage: "change contract path"},
			cli.StringFlag{Name: "output", Value: DefaultChangeAttestationPath, Usage: "attestation output path"},
			cli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary; defaults to PATH"},
			cli.StringSliceFlag{Name: "proto-path", Usage: "additional protoc import path; may be repeated"},
			cli.BoolFlag{Name: "skip-tests", Usage: "skip the final `go test ./...` gate; resulting attestation is still structural/semantic conformance only"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			attestation, root, err := VerifyChange(context.Background(), VerifyOptions{
				Root:       c.String("root"),
				Contract:   c.String("contract"),
				Protoc:     c.String("protoc"),
				ProtoPaths: c.StringSlice("proto-path"),
				SkipTests:  c.Bool("skip-tests"),
			})
			if err != nil {
				return printFailure("yunka change verify", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: err}), 1)
			}
			path, err := WriteChangeAttestation(root, c.String("output"), attestation)
			if err != nil {
				return printFailure("yunka change verify", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change verify: persist attestation: %w", err)}), 1)
			}
			output, err := RenderChangeAttestation(attestation, path, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			if !attestation.Conformant {
				return cli.NewExitError("", 1)
			}
			return nil
		},
	}
}

type VerifyOptions struct {
	Root       string
	Contract   string
	Protoc     string
	ProtoPaths []string
	SkipTests  bool
}

func VerifyChange(ctx context.Context, options VerifyOptions) (ChangeAttestation, string, error) {
	descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: options.Root, Protoc: options.Protoc, ProtoPaths: options.ProtoPaths})
	if err != nil {
		return ChangeAttestation{}, "", fmt.Errorf("change verify: resolve project: %w", err)
	}
	contractValue, _, err := LoadChangeContract(descriptor.Root, options.Contract)
	if err != nil {
		return ChangeAttestation{}, "", fmt.Errorf("change verify: load contract: %w", err)
	}
	headSHA, err := resolveGitBase(descriptor.Root, "HEAD")
	if err != nil {
		return ChangeAttestation{}, "", err
	}
	attestation := ChangeAttestation{
		SchemaVersion: ChangeAttestationSchemaVersion,
		BaseSHA:       contractValue.BaseSHA,
		HeadSHA:       headSHA,
		OperationID:   contractValue.Operation.OperationID,
		Semantic:      SemanticReport{SchemaVersion: SemanticReportSchemaVersion, OperationID: contractValue.Operation.OperationID, Deltas: []SemanticDelta{}, Violations: []SemanticDelta{}},
	}

	reconciliation, err := ReconcileGitDelta(descriptor.Root, contractValue)
	if err != nil {
		return ChangeAttestation{}, "", err
	}
	attestation.Reconciliation = reconciliation
	if len(reconciliation.Violations) == 0 {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "git-delta", Status: "pass"})
	} else {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "git-delta", Status: "fail", Detail: fmt.Sprintf("%d scope/ownership violation(s)", len(reconciliation.Violations))})
		for _, violation := range reconciliation.Violations {
			attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic("change-reconciliation", violation.Path, violation.Kind+": "+violation.Detail))
		}
	}

	workflowReport, workflowErr := projectflow.CheckWithFastFeedback(ctx, projectflow.Options{
		Root:       descriptor.Root,
		Protoc:     options.Protoc,
		ProtoPaths: options.ProtoPaths,
	}, true)
	_ = workflowReport
	if workflowErr != nil {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "yunka-check", Status: "fail", Detail: strings.TrimSpace(workflowErr.Error())})
		attestation.Diagnostics = append(attestation.Diagnostics, projectflow.Diagnose(workflowErr))
	} else {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "yunka-check", Status: "pass"})
		semantic, semanticErr := ReconcileSemanticDelta(descriptor.Root, contractValue)
		if semanticErr != nil {
			attestation.Gates = append(attestation.Gates, GateResult{Name: "semantic-delta", Status: "fail", Detail: semanticErr.Error()})
			attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic("semantic-delta", "", semanticErr.Error()))
		} else {
			attestation.Semantic = semantic
			if len(semantic.Violations) == 0 {
				attestation.Gates = append(attestation.Gates, GateResult{Name: "semantic-delta", Status: "pass"})
			} else {
				attestation.Gates = append(attestation.Gates, GateResult{Name: "semantic-delta", Status: "fail", Detail: fmt.Sprintf("%d undeclared semantic delta(s)", len(semantic.Violations))})
				for _, delta := range semantic.Violations {
					attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic("semantic-delta", delta.Subject, fmt.Sprintf("%s %s changed outside the allowed semantic envelope", delta.Category, delta.Field)))
				}
			}
		}
	}

	if descriptor.GoModule == "" {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "go-test", Status: "skipped", Detail: "project has no Go module identity"})
	} else if options.SkipTests {
		attestation.Gates = append(attestation.Gates, GateResult{Name: "go-test", Status: "skipped", Detail: "explicit --skip-tests"})
	} else {
		if err := runGoTests(ctx, descriptor.Root); err != nil {
			attestation.Gates = append(attestation.Gates, GateResult{Name: "go-test", Status: "fail", Detail: err.Error()})
			attestation.Diagnostics = append(attestation.Diagnostics, changeDiagnostic("go-test", "", err.Error()))
		} else {
			attestation.Gates = append(attestation.Gates, GateResult{Name: "go-test", Status: "pass"})
		}
	}

	attestation.Conformant = len(attestation.Diagnostics) == 0
	if attestation.Gates == nil {
		attestation.Gates = []GateResult{}
	}
	if attestation.Diagnostics == nil {
		attestation.Diagnostics = []diagnostic.Diagnostic{}
	}
	return attestation, descriptor.Root, nil
}

func runGoTests(ctx context.Context, root string) error {
	command := exec.CommandContext(ctx, "go", "test", "./...")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("go test ./...: %s", detail)
	}
	return nil
}

func changeDiagnostic(stage, path, detail string) diagnostic.Diagnostic {
	item := diagnostic.MustDefinition(diagnostic.CodeChangeEvidence).Diagnostic(diagnostic.SeverityError)
	item.Stage = stage
	item.Detail = strings.TrimSpace(detail)
	if strings.TrimSpace(path) != "" {
		item.Location = &diagnostic.Location{Path: filepath.ToSlash(path)}
	}
	return item
}

func WriteChangeAttestation(root, output string, value ChangeAttestation) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		output = DefaultChangeAttestationPath
	}
	path := output
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative), nil
	}
	return filepath.ToSlash(path), nil
}

func RenderChangeAttestation(value ChangeAttestation, path, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		payload := struct {
			Path        string            `json:"path"`
			Attestation ChangeAttestation `json:"attestation"`
		}{Path: path, Attestation: value}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(data, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change verify: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "change attestation %s\n", path)
	fmt.Fprintf(&builder, "operation %s\n", value.OperationID)
	fmt.Fprintf(&builder, "base      %s\n", value.BaseSHA)
	fmt.Fprintf(&builder, "head      %s\n", value.HeadSHA)
	for _, gate := range value.Gates {
		fmt.Fprintf(&builder, "%-16s %s", gate.Name, gate.Status)
		if gate.Detail != "" {
			fmt.Fprintf(&builder, " — %s", gate.Detail)
		}
		builder.WriteByte('\n')
	}
	fmt.Fprintf(&builder, "conformant %t\n", value.Conformant)
	return builder.String(), nil
}
