package change

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/urfave/cli"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
	applicationgraph "github.com/hvritual/yunka.io/pkg/applicationgraph"
	"github.com/hvritual/yunka.io/pkg/contract"
	"github.com/hvritual/yunka.io/pkg/diagnostic"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

const (
	AppName       = "change"
	SchemaVersion = 1

	IntentContract       = "contract"
	IntentImplementation = "implementation"
	IntentBoth           = "both"

	FormatText      = "text"
	FormatJSON      = "json"
	FormatAgentJSON = "agent-json"
)

type Operation struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Domain      string            `json:"domain,omitempty"`
	Application string            `json:"application,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

type EditableTarget struct {
	Path   string `json:"path"`
	Owner  string `json:"owner"`
	Reason string `json:"reason"`
}

type UnresolvedTarget struct {
	Kind       string   `json:"kind"`
	Scope      string   `json:"scope,omitempty"`
	Candidates []string `json:"candidates"`
	Reason     string   `json:"reason"`
}

type GeneratedEffect struct {
	Stage       string `json:"stage"`
	Path        string `json:"path,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Conditional bool   `json:"conditional"`
	Reason      string `json:"reason"`
}

type Gate struct {
	Command string `json:"command"`
	Purpose string `json:"purpose"`
}

type Risk struct {
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Evidence string `json:"evidence"`
}

type Plan struct {
	SchemaVersion     int                           `json:"schemaVersion"`
	Intent            string                        `json:"intent"`
	Operation         Operation                     `json:"operation"`
	EditableTargets   []EditableTarget              `json:"editableTargets"`
	UnresolvedTargets []UnresolvedTarget            `json:"unresolvedTargets"`
	GeneratedEffects  []GeneratedEffect             `json:"generatedEffects"`
	Affected          applicationgraph.ImpactReport `json:"affected"`
	Gates             []Gate                        `json:"gates"`
	Risks             []Risk                        `json:"risks"`
}

type FailureKind string

const (
	FailureOperation FailureKind = "operation"
	FailureIntent    FailureKind = "intent"
	FailureEvidence  FailureKind = "evidence"
)

type Failure struct {
	Kind     FailureKind
	Location string
	Err      error
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
		Usage:       "plan, constrain, and verify evidence-backed changes for existing canonical operations",
		Subcommands: []cli.Command{planCommand(), beginCommand(), checkCommand(), verifyCommand()},
	}
}

func planCommand() cli.Command {
	return cli.Command{
		Name:  "plan",
		Usage: "derive impact, mutation targets, generated effects, and verification gates without changing the project",
		Flags: []cli.Flag{
			cli.StringFlag{Name: "root", Value: ".", Usage: "project root"},
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

			plan, err := Build(c.String("root"), c.String("operation"), c.String("intent"), c.Int("depth"))
			if err != nil {
				return printFailure("yunka change plan", format, Diagnose(err), 1)
			}
			output, err := Render(plan, format)
			if err != nil {
				return err
			}
			fmt.Print(output)
			return nil
		},
	}
}

func Build(root, operation, intent string, depth int) (Plan, error) {
	intent = strings.ToLower(strings.TrimSpace(intent))
	if intent != IntentContract && intent != IntentImplementation && intent != IntentBoth {
		return Plan{}, &Failure{Kind: FailureIntent, Err: fmt.Errorf("change plan: intent %q is unsupported; use contract, implementation, or both", intent)}
	}
	if strings.TrimSpace(operation) == "" {
		return Plan{}, &Failure{Kind: FailureOperation, Err: errors.New("change plan: --operation is required")}
	}

	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: root})
	if err != nil {
		return Plan{}, &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change plan: project ownership facts: %w", err)}
	}
	graph, err := loadCanonicalGraph(inputs.Project)
	if err != nil {
		return Plan{}, err
	}
	return buildFromFacts(inputs, graph, operation, intent, depth)
}

func buildFromFacts(inputs projectflow.OwnershipInputs, graph applicationgraph.Graph, operation, intent string, depth int) (Plan, error) {
	target, err := resolveOperation(graph, operation)
	if err != nil {
		return Plan{}, err
	}
	impact, err := graph.Impact(target.ID, depth)
	if err != nil {
		return Plan{}, &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change plan: impact for %s: %w", target.ID, err)}
	}

	plan := Plan{
		SchemaVersion: SchemaVersion,
		Intent:        intent,
		Operation: Operation{
			ID:          target.ID,
			Name:        target.Name,
			Domain:      target.Attributes["domain"],
			Application: target.Attributes["application"],
			Attributes:  cloneAttributes(target.Attributes),
		},
		Affected: impact,
	}

	if intent == IntentContract || intent == IntentBoth {
		if err := addContractTargets(&plan, inputs); err != nil {
			return Plan{}, err
		}
		addContractEffects(&plan, inputs)
	}
	if intent == IntentImplementation || intent == IntentBoth {
		addImplementationTarget(&plan, inputs)
	}
	plan.Risks = deriveRisks(target, impact)
	plan.Gates = deriveGates(plan, inputs.Project.GoModule)
	normalizePlan(&plan)
	return plan, nil
}

func loadCanonicalGraph(project projectflow.ProjectDescriptor) (applicationgraph.Graph, error) {
	generated := projectflow.ResolveDescriptorPath(project, project.ContractGenerated)
	manifestPath := filepath.Join(generated, contract.ManifestFilename)
	plansPath := filepath.Join(generated, contract.OperationPlansFilename)
	manifest, err := contract.LoadManifest(manifestPath)
	if err != nil {
		return applicationgraph.Graph{}, &Failure{Kind: FailureEvidence, Location: relativeTo(project.Root, manifestPath), Err: fmt.Errorf("change plan: load canonical contract manifest: %w", err)}
	}
	plans, err := operationplan.Load(plansPath)
	if err != nil {
		return applicationgraph.Graph{}, &Failure{Kind: FailureEvidence, Location: relativeTo(project.Root, plansPath), Err: fmt.Errorf("change plan: load canonical operation plans: %w", err)}
	}
	builder := applicationgraph.NewBuilder()
	if err := applicationgraph.AddContract(builder, manifest); err != nil {
		return applicationgraph.Graph{}, &Failure{Kind: FailureEvidence, Location: relativeTo(project.Root, manifestPath), Err: fmt.Errorf("change plan: project contract facts: %w", err)}
	}
	if err := applicationgraph.AddOperationPlans(builder, plans); err != nil {
		return applicationgraph.Graph{}, &Failure{Kind: FailureEvidence, Location: relativeTo(project.Root, plansPath), Err: fmt.Errorf("change plan: project operation facts: %w", err)}
	}
	graph, err := builder.Build()
	if err != nil {
		return applicationgraph.Graph{}, &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change plan: build static application graph: %w", err)}
	}
	return graph, nil
}

func resolveOperation(graph applicationgraph.Graph, value string) (applicationgraph.Node, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, string(applicationgraph.NodeOperation)+":") {
		node, ok := graph.Node(value)
		if !ok || node.Kind != applicationgraph.NodeOperation || strings.TrimSpace(node.Attributes["operationId"]) == "" {
			return applicationgraph.Node{}, &Failure{Kind: FailureOperation, Err: fmt.Errorf("change plan: canonical operation %q was not found", value)}
		}
		return node, nil
	}
	matches := make([]applicationgraph.Node, 0, 1)
	for _, node := range graph.Nodes {
		if node.Kind != applicationgraph.NodeOperation || strings.TrimSpace(node.Attributes["operationId"]) == "" {
			continue
		}
		if node.Name == value || node.Attributes["operationId"] == value {
			matches = append(matches, node)
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return applicationgraph.Node{}, &Failure{Kind: FailureOperation, Err: fmt.Errorf("change plan: canonical operation %q was not found; AX4 does not infer plans for undeclared operations", value)}
	default:
		ids := make([]string, 0, len(matches))
		for _, node := range matches {
			ids = append(ids, node.ID)
		}
		return applicationgraph.Node{}, &Failure{Kind: FailureOperation, Err: fmt.Errorf("change plan: operation %q is ambiguous: %s", value, strings.Join(ids, ", "))}
	}
}

func addContractTargets(plan *Plan, inputs projectflow.OwnershipInputs) error {
	candidates := uniqueSorted(inputs.ContractSourceFiles)
	if len(candidates) == 1 {
		report, err := ownership.Build(inputs.Project.Root, candidates)
		if err != nil {
			return &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change plan: classify contract target: %w", err)}
		}
		if len(report.Decisions) == 1 && report.Decisions[0].SafeAutoEdit {
			decision := report.Decisions[0]
			plan.EditableTargets = append(plan.EditableTargets, EditableTarget{Path: decision.Path, Owner: decision.Owner, Reason: "only canonical contract source; AX2 proves it safe for automatic editing"})
			return nil
		}
	}
	reason := "canonical artifacts do not identify which protobuf source declares this operation; semantic source selection remains with the agent/developer"
	if len(candidates) == 0 {
		reason = "no canonical protobuf source files were resolved; regenerate or repair the project contract source configuration"
	}
	plan.UnresolvedTargets = append(plan.UnresolvedTargets, UnresolvedTarget{Kind: "contract-source", Scope: inputs.Project.ContractSource, Candidates: candidates, Reason: reason})
	return nil
}

func addImplementationTarget(plan *Plan, inputs projectflow.OwnershipInputs) {
	domain := strings.TrimSpace(plan.Operation.Domain)
	scope := inputs.Project.GeneratedGoRoot
	if domain != "" {
		scope = filepath.ToSlash(filepath.Join(scope, domain, "application"))
	}
	plan.UnresolvedTargets = append(plan.UnresolvedTargets, UnresolvedTarget{
		Kind:       "implementation",
		Scope:      scope,
		Candidates: []string{},
		Reason:     "canonical OperationPlan identifies the application boundary but does not own a handwritten implementation filename; select an implementation file inside the application boundary and pass it through `yunka ownership check` before editing",
	})
}

func addContractEffects(plan *Plan, inputs projectflow.OwnershipInputs) {
	root := strings.TrimSuffix(inputs.Project.ContractGenerated, "/")
	for _, effect := range []GeneratedEffect{
		{Stage: "contract", Path: join(root, contract.ManifestFilename), Reason: "canonical manifest is derived from protobuf contract facts"},
		{Stage: "operation-plan", Path: join(root, contract.OperationPlansFilename), Reason: "OperationPlan is derived from typed operation declarations"},
		{Stage: "openapi", Path: join(root, contract.OpenAPIFilename), Conditional: true, Reason: "HTTP/API projection may change when the operation contract changes"},
		{Stage: "typescript", Path: join(root, contract.TypeScriptFilename), Conditional: true, Reason: "client projection may change when request/response/API facts change"},
		{Stage: "assembly", Path: join(root, contract.AssemblyPlanFilename), Conditional: true, Reason: "assembly may change when application or operation dependencies change"},
	} {
		plan.GeneratedEffects = append(plan.GeneratedEffects, effect)
	}
	for _, path := range uniqueSorted(inputs.ProtobufGoGeneratedFiles) {
		plan.GeneratedEffects = append(plan.GeneratedEffects, GeneratedEffect{Stage: "protobuf-go", Path: path, Conditional: true, Reason: "protobuf Go output is derived from canonical proto sources"})
	}
	if strings.TrimSpace(inputs.Project.GeneratedGoRoot) != "" {
		plan.GeneratedEffects = append(plan.GeneratedEffects, GeneratedEffect{Stage: "application-codegen", Scope: inputs.Project.GeneratedGoRoot, Conditional: true, Reason: "typed application adapters/ports may be regenerated from changed contract facts"})
	}
}

func deriveRisks(target applicationgraph.Node, impact applicationgraph.ImpactReport) []Risk {
	var risks []Risk
	for _, key := range []string{"transaction", "idempotency", "permissionMode", "public", "tenantRequired", "composition"} {
		if value := strings.TrimSpace(target.Attributes[key]); value != "" {
			risks = append(risks, Risk{Kind: key, Value: value, Evidence: "operation.plan"})
		}
	}
	currentApplication := strings.TrimSpace(target.Attributes["domain"]) + "/" + strings.TrimSpace(target.Attributes["application"])
	var cross []string
	for _, entry := range append(append([]applicationgraph.ImpactEntry{}, impact.Dependencies...), impact.Dependents...) {
		if entry.Node.Kind != applicationgraph.NodeApplication || entry.Node.Name == "" || entry.Node.Name == currentApplication {
			continue
		}
		cross = append(cross, entry.Node.ID)
	}
	cross = uniqueSorted(cross)
	if len(cross) > 0 {
		risks = append(risks, Risk{Kind: "cross-application-impact", Value: strings.Join(cross, ","), Evidence: "application.graph"})
	}
	sort.Slice(risks, func(i, j int) bool {
		if risks[i].Kind != risks[j].Kind {
			return risks[i].Kind < risks[j].Kind
		}
		return risks[i].Value < risks[j].Value
	})
	return risks
}

func deriveGates(plan Plan, goModule string) []Gate {
	var gates []Gate
	if len(plan.EditableTargets) > 0 {
		paths := make([]string, 0, len(plan.EditableTargets))
		for _, target := range plan.EditableTargets {
			paths = append(paths, target.Path)
		}
		sort.Strings(paths)
		var builder strings.Builder
		builder.WriteString("yunka ownership check")
		for _, path := range paths {
			builder.WriteString(" --path ")
			builder.WriteString(strconv.Quote(path))
		}
		builder.WriteString(" --format json")
		gates = append(gates, Gate{Command: builder.String(), Purpose: "reconfirm AX2 mutation ownership before editing exact planned targets"})
	}
	gates = append(gates,
		Gate{Command: "yunka generate", Purpose: "regenerate canonical derived artifacts"},
		Gate{Command: "yunka check --format agent-json", Purpose: "validate structure, generated drift, and receive machine remediation if validation fails"},
	)
	if strings.TrimSpace(goModule) != "" {
		gates = append(gates, Gate{Command: "go test ./...", Purpose: "run consumer Go tests"})
	}
	gates = append(gates, Gate{Command: "yunka dev", Purpose: "start the qualified developer runtime and verify readiness/runtime behavior"})
	return gates
}

func Diagnose(err error) diagnostic.Diagnostic {
	var failure *Failure
	if !errors.As(err, &failure) {
		item := diagnostic.MustDefinition(diagnostic.CodeDeveloperWorkflowFailed).Diagnostic(diagnostic.SeverityError)
		item.Detail = strings.TrimSpace(err.Error())
		return item
	}
	code := diagnostic.CodeChangeEvidence
	switch failure.Kind {
	case FailureOperation:
		code = diagnostic.CodeChangeOperation
	case FailureIntent:
		code = diagnostic.CodeChangeIntent
	case FailureEvidence:
		code = diagnostic.CodeChangeEvidence
	}
	item := diagnostic.MustDefinition(code).Diagnostic(diagnostic.SeverityError)
	item.Detail = strings.TrimSpace(failure.Err.Error())
	if failure.Location != "" {
		item.Location = &diagnostic.Location{Path: failure.Location}
	}
	return item
}

func Render(plan Plan, format string) (string, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case FormatJSON, FormatAgentJSON:
		contents, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return "", err
		}
		return string(append(contents, '\n')), nil
	case "", FormatText:
		return renderText(plan), nil
	default:
		return "", fmt.Errorf("change plan: unsupported format %q", format)
	}
}

func printFailure(command, format string, item diagnostic.Diagnostic, exitCode int) error {
	var output string
	var err error
	switch format {
	case FormatAgentJSON:
		var contents []byte
		contents, err = diagnostic.RenderAgentJSON(command, []diagnostic.Diagnostic{item}, false)
		output = string(contents)
	case FormatJSON:
		var contents []byte
		contents, err = diagnostic.RenderJSON(command, []diagnostic.Diagnostic{item})
		output = string(contents)
	default:
		output, err = diagnostic.RenderText([]diagnostic.Diagnostic{item})
	}
	if err != nil {
		return err
	}
	fmt.Print(output)
	return cli.NewExitError("", exitCode)
}

func renderText(plan Plan) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "operation %s\nintent    %s\n", plan.Operation.ID, plan.Intent)
	builder.WriteString("editable targets:\n")
	if len(plan.EditableTargets) == 0 {
		builder.WriteString("  (none)\n")
	}
	for _, target := range plan.EditableTargets {
		fmt.Fprintf(&builder, "  %s [%s]\n", target.Path, target.Owner)
	}
	builder.WriteString("unresolved targets:\n")
	if len(plan.UnresolvedTargets) == 0 {
		builder.WriteString("  (none)\n")
	}
	for _, target := range plan.UnresolvedTargets {
		fmt.Fprintf(&builder, "  %s scope=%s candidates=%d — %s\n", target.Kind, target.Scope, len(target.Candidates), target.Reason)
	}
	builder.WriteString("generated effects:\n")
	for _, effect := range plan.GeneratedEffects {
		location := effect.Path
		if location == "" {
			location = effect.Scope
		}
		fmt.Fprintf(&builder, "  %s %s conditional=%t\n", effect.Stage, location, effect.Conditional)
	}
	fmt.Fprintf(&builder, "affected dependencies=%d dependents=%d\n", len(plan.Affected.Dependencies), len(plan.Affected.Dependents))
	builder.WriteString("gates:\n")
	for _, gate := range plan.Gates {
		fmt.Fprintf(&builder, "  %s — %s\n", gate.Command, gate.Purpose)
	}
	builder.WriteString("risks:\n")
	if len(plan.Risks) == 0 {
		builder.WriteString("  (none from canonical evidence)\n")
	}
	for _, risk := range plan.Risks {
		fmt.Fprintf(&builder, "  %s=%s [%s]\n", risk.Kind, risk.Value, risk.Evidence)
	}
	return builder.String()
}

func normalizePlan(plan *Plan) {
	sort.Slice(plan.EditableTargets, func(i, j int) bool { return plan.EditableTargets[i].Path < plan.EditableTargets[j].Path })
	for index := range plan.UnresolvedTargets {
		plan.UnresolvedTargets[index].Candidates = uniqueSorted(plan.UnresolvedTargets[index].Candidates)
	}
	sort.Slice(plan.UnresolvedTargets, func(i, j int) bool {
		if plan.UnresolvedTargets[i].Kind != plan.UnresolvedTargets[j].Kind {
			return plan.UnresolvedTargets[i].Kind < plan.UnresolvedTargets[j].Kind
		}
		return plan.UnresolvedTargets[i].Scope < plan.UnresolvedTargets[j].Scope
	})
	sort.Slice(plan.GeneratedEffects, func(i, j int) bool {
		if plan.GeneratedEffects[i].Stage != plan.GeneratedEffects[j].Stage {
			return plan.GeneratedEffects[i].Stage < plan.GeneratedEffects[j].Stage
		}
		if plan.GeneratedEffects[i].Path != plan.GeneratedEffects[j].Path {
			return plan.GeneratedEffects[i].Path < plan.GeneratedEffects[j].Path
		}
		return plan.GeneratedEffects[i].Scope < plan.GeneratedEffects[j].Scope
	})
	if plan.EditableTargets == nil {
		plan.EditableTargets = []EditableTarget{}
	}
	if plan.UnresolvedTargets == nil {
		plan.UnresolvedTargets = []UnresolvedTarget{}
	}
	if plan.GeneratedEffects == nil {
		plan.GeneratedEffects = []GeneratedEffect{}
	}
	if plan.Gates == nil {
		plan.Gates = []Gate{}
	}
	if plan.Risks == nil {
		plan.Risks = []Risk{}
	}
}

func cloneAttributes(values map[string]string) map[string]string {
	if len(values) == 0 {
		return map[string]string{}
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func uniqueSorted(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = filepath.ToSlash(filepath.Clean(filepath.FromSlash(strings.TrimSpace(value))))
		if value == "." || value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func relativeTo(root, path string) string {
	value, err := filepath.Rel(root, path)
	if err != nil || value == ".." || strings.HasPrefix(value, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(value)
}

func join(root, name string) string {
	return filepath.ToSlash(filepath.Join(filepath.FromSlash(root), filepath.FromSlash(name)))
}

// Keep os referenced in the command package so future evidence adapters can
// remain read-only without adding a separate package solely for existence
// checks; canonical loaders above are the current owners of file evidence.
var _ = os.ErrNotExist