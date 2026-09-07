package add

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

var ErrBoundaryBlocked = errors.New("OPERATION_BOUNDARY_BLOCKED")

func operationContext(options OperationOptions) context.Context {
	if options.Context != nil {
		return options.Context
	}
	return context.Background()
}
func operationBase(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--verify", "HEAD^{commit}")
	command.Env = append(os.Environ(), "GIT_NO_REPLACE_OBJECTS=1")
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("add operation: an existing Git HEAD is required for boundary proof: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}
func bindOperationBoundary(options OperationOptions, source string, original, replacement []byte, report *Report) error {
	ctx := operationContext(options)
	base, err := operationBase(ctx, options.Root)
	if err != nil {
		return err
	}
	edit, err := projectflow.PreviewContractEdit(ctx, projectflow.Options{Root: options.Root, ProtoPaths: options.ProtoPaths}, source, original, replacement)
	if err != nil {
		return err
	}
	currentBase, err := operationBase(ctx, options.Root)
	if err != nil {
		return err
	}
	if currentBase != base {
		return fmt.Errorf("%w: HEAD changed during preview", boundarycore.ErrStaleBoundaryProof)
	}
	decision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{BaseSHA: base, Application: options.ApplicationKey, OperationID: options.OperationID}, edit.Before, edit.After)
	if err != nil {
		return err
	}
	report.SchemaVersion = OperationReportVersion
	report.BaseSHA = base
	report.InputsDigest = edit.InputsDigest
	report.ProtoPaths = append([]string(nil), options.ProtoPaths...)
	report.BoundaryDecision = &decision
	return nil
}
func boundaryAllows(report Report) bool {
	return report.BoundaryDecision != nil && report.BoundaryDecision.Outcome == boundarycore.ReuseExistingApplication
}
func blockedOperationReport(report *Report) {
	report.Mutations = []Mutation{}
	report.Effects = []Effect{}
	report.NextActions = []NextAction{{Command: "yunka boundary inspect " + shellQuote(report.BoundaryDecision.TargetApplication), Purpose: "inspect evidence; resolve missing or conflicting architectural intent before creating a new plan"}}
	report.Notes = append(report.Notes, "Blocked boundary: no source or implementation mutation is permitted. Empty/new boundaries and unproven client-contract changes require a separately scoped architecture decision; there is no --force bypass.")
}
func verifyOperationInputs(options OperationOptions, report Report) error {
	ctx := operationContext(options)
	if err := ctx.Err(); err != nil {
		return err
	}
	base, err := operationBase(ctx, options.Root)
	if err != nil {
		return err
	}
	digest, err := projectflow.ContractInputsDigest(ctx, projectflow.Options{Root: options.Root, ProtoPaths: options.ProtoPaths})
	if err != nil {
		return err
	}
	if base != report.BaseSHA || digest != report.InputsDigest {
		return fmt.Errorf("%w: HEAD or compiler inputs changed before write", boundarycore.ErrStaleBoundaryProof)
	}
	return nil
}

// The lock serializes Yunka Operation writers in this worktree. It is not a
// filesystem sandbox: arbitrary external editors must not race an apply.
func lockOperationWriter(ctx context.Context, root string) (func(), error) {
	cmd := exec.CommandContext(ctx, "git", "-C", root, "rev-parse", "--path-format=absolute", "--git-path", "yunka/operation-authoring.lock")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("add operation: resolve Git-private lock: %w", err)
	}
	path := strings.TrimSpace(string(output))
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("add operation: invalid Git-private lock path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("add operation: writer lock unavailable; inspect an interrupted writer before removing its lock: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}

func finishOperation(c *cli.Context, command string, report Report, err error) error {
	if report.BoundaryDecision == nil {
		return finish(c, command, report, err)
	}
	if err != nil && !errors.Is(err, ErrBoundaryBlocked) {
		return finish(c, command, Report{}, err)
	}
	output, renderErr := Render(report, c.String("format"))
	if renderErr != nil {
		return renderErr
	}
	if _, writeErr := fmt.Fprint(c.App.Writer, output); writeErr != nil {
		return writeErr
	}
	if !boundaryAllows(report) {
		return cli.NewExitError("", 1)
	}
	return nil
}
