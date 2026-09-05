package change

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/gitproject"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
)

const ChangeReconciliationSchemaVersion = 1

type FileChange struct {
	Status       string `json:"status"`
	Path         string `json:"path"`
	PreviousPath string `json:"previousPath,omitempty"`
	Class        string `json:"class"`
	Owner        string `json:"owner,omitempty"`
}

type ChangeViolation struct {
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Detail string `json:"detail"`
}

type Reconciliation struct {
	SchemaVersion int               `json:"schemaVersion"`
	BaseSHA       string            `json:"baseSha"`
	OperationID   string            `json:"operationId"`
	Changes       []FileChange      `json:"changes"`
	Violations    []ChangeViolation `json:"violations"`
}

func checkCommand() cli.Command {
	return cli.Command{
		Name:  "check",
		Usage: "quickly reconcile the actual Git delta with the active change contract",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "contract", Value: DefaultChangeContractPath, Usage: "change contract path"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			descriptor, err := projectflow.DescribeProject(projectflow.Options{Root: c.String("root")})
			if err != nil {
				return printFailure("yunka change check", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change check: resolve project: %w", err)}), 1)
			}
			contractValue, _, err := LoadChangeContract(descriptor.Root, c.String("contract"))
			if err != nil {
				return printFailure("yunka change check", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: fmt.Errorf("change check: load contract: %w", err)}), 1)
			}
			report, err := ReconcileGitDelta(descriptor.Root, contractValue)
			if err != nil {
				return printFailure("yunka change check", c.String("format"), Diagnose(&Failure{Kind: FailureEvidence, Err: err}), 1)
			}
			output, err := RenderReconciliation(report, c.String("format"))
			if err != nil {
				return err
			}
			fmt.Print(output)
			if len(report.Violations) > 0 {
				return cli.NewExitError("", 1)
			}
			return nil
		},
	}
}

func ReconcileGitDelta(root string, contractValue ChangeContract) (Reconciliation, error) {
	if contractValue.SchemaVersion != ChangeContractSchemaVersion {
		return Reconciliation{}, fmt.Errorf("change check: unsupported contract schemaVersion %d", contractValue.SchemaVersion)
	}
	changes, err := gitChanges(root, contractValue.BaseSHA)
	if err != nil {
		return Reconciliation{}, err
	}
	report := Reconciliation{
		SchemaVersion: ChangeReconciliationSchemaVersion,
		BaseSHA:       contractValue.BaseSHA,
		OperationID:   contractValue.Operation.OperationID,
	}
	for _, change := range changes {
		classified, violation, err := reconcileFile(root, contractValue, change)
		if err != nil {
			return Reconciliation{}, err
		}
		report.Changes = append(report.Changes, classified)
		if violation != nil {
			report.Violations = append(report.Violations, *violation)
		}
		if change.PreviousPath != "" && change.PreviousPath != change.Path {
			previous := FileChange{Status: "D", Path: change.PreviousPath}
			_, previousViolation, err := reconcileFile(root, contractValue, previous)
			if err != nil {
				return Reconciliation{}, err
			}
			if previousViolation != nil {
				report.Violations = append(report.Violations, *previousViolation)
			}
		}
	}
	sort.Slice(report.Changes, func(i, j int) bool {
		if report.Changes[i].Path != report.Changes[j].Path {
			return report.Changes[i].Path < report.Changes[j].Path
		}
		return report.Changes[i].Status < report.Changes[j].Status
	})
	sort.Slice(report.Violations, func(i, j int) bool {
		if report.Violations[i].Path != report.Violations[j].Path {
			return report.Violations[i].Path < report.Violations[j].Path
		}
		return report.Violations[i].Kind < report.Violations[j].Kind
	})
	if report.Changes == nil {
		report.Changes = []FileChange{}
	}
	if report.Violations == nil {
		report.Violations = []ChangeViolation{}
	}
	return report, nil
}

func reconcileFile(root string, contractValue ChangeContract, change FileChange) (FileChange, *ChangeViolation, error) {
	path := cleanProjectPath(change.Path)
	change.Path = path
	if path == "" {
		change.Class = "outside"
		return change, &ChangeViolation{Kind: "scope", Path: change.Path, Detail: "changed path is outside the canonical project-relative namespace"}, nil
	}

	// Exact generated paths come from canonical generated-effect evidence and are
	// safe to accept as expected derived output. Generated *scopes* are broader
	// impact hints (for example the application codegen root), so they must not
	// become a blanket mutation allowlist: AX2 must independently prove the
	// concrete changed path is generator-owned.
	if contains(contractValue.GeneratedPaths, path) {
		change.Class = "generated"
		change.Owner = "yunka-generator"
		return change, nil, nil
	}
	if withinAnyScope(path, contractValue.GeneratedScopes) {
		report, err := ownership.Build(root, []string{path})
		if err == nil && len(report.Decisions) == 1 && report.Decisions[0].Mutation == ownership.MutationGeneratedOnly {
			change.Class = "generated"
			change.Owner = report.Decisions[0].Owner
			return change, nil, nil
		}
	}

	if !isEditableAllowed(contractValue, path) {
		change.Class = "outside"
		return change, &ChangeViolation{Kind: "scope", Path: path, Detail: "actual Git delta is outside the declared change contract"}, nil
	}

	// Pressure-proven AX7 placement rule: a broad Application scope proves only
	// where existing handwritten implementation may live. It does not prove that
	// an Agent may introduce a new handwritten file there. Added, renamed, or
	// copied destinations therefore require an exact EditablePaths declaration
	// captured by `yunka change begin --path ...` before the mutation.
	if requiresExactPlacement(change) && !contains(contractValue.EditablePaths, path) {
		change.Class = "placement-blocked"
		return change, &ChangeViolation{
			Kind:   "placement",
			Path:   path,
			Detail: "new handwritten path inside a candidate editable scope requires an explicit exact path in the change contract; declare it with `yunka change begin --path <path>` before creating, renaming, or copying the file",
		}, nil
	}

	report, err := ownership.Build(root, []string{path})
	if err != nil {
		return FileChange{}, nil, fmt.Errorf("change check: classify %s: %w", path, err)
	}
	if len(report.Decisions) != 1 {
		return FileChange{}, nil, fmt.Errorf("change check: ownership returned %d decisions for %s", len(report.Decisions), path)
	}
	decision := report.Decisions[0]
	change.Owner = decision.Owner
	if !decision.SafeAutoEdit {
		change.Class = "ownership-blocked"
		return change, &ChangeViolation{Kind: "ownership", Path: path, Detail: decision.Reason}, nil
	}
	change.Class = "editable"
	return change, nil, nil
}

func requiresExactPlacement(change FileChange) bool {
	status := strings.ToUpper(strings.TrimSpace(change.Status))
	return strings.HasPrefix(status, "A") || strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C")
}

func isEditableAllowed(contractValue ChangeContract, path string) bool {
	return contains(contractValue.EditablePaths, path) || withinAnyScope(path, contractValue.EditableScopes)
}

func isGeneratedAllowed(contractValue ChangeContract, path string) bool {
	return contains(contractValue.GeneratedPaths, path) || withinAnyScope(path, contractValue.GeneratedScopes)
}

func gitChanges(root, baseSHA string) ([]FileChange, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	if baseSHA == "" {
		return nil, fmt.Errorf("change check: base SHA is required")
	}
	paths, err := gitproject.Resolve(root)
	if err != nil {
		return nil, fmt.Errorf("change check: resolve Git project paths: %w", err)
	}

	diffArgs := []string{"diff", "--name-status", "-z", "--find-renames", baseSHA, "--"}
	if paths.ProjectPrefix != "." {
		diffArgs = append(diffArgs, paths.ProjectPrefix)
	}
	tracked, err := runGitRawBytes(paths.RepositoryRoot, diffArgs...)
	if err != nil {
		return nil, fmt.Errorf("change check: Git diff: %w", err)
	}
	repositoryChanges, err := parseNameStatusZ(tracked)
	if err != nil {
		return nil, err
	}
	changes, err := projectFileChanges(paths, repositoryChanges)
	if err != nil {
		return nil, err
	}

	untrackedArgs := []string{"ls-files", "--others", "--exclude-standard", "-z", "--"}
	if paths.ProjectPrefix != "." {
		untrackedArgs = append(untrackedArgs, paths.ProjectPrefix)
	}
	untracked, err := runGitRawBytes(paths.RepositoryRoot, untrackedArgs...)
	if err != nil {
		return nil, fmt.Errorf("change check: Git untracked files: %w", err)
	}
	for _, repositoryPath := range splitNUL(untracked) {
		projectPath, inside, err := paths.ToProject(repositoryPath)
		if err != nil {
			return nil, fmt.Errorf("change check: translate untracked path %s: %w", repositoryPath, err)
		}
		if !inside {
			continue
		}
		projectPath = cleanProjectPath(projectPath)
		if projectPath == "" {
			continue
		}
		changes = append(changes, FileChange{Status: "A", Path: projectPath})
	}
	return dedupeFileChanges(changes), nil
}

func projectFileChanges(paths gitproject.Paths, repositoryChanges []FileChange) ([]FileChange, error) {
	result := make([]FileChange, 0, len(repositoryChanges))
	for _, change := range repositoryChanges {
		projectPath, pathInside, err := paths.ToProject(change.Path)
		if err != nil {
			return nil, fmt.Errorf("change check: translate Git path %s: %w", change.Path, err)
		}
		if change.PreviousPath == "" {
			if !pathInside {
				continue
			}
			change.Path = projectPath
			result = append(result, change)
			continue
		}

		previousProjectPath, previousInside, err := paths.ToProject(change.PreviousPath)
		if err != nil {
			return nil, fmt.Errorf("change check: translate previous Git path %s: %w", change.PreviousPath, err)
		}
		switch {
		case pathInside && previousInside:
			change.Path = projectPath
			change.PreviousPath = previousProjectPath
			result = append(result, change)
		case pathInside && !previousInside:
			// A rename/copy entering the project is a new project destination. Keep
			// the Git status so AX7 exact-placement rules still fail closed.
			change.Path = projectPath
			change.PreviousPath = ""
			result = append(result, change)
		case !pathInside && previousInside && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(change.Status)), "R"):
			// A rename leaving the project is a deletion from the project path
			// domain. Copies leaving the project do not change project contents.
			result = append(result, FileChange{Status: "D", Path: previousProjectPath})
		}
	}
	return result, nil
}

func runGitBytes(root string, args ...string) ([]byte, error) {
	commandRoot := root
	commandArgs := append([]string(nil), args...)
	if len(commandArgs) >= 2 && commandArgs[0] == "show" {
		if ref, path, ok := splitGitObjectPath(commandArgs[1]); ok {
			paths, err := gitproject.Resolve(root)
			if err != nil {
				return nil, err
			}
			repositoryPath, err := paths.ToRepository(path)
			if err != nil {
				return nil, err
			}
			commandRoot = paths.RepositoryRoot
			commandArgs[1] = ref + ":" + repositoryPath
		}
	}
	return runGitRawBytes(commandRoot, commandArgs...)
}

func splitGitObjectPath(value string) (string, string, bool) {
	index := strings.IndexByte(value, ':')
	if index <= 0 || index == len(value)-1 {
		return "", "", false
	}
	ref := strings.TrimSpace(value[:index])
	path := cleanProjectPath(value[index+1:])
	if ref == "" || path == "" {
		return "", "", false
	}
	return ref, path, true
}

func runGitRawBytes(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.Bytes(), nil
}

func parseNameStatusZ(data []byte) ([]FileChange, error) {
	tokens := splitNUL(data)
	var result []FileChange
	for index := 0; index < len(tokens); {
		status := strings.TrimSpace(tokens[index])
		index++
		if status == "" {
			continue
		}
		if index >= len(tokens) {
			return nil, fmt.Errorf("change check: malformed Git name-status output after %s", status)
		}
		path := cleanProjectPath(tokens[index])
		index++
		change := FileChange{Status: status, Path: path}
		if strings.HasPrefix(status, "R") || strings.HasPrefix(status, "C") {
			if index >= len(tokens) {
				return nil, fmt.Errorf("change check: malformed Git rename/copy output after %s", status)
			}
			change.PreviousPath = path
			change.Path = cleanProjectPath(tokens[index])
			index++
		}
		result = append(result, change)
	}
	return result, nil
}

func splitNUL(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		result = append(result, string(part))
	}
	return result
}

func dedupeFileChanges(values []FileChange) []FileChange {
	seen := map[string]FileChange{}
	for _, value := range values {
		key := value.Status + "\x00" + value.PreviousPath + "\x00" + value.Path
		seen[key] = value
	}
	result := make([]FileChange, 0, len(seen))
	for _, value := range seen {
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		return result[i].Status < result[j].Status
	})
	return result
}

func RenderReconciliation(report Reconciliation, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(data, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change check: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "operation %s\n", report.OperationID)
	fmt.Fprintf(&builder, "base      %s\n", report.BaseSHA)
	fmt.Fprintf(&builder, "changes   %d\n", len(report.Changes))
	for _, item := range report.Changes {
		fmt.Fprintf(&builder, "  %-4s %-18s %s\n", item.Status, item.Class, item.Path)
	}
	fmt.Fprintf(&builder, "violations %d\n", len(report.Violations))
	for _, item := range report.Violations {
		fmt.Fprintf(&builder, "  %s %s — %s\n", item.Kind, item.Path, item.Detail)
	}
	return builder.String(), nil
}
