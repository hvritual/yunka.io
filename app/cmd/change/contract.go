package change

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
)

const (
	ChangeContractSchemaVersion = 1
	DefaultChangeContractPath    = ".yunka/change-contract.json"

	SemanticPermission     = "permission"
	SemanticTenant         = "tenant"
	SemanticAuthentication = "authentication"
	SemanticTransaction    = "transaction"
	SemanticIdempotency    = "idempotency"
	SemanticComposition    = "composition"
	SemanticDependencies   = "dependencies"
	SemanticCapabilities   = "capabilities"
	SemanticTransport      = "transport"
)

var supportedSemanticCategories = map[string]struct{}{
	SemanticPermission:     {},
	SemanticTenant:         {},
	SemanticAuthentication: {},
	SemanticTransaction:    {},
	SemanticIdempotency:    {},
	SemanticComposition:    {},
	SemanticDependencies:   {},
	SemanticCapabilities:   {},
	SemanticTransport:      {},
}

type ChangeOperation struct {
	NodeID      string `json:"nodeId"`
	OperationID string `json:"operationId"`
	Domain      string `json:"domain"`
	Application string `json:"application"`
}

type ChangeContract struct {
	SchemaVersion    int             `json:"schemaVersion"`
	BaseSHA          string          `json:"baseSha"`
	Intent           string          `json:"intent"`
	Operation        ChangeOperation `json:"operation"`
	AllowedSemantic  []string        `json:"allowedSemantic"`
	EditablePaths    []string        `json:"editablePaths"`
	EditableScopes   []string        `json:"editableScopes"`
	GeneratedPaths   []string        `json:"generatedPaths"`
	GeneratedScopes  []string        `json:"generatedScopes"`
}

func beginCommand() cli.Command {
	return cli.Command{
		Name:  "begin",
		Usage: "start a bounded change contract for one existing canonical operation",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
			cli.StringFlag{Name: "operation", Usage: "exact canonical operation ID or operation:<ID> graph node ID"},
			cli.StringFlag{Name: "intent", Value: IntentImplementation, Usage: "change intent: contract, implementation, or both"},
			cli.StringFlag{Name: "base", Value: "HEAD", Usage: "Git commit/ref used as the authoritative change baseline"},
			cli.StringSliceFlag{Name: "path", Usage: "explicit developer-owned mutation target; may be repeated to resolve ambiguous contract-source evidence"},
			cli.StringSliceFlag{Name: "allow-semantic", Usage: "explicitly allow one canonical semantic category; may be repeated"},
			cli.IntFlag{Name: "depth", Value: 3, Usage: "maximum static graph impact depth used by the underlying change plan"},
			cli.StringFlag{Name: "output", Value: DefaultChangeContractPath, Usage: "change contract path, relative to project root unless absolute"},
			cli.StringFlag{Name: "format", Value: FormatText, Usage: "output format: text, json, or agent-json"},
		},
		Action: func(c *cli.Context) error {
			format := strings.ToLower(strings.TrimSpace(c.String("format")))
			if format == "" {
				format = FormatText
			}
			if format != FormatText && format != FormatJSON && format != FormatAgentJSON {
				return fmt.Errorf("change begin: unsupported format %q", format)
			}

			contractValue, projectRoot, err := BuildChangeContract(
				c.String("root"),
				c.String("operation"),
				c.String("intent"),
				c.String("base"),
				c.StringSlice("path"),
				c.StringSlice("allow-semantic"),
				c.Int("depth"),
			)
			if err != nil {
				return printFailure("yunka change begin", format, Diagnose(err), 1)
			}
			path, err := WriteChangeContract(projectRoot, c.String("output"), contractValue)
			if err != nil {
				failure := &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change begin: persist contract: %w", err)}
				return printFailure("yunka change begin", format, Diagnose(failure), 1)
			}
			output, err := RenderChangeContract(contractValue, path, format)
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func BuildChangeContract(root, operation, intent, base string, explicitPaths, allowedSemantic []string, depth int) (ChangeContract, string, error) {
	if err := ensureCleanWorktree(root); err != nil {
		return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: err}
	}
	plan, err := Build(root, operation, intent, depth)
	if err != nil {
		return ChangeContract{}, "", err
	}
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: root})
	if err != nil {
		return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change begin: project ownership facts: %w", err)}
	}
	baseSHA, err := resolveGitBase(inputs.Project.Root, base)
	if err != nil {
		return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: err}
	}
	semantic, err := normalizeAllowedSemantic(allowedSemantic)
	if err != nil {
		return ChangeContract{}, "", &Failure{Kind: FailureIntent, Err: err}
	}

	contractValue := ChangeContract{
		SchemaVersion:   ChangeContractSchemaVersion,
		BaseSHA:         baseSHA,
		Intent:          plan.Intent,
		Operation: ChangeOperation{
			NodeID:      plan.Operation.ID,
			OperationID: strings.TrimSpace(plan.Operation.Attributes["operationId"]),
			Domain:      strings.TrimSpace(plan.Operation.Domain),
			Application: strings.TrimSpace(plan.Operation.Application),
		},
		AllowedSemantic: semantic,
	}
	if contractValue.Operation.OperationID == "" {
		return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change begin: canonical operation %s has no operationId evidence", plan.Operation.ID)}
	}

	for _, target := range plan.EditableTargets {
		contractValue.EditablePaths = append(contractValue.EditablePaths, target.Path)
	}
	for _, target := range plan.UnresolvedTargets {
		switch target.Kind {
		case "implementation":
			if target.Scope != "" {
				contractValue.EditableScopes = append(contractValue.EditableScopes, target.Scope)
			}
		case "contract-source":
			selected, selectErr := resolveExplicitContractTarget(inputs.Project.Root, target, explicitPaths)
			if selectErr != nil {
				return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: selectErr}
			}
			contractValue.EditablePaths = append(contractValue.EditablePaths, selected)
		}
	}

	for _, path := range explicitPaths {
		path = cleanProjectPath(path)
		if path == "" || contains(contractValue.EditablePaths, path) {
			continue
		}
		if !withinAnyScope(path, contractValue.EditableScopes) {
			return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change begin: explicit path %s is outside the canonical editable scopes for %s", path, contractValue.Operation.OperationID)}
		}
		if err := requireEditableOwnership(inputs.Project.Root, path); err != nil {
			return ChangeContract{}, "", &Failure{Kind: FailureEvidence, Err: err}
		}
		contractValue.EditablePaths = append(contractValue.EditablePaths, path)
	}

	for _, effect := range plan.GeneratedEffects {
		if effect.Path != "" {
			contractValue.GeneratedPaths = append(contractValue.GeneratedPaths, effect.Path)
		}
		if effect.Scope != "" {
			contractValue.GeneratedScopes = append(contractValue.GeneratedScopes, effect.Scope)
		}
	}
	normalizeChangeContract(&contractValue)
	return contractValue, inputs.Project.Root, nil
}

func resolveExplicitContractTarget(root string, target UnresolvedTarget, explicitPaths []string) (string, error) {
	if len(explicitPaths) == 0 {
		return "", fmt.Errorf("change begin: contract source is ambiguous; select exactly one candidate with --path (%s)", strings.Join(target.Candidates, ", "))
	}
	candidateSet := make(map[string]struct{}, len(target.Candidates))
	for _, candidate := range target.Candidates {
		candidateSet[cleanProjectPath(candidate)] = struct{}{}
	}
	var selected []string
	for _, path := range explicitPaths {
		path = cleanProjectPath(path)
		if _, ok := candidateSet[path]; ok {
			selected = append(selected, path)
		}
	}
	selected = uniqueSorted(selected)
	if len(selected) != 1 {
		return "", fmt.Errorf("change begin: ambiguous contract source requires exactly one candidate --path; selected=%d candidates=%s", len(selected), strings.Join(target.Candidates, ", "))
	}
	if err := requireEditableOwnership(root, selected[0]); err != nil {
		return "", err
	}
	return selected[0], nil
}

func requireEditableOwnership(root, path string) error {
	report, err := ownership.Build(root, []string{path})
	if err != nil {
		return fmt.Errorf("change begin: classify %s: %w", path, err)
	}
	if len(report.Decisions) != 1 || !report.Decisions[0].SafeAutoEdit {
		return fmt.Errorf("change begin: AX2 ownership does not prove %s safe for automatic editing", path)
	}
	return nil
}

func normalizeAllowedSemantic(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := supportedSemanticCategories[value]; !ok {
			keys := make([]string, 0, len(supportedSemanticCategories))
			for key := range supportedSemanticCategories {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			return nil, fmt.Errorf("change begin: semantic category %q is unsupported; use %s", value, strings.Join(keys, ", "))
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func normalizeChangeContract(value *ChangeContract) {
	if value == nil {
		return
	}
	value.EditablePaths = uniqueSorted(value.EditablePaths)
	value.EditableScopes = uniqueSorted(value.EditableScopes)
	value.GeneratedPaths = uniqueSorted(value.GeneratedPaths)
	value.GeneratedScopes = uniqueSorted(value.GeneratedScopes)
	value.AllowedSemantic = append([]string(nil), value.AllowedSemantic...)
	sort.Strings(value.AllowedSemantic)
	if value.EditablePaths == nil {
		value.EditablePaths = []string{}
	}
	if value.EditableScopes == nil {
		value.EditableScopes = []string{}
	}
	if value.GeneratedPaths == nil {
		value.GeneratedPaths = []string{}
	}
	if value.GeneratedScopes == nil {
		value.GeneratedScopes = []string{}
	}
	if value.AllowedSemantic == nil {
		value.AllowedSemantic = []string{}
	}
}

func WriteChangeContract(root, output string, value ChangeContract) (string, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		output = DefaultChangeContractPath
	}
	path := output
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	temporary, err := os.CreateTemp(filepath.Dir(path), ".change-contract-*")
	if err != nil {
		return "", err
	}
	tmp := temporary.Name()
	defer os.Remove(tmp)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, path)
	if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(relative), nil
	}
	return filepath.ToSlash(path), nil
}

func LoadChangeContract(root, input string) (ChangeContract, string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		input = DefaultChangeContractPath
	}
	path := input
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, filepath.FromSlash(path))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ChangeContract{}, "", err
	}
	var value ChangeContract
	if err := json.Unmarshal(data, &value); err != nil {
		return ChangeContract{}, "", fmt.Errorf("decode change contract: %w", err)
	}
	if value.SchemaVersion != ChangeContractSchemaVersion {
		return ChangeContract{}, "", fmt.Errorf("unsupported change contract schemaVersion %d", value.SchemaVersion)
	}
	normalizeChangeContract(&value)
	return value, filepath.ToSlash(path), nil
}

func RenderChangeContract(value ChangeContract, path, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == FormatJSON || format == FormatAgentJSON {
		payload := struct {
			Path     string         `json:"path"`
			Contract ChangeContract `json:"contract"`
		}{Path: path, Contract: value}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(data, '\n')), nil
	}
	if format != "" && format != FormatText {
		return "", fmt.Errorf("change begin: unsupported format %q", format)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "change contract %s\n", path)
	fmt.Fprintf(&builder, "base            %s\n", value.BaseSHA)
	fmt.Fprintf(&builder, "operation       %s\n", value.Operation.OperationID)
	fmt.Fprintf(&builder, "intent          %s\n", value.Intent)
	fmt.Fprintf(&builder, "editable paths  %d\n", len(value.EditablePaths))
	fmt.Fprintf(&builder, "editable scopes %d\n", len(value.EditableScopes))
	fmt.Fprintf(&builder, "allowed semantic %s\n", strings.Join(value.AllowedSemantic, ","))
	builder.WriteString("next            yunka change check\n")
	return builder.String(), nil
}

func cleanProjectPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(filepath.FromSlash(clean)) {
		return ""
	}
	return clean
}

func withinAnyScope(path string, scopes []string) bool {
	path = cleanProjectPath(path)
	for _, scope := range scopes {
		scope = cleanProjectPath(scope)
		if scope == "" {
			continue
		}
		if path == scope || strings.HasPrefix(path, strings.TrimSuffix(scope, "/")+"/") {
			return true
		}
	}
	return false
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
