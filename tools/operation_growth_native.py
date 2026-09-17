#!/usr/bin/env python3
from pathlib import Path
import subprocess

ROOT = Path.cwd()
OLD_FOUNDATION = "23789633f2e0dbcd20230cacca42e814047d86bc"
OLD_DECISION = "f8fdce6c3bd7bd63eef05ae23b3fc9ba83c3b0de"


def run(*args, input=None):
    return subprocess.run(args, cwd=ROOT, input=input, text=True, check=True, capture_output=True).stdout


def read(path):
    return (ROOT / path).read_text()


def write(path, text):
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text)


def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:120]!r}")
    write(path, text.replace(old, new, 1))


def copy_old(commit, path):
    write(path, run("git", "show", f"{commit}:{path}"))


# Reuse only qualified pure/new foundation files. Existing current-main files are
# patched below rather than checked out from the stale stack.
copy_old(OLD_FOUNDATION, "pkg/contract/boundary_intent.go")
copy_old(OLD_FOUNDATION, "pkg/contract/boundary_intent_test.go")
copy_old(OLD_DECISION, "app/cmd/boundarycore/inspect.go")
copy_old(OLD_DECISION, "app/cmd/boundarycore/addition_scope.go")
copy_old(OLD_DECISION, "app/cmd/boundarycore/decision.go")

# Canonical DSL / manifest v5.
replace_once(
    "contracts/proto/yunka/dsl/v1/options.proto",
    "message OperationDeclaration {\n",
    "message BoundaryIntent {\n"
    "  // Explicit architecture facts only. Missing intent remains unknown.\n"
    "  string context = 1;\n"
    "  string aggregate = 2;\n"
    "  string aggregate_not_applicable_reason = 3;\n"
    "}\n\n"
    "message OperationDeclaration {\n",
)
replace_once(
    "contracts/proto/yunka/dsl/v1/options.proto",
    "  string application_method = 13;\n}",
    "  string application_method = 13;\n"
    "  // Architectural Service Boundary intent. This is not runtime authority.\n"
    "  BoundaryIntent boundary = 14;\n}",
)

replace_once("pkg/contract/model.go", "const ManifestVersion = 4", "const ManifestVersion = 5")
replace_once(
    "pkg/contract/model.go",
    "if manifest.SchemaVersion == 0 || manifest.SchemaVersion == 1 || manifest.SchemaVersion == 2 || manifest.SchemaVersion == 3 {",
    "if manifest.SchemaVersion == 0 || manifest.SchemaVersion == 1 || manifest.SchemaVersion == 2 || manifest.SchemaVersion == 3 || manifest.SchemaVersion == 4 {",
)
replace_once(
    "pkg/contract/model.go",
    "type OperationDeclaration struct {\n",
    "type BoundaryIntent struct {\n"
    "\tContext                      string `json:\"context\"`\n"
    "\tAggregate                    string `json:\"aggregate,omitempty\"`\n"
    "\tAggregateNotApplicableReason string `json:\"aggregateNotApplicableReason,omitempty\"`\n"
    "}\n\n"
    "type OperationDeclaration struct {\n",
)
replace_once(
    "pkg/contract/model.go",
    "\tApplicationMethod  string           `json:\"applicationMethod,omitempty\"`\n}",
    "\tApplicationMethod  string           `json:\"applicationMethod,omitempty\"`\n"
    "\tBoundary           *BoundaryIntent  `json:\"boundary,omitempty\"`\n}",
)
replace_once(
    "pkg/contract/model.go",
    "\toperation.ApplicationMethod = strings.TrimSpace(operation.ApplicationMethod)\n",
    "\toperation.ApplicationMethod = strings.TrimSpace(operation.ApplicationMethod)\n"
    "\tif operation.Boundary != nil {\n"
    "\t\toperation.Boundary.Context = strings.TrimSpace(operation.Boundary.Context)\n"
    "\t\toperation.Boundary.Aggregate = strings.TrimSpace(operation.Boundary.Aggregate)\n"
    "\t\toperation.Boundary.AggregateNotApplicableReason = strings.TrimSpace(operation.Boundary.AggregateNotApplicableReason)\n"
    "\t}\n",
)
replace_once(
    "pkg/contract/model.go",
    "\tif operation.Execution != nil {\n\t\texecution := *operation.Execution\n\t\tclone.Execution = &execution\n\t}\n\treturn clone\n",
    "\tif operation.Execution != nil {\n\t\texecution := *operation.Execution\n\t\tclone.Execution = &execution\n\t}\n"
    "\tif operation.Boundary != nil {\n\t\tboundary := *operation.Boundary\n\t\tclone.Boundary = &boundary\n\t}\n\treturn clone\n",
)

replace_once(
    "pkg/contract/dsl_descriptor.go",
    "\t\tcase 13:\n\t\t\tresult.ApplicationMethod = string(field.Bytes)\n",
    "\t\tcase 13:\n\t\t\tresult.ApplicationMethod = string(field.Bytes)\n"
    "\t\tcase 14:\n"
    "\t\t\tif field.Type == 2 {\n"
    "\t\t\t\tboundary, err := parseBoundaryIntent(field.Bytes)\n"
    "\t\t\t\tif err != nil {\n\t\t\t\t\treturn err\n\t\t\t\t}\n"
    "\t\t\t\tresult.Boundary = boundary\n\t\t\t}\n",
)

# Lint loaded manifests too; source compilation already rejects malformed wire data.
replace_once(
    "pkg/contract/lint.go",
    "\t\t\tif operation := method.Operation; operation != nil {\n",
    "\t\t\tif operation := method.Operation; operation != nil {\n"
    "\t\t\t\tif err := ValidateBoundaryIntent(operation.Boundary); err != nil {\n"
    "\t\t\t\t\tdiagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path + \".boundary\", Message: err.Error()})\n"
    "\t\t\t\t}\n",
)
replace_once(
    "pkg/contract/lint.go",
    "\tvar diagnostics []Diagnostic\n\tif !validPolicyKey(operation.ID) {",
    "\tvar diagnostics []Diagnostic\n"
    "\tif err := ValidateBoundaryIntent(operation.Boundary); err != nil {\n"
    "\t\tdiagnostics = append(diagnostics, Diagnostic{Severity: SeverityError, Path: path + \".boundary\", Message: err.Error()})\n"
    "\t}\n"
    "\tif !validPolicyKey(operation.ID) {",
)

# Current-main policy naming and first-Operation initialization. The policy stays
# conservative for growth; initialization is not growth and may establish a new
# empty Application boundary.
decision_path = ROOT / "app/cmd/boundarycore/decision.go"
decision = decision_path.read_text()
decision = decision.replace("ReuseExistingApplication", "ReuseExistingService")
decision = decision.replace("CreateNewApplication", "CreateNewService")
decision = decision.replace('"reuse_existing_application"', '"reuse_existing_service"')
decision = decision.replace('"create_new_application"', '"create_new_service"')
decision = decision.replace(
    "if !fullCommitSHA(request.BaseSHA) || request.Application != strings.TrimSpace(request.Application) || request.OperationID == \"\" || request.OperationID != strings.TrimSpace(request.OperationID) {\n\t\treturn ServiceBoundaryDecision{}, fmt.Errorf(\"boundary addition: exact commit SHA, application and operation identity are required\")\n\t}",
    "if (request.BaseSHA != \"\" && !fullCommitSHA(request.BaseSHA)) || request.Application != strings.TrimSpace(request.Application) || request.OperationID == \"\" || request.OperationID != strings.TrimSpace(request.OperationID) {\n\t\treturn ServiceBoundaryDecision{}, fmt.Errorf(\"boundary addition: optional base SHA must be exact; application and operation identity are required\")\n\t}",
)
old_outcome = """knownContexts := left.IntentCoverage.Contexts
\tif scope.Verdict == \"same\" && candidate.Boundary != nil && len(knownContexts) > 0 && !containsString(knownContexts, candidate.Boundary.Context) {
\t\td.Outcome = CreateNewService
\t} else {
\t\tall := true
\t\tfor _, dim := range d.Dimensions {
\t\t\tif dim.Critical && dim.Verdict != \"same\" && dim.Verdict != \"compatible\" {
\t\t\t\tall = false
\t\t\t}
\t\t}
\t\tif all {
\t\t\td.Outcome = ReuseExistingService
\t\t}
\t}
"""
new_outcome = """knownContexts := left.IntentCoverage.Contexts
\tif len(left.Fingerprint.Operations) == 0 && scope.Verdict == \"same\" {
\t\t// Establishing the first Operation is initialization, not Operation Growth.
\t\t// Subsequent additions must satisfy the normal peer-evidence policy.
\t\td.Outcome = ReuseExistingService
\t} else if scope.Verdict == \"same\" && candidate.Boundary != nil && len(knownContexts) > 0 && !containsString(knownContexts, candidate.Boundary.Context) {
\t\td.Outcome = CreateNewService
\t} else {
\t\tall := true
\t\tfor _, dim := range d.Dimensions {
\t\t\tif dim.Critical && dim.Verdict != \"same\" && dim.Verdict != \"compatible\" {
\t\t\t\tall = false
\t\t\t}
\t\t}
\t\tif all {
\t\t\td.Outcome = ReuseExistingService
\t\t}
\t}
"""
if old_outcome not in decision:
    raise SystemExit("decision.go: outcome block not found")
decision = decision.replace(old_outcome, new_outcome, 1)
decision_path.write_text(decision)

# Universal current/base growth detector. It deliberately consumes canonical
# manifests, not persisted decision JSON.
write("app/cmd/boundarycore/growth.go", r'''package boundarycore

import (
    "encoding/json"
    "sort"
    "strings"

    "github.com/hvritual/yunka.io/pkg/contract"
)

const GrowthPolicyVersion = "operation-growth/v1"

const (
    GrowthOperationAdded         = "operation_added"
    GrowthOperationMoved         = "operation_moved_service"
    GrowthContextChanged         = "operation_context_changed"
    GrowthAggregateChanged       = "operation_aggregate_changed"
    GrowthPermissionChanged      = "permission_domain_changed"
    GrowthDependencyExpanded     = "dependency_expanded"
    GrowthCompositionChanged     = "composition_changed"
    GrowthConsistencyChanged     = "consistency_changed"
)

type GrowthEvent struct {
    PolicyVersion     string                   `json:"policyVersion"`
    Kind              string                   `json:"kind"`
    OperationID       string                   `json:"operationId"`
    BeforeApplication string                   `json:"beforeApplication,omitempty"`
    AfterApplication  string                   `json:"afterApplication,omitempty"`
    BeforeService     string                   `json:"beforeService,omitempty"`
    AfterService      string                   `json:"afterService,omitempty"`
    Outcome           string                   `json:"outcome"`
    Reason            string                   `json:"reason"`
    Decision          *ServiceBoundaryDecision `json:"decision,omitempty"`
}

type operationState struct {
    Application string
    Service     string
    Operation   contract.OperationDeclaration
}

func EvaluateGrowth(baseSHA string, before, after contract.Manifest) []GrowthEvent {
    before = detachedManifest(before)
    after = detachedManifest(after)
    left := operationStates(before)
    right := operationStates(after)
    ids := map[string]struct{}{}
    for id := range left { ids[id] = struct{}{} }
    for id := range right { ids[id] = struct{}{} }
    ordered := make([]string, 0, len(ids))
    for id := range ids { ordered = append(ordered, id) }
    sort.Strings(ordered)

    events := []GrowthEvent{}
    for _, id := range ordered {
        b, bok := left[id]
        a, aok := right[id]
        if !aok { // removal is not growth
            continue
        }
        if !bok {
            event := GrowthEvent{PolicyVersion: GrowthPolicyVersion, Kind: GrowthOperationAdded, OperationID: id, AfterApplication: a.Application, AfterService: a.Service, Outcome: ArchitectureReviewRequired, Reason: "new Operation requires a boundary decision against the immutable base"}
            if a.Application != "" {
                decision, err := EvaluateAddition(AdditionRequest{BaseSHA: baseSHA, Application: a.Application, OperationID: id}, before, after)
                if err == nil {
                    event.Outcome = decision.Outcome
                    event.Reason = "single-addition boundary policy recomputed from canonical base/current facts"
                    event.Decision = &decision
                } else {
                    event.Reason = "boundary decision could not be established: " + err.Error()
                }
            }
            events = append(events, event)
            continue
        }
        appendEvent := func(kind, outcome, reason string) {
            events = append(events, GrowthEvent{PolicyVersion: GrowthPolicyVersion, Kind: kind, OperationID: id, BeforeApplication: b.Application, AfterApplication: a.Application, BeforeService: b.Service, AfterService: a.Service, Outcome: outcome, Reason: reason})
        }
        if b.Application != a.Application || b.Service != a.Service {
            appendEvent(GrowthOperationMoved, ArchitectureReviewRequired, "existing Operation moved across its canonical Application/Service projection")
        }
        if boundaryContext(b.Operation.Boundary) != boundaryContext(a.Operation.Boundary) {
            outcome := ArchitectureReviewRequired
            if b.Operation.Boundary != nil && a.Operation.Boundary != nil { outcome = CreateNewService }
            appendEvent(GrowthContextChanged, outcome, "explicit context boundary changed")
        }
        if boundaryAggregate(b.Operation.Boundary) != boundaryAggregate(a.Operation.Boundary) {
            outcome := ArchitectureReviewRequired
            if b.Operation.Boundary != nil && a.Operation.Boundary != nil { outcome = CreateNewService }
            appendEvent(GrowthAggregateChanged, outcome, "explicit aggregate boundary changed")
        }
        if securityBoundary(b.Operation) != securityBoundary(a.Operation) {
            appendEvent(GrowthPermissionChanged, ArchitectureReviewRequired, "canonical access/permission/authentication facts changed; permission names are not reinterpreted as business domains")
        }
        if expanded(b.Operation.RequiresOperations, a.Operation.RequiresOperations) {
            appendEvent(GrowthDependencyExpanded, ArchitectureReviewRequired, "Operation dependency set expanded")
        }
        if strings.TrimSpace(b.Operation.Composition) != strings.TrimSpace(a.Operation.Composition) {
            appendEvent(GrowthCompositionChanged, ArchitectureReviewRequired, "composition boundary changed")
        }
        if jsonValue(b.Operation.Execution) != jsonValue(a.Operation.Execution) {
            appendEvent(GrowthConsistencyChanged, ArchitectureReviewRequired, "execution consistency policy changed")
        }
    }
    sort.Slice(events, func(i, j int) bool {
        if events[i].OperationID != events[j].OperationID { return events[i].OperationID < events[j].OperationID }
        return events[i].Kind < events[j].Kind
    })
    return events
}

func operationStates(manifest contract.Manifest) map[string]operationState {
    result := map[string]operationState{}
    for _, service := range manifest.Services {
        application := ""
        if service.Application != nil { application = strings.TrimSpace(service.Domain) + "/" + strings.TrimSpace(service.Application.Name) }
        for _, method := range service.Methods {
            if method.Operation == nil || strings.TrimSpace(method.Operation.ID) == "" { continue }
            result[method.Operation.ID] = operationState{Application: application, Service: service.FullName, Operation: *method.Operation}
        }
        if service.Application != nil {
            for _, operation := range service.Application.Operations {
                if strings.TrimSpace(operation.ID) == "" { continue }
                result[operation.ID] = operationState{Application: application, Service: service.FullName, Operation: operation}
            }
        }
    }
    return result
}

func boundaryContext(value *contract.BoundaryIntent) string {
    if value == nil { return "<unknown>" }
    return strings.TrimSpace(value.Context)
}
func boundaryAggregate(value *contract.BoundaryIntent) string {
    if value == nil { return "<unknown>" }
    if v := strings.TrimSpace(value.Aggregate); v != "" { return "aggregate:" + v }
    return "not-applicable:" + strings.TrimSpace(value.AggregateNotApplicableReason)
}
func securityBoundary(value contract.OperationDeclaration) string {
    return jsonValue(struct {
        Public bool `json:"public"`
        Permissions []string `json:"permissions"`
        PermissionMode string `json:"permissionMode"`
        TenantRequired bool `json:"tenantRequired"`
        Authentication []string `json:"authentication"`
    }{value.Public, value.Permissions, value.PermissionMode, value.TenantRequired, value.Authentication})
}
func expanded(before, after []string) bool {
    known := map[string]struct{}{}
    for _, value := range before { known[strings.TrimSpace(value)] = struct{}{} }
    for _, value := range after { if _, ok := known[strings.TrimSpace(value)]; !ok { return true } }
    return false
}
func jsonValue(value any) string {
    data, _ := json.Marshal(value)
    return string(data)
}
''')

# Current-main native authoring adapter. It compiles current canonical source and
# builds only an in-memory prospective Manifest; persistent source is untouched
# until the decision allows reuse/initialization.
write("app/cmd/add/boundary_gate.go", r'''package add

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/hvritual/yunka.io/pkg/contract"
    "yunka.io/app/cmd/boundarycore"
    "yunka.io/app/cmd/projectflow"
)

type OperationBoundarySemantics struct {
    Context string `json:"context"`
    Aggregate string `json:"aggregate,omitempty"`
    AggregateNotApplicableReason string `json:"aggregateNotApplicableReason,omitempty"`
}

type OperationBoundaryDecision = boundarycore.ServiceBoundaryDecision

func operationBoundaryIntent(options OperationOptions) *contract.BoundaryIntent {
    if options.BoundaryContext == "" && options.BoundaryAggregate == "" && options.BoundaryNoAggregateReason == "" { return nil }
    return &contract.BoundaryIntent{Context: options.BoundaryContext, Aggregate: options.BoundaryAggregate, AggregateNotApplicableReason: options.BoundaryNoAggregateReason}
}

func evaluateOperationBoundary(root, sourcePath, domain, application, packageName, rpcName, requestType, responseType string, options OperationOptions) (*OperationBoundaryDecision, error) {
    snapshot, err := projectflow.DescribeContractSourceSnapshot(context.Background(), projectflow.Options{Root: root, ProtoPaths: append([]string(nil), options.ProtoPaths...)})
    if err != nil { return nil, fmt.Errorf("add operation: compile current canonical boundary facts: %w", err) }
    before := snapshot.Manifest
    data, err := json.Marshal(before)
    if err != nil { return nil, err }
    var after contract.Manifest
    if err := json.Unmarshal(data, &after); err != nil { return nil, err }
    after.Normalize()

    applicationKey := domain + "/" + application
    var target *contract.Service
    for i := range after.Services {
        service := &after.Services[i]
        if service.Application == nil || service.Domain+"/"+service.Application.Name != applicationKey { continue }
        if target != nil { return nil, fmt.Errorf("add operation: application %s has multiple canonical Service projections", applicationKey) }
        target = service
    }
    if target == nil { return nil, fmt.Errorf("add operation: canonical application %s was not found", applicationKey) }
    if projectPath, ok := snapshot.Paths[target.SourceFile]; ok && cleanRelative(projectPath) != cleanRelative(sourcePath) {
        return nil, fmt.Errorf("add operation: selected source %s disagrees with canonical Service source %s", sourcePath, projectPath)
    }

    permissionMode := options.PermissionMode
    if permissionMode == "" { permissionMode = "all" }
    composition := options.Composition
    if composition == "none" { composition = "" }
    operation := contract.OperationDeclaration{
        ID: options.OperationID, UseCase: options.UseCase, Permissions: append([]string(nil), options.Permissions...), PermissionMode: permissionMode,
        TenantRequired: options.Tenant == "required", Authentication: append([]string(nil), options.Authentication...), Public: options.Access == "public",
        RequiresOperations: append([]string(nil), options.RequiresOperations...), Composition: composition,
        Execution: &contract.ExecutionPolicy{Transaction: options.Transaction, Idempotency: options.Idempotency}, Boundary: operationBoundaryIntent(options),
    }
    requestFull := packageName + "." + requestType
    responseFull := packageName + "." + responseType
    method := contract.Method{Name: rpcName, FullName: target.FullName + "." + rpcName, SourceFile: target.SourceFile, Request: requestFull, Response: responseFull, Operation: &operation}
    if options.Access == "protected" {
        method.Authorization = &contract.AuthorizationPolicy{OperationID: options.OperationID, Permissions: append([]string(nil), options.Permissions...), PermissionMode: permissionMode, TenantRequired: options.Tenant == "required", Authentication: append([]string(nil), options.Authentication...)}
    }
    if options.HTTPMethod != "" { method.HTTP = []contract.HTTPBinding{{Method: options.HTTPMethod, Path: options.HTTPPath, Body: options.HTTPBody}} }
    target.Methods = append(target.Methods, method)

    hasMessage := func(name string) bool { for _, item := range after.Messages { if item.FullName == name { return true } }; return false }
    if !hasMessage(requestFull) { after.Messages = append(after.Messages, contract.Message{Name: requestType, FullName: requestFull, SourceFile: target.SourceFile, DTO: &contract.DTODeclaration{Kind: "input"}, Fields: []contract.Field{}}) }
    if !hasMessage(responseFull) { after.Messages = append(after.Messages, contract.Message{Name: responseType, FullName: responseFull, SourceFile: target.SourceFile, DTO: &contract.DTODeclaration{Kind: "output"}, Fields: []contract.Field{}}) }
    after.Normalize()

    decision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{Application: applicationKey, OperationID: options.OperationID}, before, after)
    if err != nil { return nil, err }
    return &decision, nil
}

func boundaryAllowsMutation(value *OperationBoundaryDecision) bool {
    return value != nil && value.Outcome == boundarycore.ReuseExistingService
}

func boundaryBlockedError(value *OperationBoundaryDecision) error {
    if value == nil { return fmt.Errorf("add operation: boundary decision is missing") }
    return fmt.Errorf("add operation: Operation Growth boundary outcome=%s; review the plan boundaryDecision before changing the Service", value.Outcome)
}

func boundarySemantics(value *contract.BoundaryIntent) *OperationBoundarySemantics {
    if value == nil { return nil }
    return &OperationBoundarySemantics{Context: strings.TrimSpace(value.Context), Aggregate: strings.TrimSpace(value.Aggregate), AggregateNotApplicableReason: strings.TrimSpace(value.AggregateNotApplicableReason)}
}
''')

# Add report/options/CLI fields without changing unrelated add semantics.
replace_once(
    "app/cmd/add/command.go",
    "type OperationSemantics struct {\n",
    "type OperationSemantics struct {\n\tBoundary           *OperationBoundarySemantics `json:\"boundary,omitempty\"`\n",
)
replace_once(
    "app/cmd/add/command.go",
    "\tExplicitSemantics *OperationSemantics `json:\"explicitSemantics,omitempty\"`\n",
    "\tExplicitSemantics *OperationSemantics `json:\"explicitSemantics,omitempty\"`\n"
    "\tBoundaryDecision  *OperationBoundaryDecision `json:\"boundaryDecision,omitempty\"`\n"
    "\tProtoPaths        []string `json:\"protoPaths,omitempty\"`\n",
)
replace_once(
    "app/cmd/add/command.go",
    "\tHTTPBody           string\n}",
    "\tHTTPBody           string\n"
    "\tBoundaryContext    string\n"
    "\tBoundaryAggregate  string\n"
    "\tBoundaryNoAggregateReason string\n"
    "\tProtoPaths         []string\n}",
)
replace_once(
    "app/cmd/add/command.go",
    "\t\tcli.StringFlag{Name: \"http-body\", Usage: \"optional HTTP body mapping; typically *\"},\n",
    "\t\tcli.StringFlag{Name: \"http-body\", Usage: \"optional HTTP body mapping; typically *\"},\n"
    "\t\tcli.StringFlag{Name: \"boundary-context\", Usage: \"explicit Service Boundary context key for Operation Growth\"},\n"
    "\t\tcli.StringFlag{Name: \"boundary-aggregate\", Usage: \"explicit aggregate key; mutually exclusive with --boundary-no-aggregate-reason\"},\n"
    "\t\tcli.StringFlag{Name: \"boundary-no-aggregate-reason\", Usage: \"explicit reason why aggregate does not apply\"},\n"
    "\t\tcli.StringSliceFlag{Name: \"proto-path\", Usage: \"additional protobuf include directory; repeatable for proto-root projects\"},\n",
)
replace_once(
    "app/cmd/add/command.go",
    "\t\t\t\tHTTPBody:           c.String(\"http-body\"),\n",
    "\t\t\t\tHTTPBody:           c.String(\"http-body\"),\n"
    "\t\t\t\tBoundaryContext:    c.String(\"boundary-context\"),\n"
    "\t\t\t\tBoundaryAggregate:  c.String(\"boundary-aggregate\"),\n"
    "\t\t\t\tBoundaryNoAggregateReason: c.String(\"boundary-no-aggregate-reason\"),\n"
    "\t\t\t\tProtoPaths:         append([]string(nil), c.StringSlice(\"proto-path\")...),\n",
)

# Normalize/validate/render explicit BoundaryIntent.
replace_once(
    "app/cmd/add/semantics.go",
    "\toptions.HTTPBody = strings.TrimSpace(options.HTTPBody)\n",
    "\toptions.HTTPBody = strings.TrimSpace(options.HTTPBody)\n"
    "\toptions.BoundaryContext = strings.TrimSpace(options.BoundaryContext)\n"
    "\toptions.BoundaryAggregate = strings.TrimSpace(options.BoundaryAggregate)\n"
    "\toptions.BoundaryNoAggregateReason = strings.TrimSpace(options.BoundaryNoAggregateReason)\n",
)
replace_once(
    "app/cmd/add/semantics.go",
    "\tif options.HTTPMethod != \"\" && !oneOf(options.HTTPMethod, \"GET\", \"POST\", \"PUT\", \"PATCH\", \"DELETE\") {\n\t\treturn fmt.Errorf(\"add operation: unsupported HTTP method %s\", options.HTTPMethod)\n\t}\n\treturn nil\n",
    "\tif options.HTTPMethod != \"\" && !oneOf(options.HTTPMethod, \"GET\", \"POST\", \"PUT\", \"PATCH\", \"DELETE\") {\n\t\treturn fmt.Errorf(\"add operation: unsupported HTTP method %s\", options.HTTPMethod)\n\t}\n"
    "\tif intent := operationBoundaryIntent(*options); intent != nil {\n"
    "\t\tif err := contract.ValidateBoundaryIntent(intent); err != nil { return fmt.Errorf(\"add operation: %w\", err) }\n"
    "\t}\n"
    "\treturn nil\n",
)
replace_once(
    "app/cmd/add/semantics.go",
    "\tfmt.Fprintf(&b, \"      execution: { transaction: %s idempotency: %s }\\n\", transactionEnum(options.Transaction), idempotencyEnum(options.Idempotency))\n",
    "\tfmt.Fprintf(&b, \"      execution: { transaction: %s idempotency: %s }\\n\", transactionEnum(options.Transaction), idempotencyEnum(options.Idempotency))\n"
    "\tif intent := operationBoundaryIntent(options); intent != nil {\n"
    "\t\tfmt.Fprintf(&b, \"      boundary: { context: %q\", intent.Context)\n"
    "\t\tif intent.Aggregate != \"\" { fmt.Fprintf(&b, \" aggregate: %q\", intent.Aggregate) } else { fmt.Fprintf(&b, \" aggregate_not_applicable_reason: %q\", intent.AggregateNotApplicableReason) }\n"
    "\t\tb.WriteString(\" }\\n\")\n"
    "\t}\n",
)

replace_once(
    "app/cmd/add/plan_semantics.go",
    "\tresult := &OperationSemantics{\n",
    "\tresult := &OperationSemantics{\n\t\tBoundary:           boundarySemantics(operationBoundaryIntent(options)),\n",
)

# Revalidation preserves include profile and BoundaryIntent.
replace_once(
    "app/cmd/add/plan_validation.go",
    "\t\tRequiresOperations: append([]string{}, semantics.RequiresOperations...),\n\t}\n",
    "\t\tRequiresOperations: append([]string{}, semantics.RequiresOperations...),\n"
    "\t\tProtoPaths:          append([]string(nil), candidate.ProtoPaths...),\n\t}\n"
    "\tif semantics.Boundary != nil {\n"
    "\t\toptions.BoundaryContext = semantics.Boundary.Context\n"
    "\t\toptions.BoundaryAggregate = semantics.Boundary.Aggregate\n"
    "\t\toptions.BoundaryNoAggregateReason = semantics.Boundary.AggregateNotApplicableReason\n"
    "\t}\n",
)
replace_once(
    "app/cmd/add/plan_validation.go",
    "\tif candidateJSON != rebuiltJSON {\n\t\treturn Report{}, fmt.Errorf(\"add operation plan: supplied plan does not match canonical replan for the current project\")\n\t}\n\treturn rebuilt, nil\n",
    "\tif candidateJSON != rebuiltJSON {\n\t\treturn Report{}, fmt.Errorf(\"add operation plan: supplied plan does not match canonical replan for the current project\")\n\t}\n"
    "\tif !boundaryAllowsMutation(rebuilt.BoundaryDecision) {\n"
    "\t\treturn Report{}, fmt.Errorf(\"add operation plan: %w\", boundaryBlockedError(rebuilt.BoundaryDecision))\n"
    "\t}\n"
    "\treturn rebuilt, nil\n",
)

# Evaluate the prospective candidate before any ownership/persistent write.
replace_once(
    "app/cmd/add/operation.go",
    "\towner, err := requireEditable(inputs.Project.Root, source.Relative)\n",
    "\tboundaryDecision, err := evaluateOperationBoundary(inputs.Project.Root, source.Relative, domain, application, packageName, rpcName, requestType, responseType, options)\n"
    "\tif err != nil { return Report{}, sourceFailure(source.Relative, err) }\n"
    "\towner, err := requireEditable(inputs.Project.Root, source.Relative)\n",
)
replace_once(
    "app/cmd/add/operation.go",
    "\t\tNotes: []string{\n",
    "\t\tBoundaryDecision: boundaryDecision,\n"
    "\t\tProtoPaths: append([]string(nil), options.ProtoPaths...),\n"
    "\t\tNotes: []string{\n",
)
replace_once(
    "app/cmd/add/operation.go",
    "\tif !apply {\n\t\treport.Kind = \"operation-plan\"\n",
    "\tif !apply {\n\t\treport.Kind = \"operation-plan\"\n",
)
# Insert blocking apply immediately after plan-only return block.
replace_once(
    "app/cmd/add/operation.go",
    "\t\tnormalizeReport(&report)\n\t\treturn report, nil\n\t}\n\n\tif sealed {\n",
    "\t\tnormalizeReport(&report)\n\t\treturn report, nil\n\t}\n"
    "\tif !boundaryAllowsMutation(boundaryDecision) {\n"
    "\t\tnormalizeReport(&report)\n"
    "\t\treturn report, conflictFailure(source.Relative, boundaryBlockedError(boundaryDecision))\n"
    "\t}\n\n\tif sealed {\n",
)

replace_once(
    "app/cmd/add/helpers.go",
    "\tif report.Identity == nil {\n\t\treport.Identity = map[string]string{}\n\t}\n",
    "\tif report.Identity == nil {\n\t\treport.Identity = map[string]string{}\n\t}\n"
    "\tif report.ProtoPaths == nil { report.ProtoPaths = []string{} }\n",
)

# Make add test fixtures compile canonical protobuf by carrying the repository DSL.
replace_once(
    "app/cmd/add/main_test.go",
    "\tif err := os.MkdirAll(filepath.Join(root, \"modules\"), 0o755); err != nil {\n\t\tt.Fatal(err)\n\t}\n",
    "\tif err := os.MkdirAll(filepath.Join(root, \"modules\"), 0o755); err != nil {\n\t\tt.Fatal(err)\n\t}\n"
    "\tsupport, err := os.ReadFile(filepath.Join(\"..\", \"..\", \"..\", \"contracts\", \"proto\", \"yunka\", \"dsl\", \"v1\", \"options.proto\"))\n"
    "\tif err != nil { t.Fatal(err) }\n"
    "\tmustWriteFile(t, filepath.Join(root, \"contracts\", \"proto\", \"yunka\", \"dsl\", \"v1\", \"options.proto\"), string(support))\n",
)

# Current-main exact direct-edit gate: compile immutable base/current source and
# classify pairwise growth as new blocking audit debt.
write("app/cmd/audit/boundary_growth.go", r'''package audit

import (
    "context"
    "fmt"

    "yunka.io/app/cmd/auditcore"
    "yunka.io/app/cmd/boundarycore"
    "yunka.io/app/cmd/projectflow"
)

const RuleOperationGrowthBoundary = "AUDIT-BOUNDARY-001"

func boundaryGrowthFindings(currentRoot, baselineRoot, baseSHA string) ([]auditcore.Finding, error) {
    baseline, err := projectflow.DescribeContractSourceSnapshot(context.Background(), projectflow.Options{Root: baselineRoot})
    if err != nil { return nil, fmt.Errorf("audit boundary growth: compile immutable baseline %s: %w", baseSHA, err) }
    current, err := projectflow.DescribeContractSourceSnapshot(context.Background(), projectflow.Options{Root: currentRoot})
    if err != nil { return nil, fmt.Errorf("audit boundary growth: compile current canonical source: %w", err) }
    events := boundarycore.EvaluateGrowth(baseSHA, baseline.Manifest, current.Manifest)
    result := []auditcore.Finding{}
    for _, event := range events {
        if event.Outcome == boundarycore.ReuseExistingService { continue }
        result = append(result, auditcore.Finding{
            ID: RuleOperationGrowthBoundary + ":" + event.OperationID + ":" + event.Kind,
            Rule: RuleOperationGrowthBoundary, Class: auditcore.FindingProvenViolation, Blocking: true,
            Subject: event.OperationID,
            Summary: "Operation Growth is not proven to remain inside the existing Service Boundary",
            Invariant: "new or boundary-changing Operations must recompute canonical base/current Service Boundary evidence; non-reuse outcomes cannot enter silently",
            Reason: event.Reason + "; outcome=" + event.Outcome,
            Remediation: "review the boundary decision; split the Service/Application when contradicted, or supply explicit compatible canonical boundary evidence before growth",
            Evidence: []auditcore.Evidence{
                {Kind: auditcore.EvidenceGit, Source: "git.base", Detail: baseSHA},
                {Kind: auditcore.EvidenceCanonical, Source: "contract.source", Detail: event.Kind + " outcome=" + event.Outcome},
            },
        })
    }
    return result, nil
}
''')

replace_once(
    "app/cmd/audit/command.go",
    "\tdebt := auditcore.CompareProvenFindings(baseline.Findings, current.Findings)\n",
    "\tgrowth, err := boundaryGrowthFindings(descriptor.Root, baselineRoot, baseSHA)\n"
    "\tif err != nil { return auditcore.Report{}, err }\n"
    "\tcurrent.Findings = append(current.Findings, growth...)\n"
    "\tdebt := auditcore.CompareProvenFindings(baseline.Findings, current.Findings)\n",
)

# ChangeSet v2 remains the persisted envelope. Recompute the same pairwise growth
# from its exact base/current canonical facts instead of adding old #169 proof JSON.
write("app/cmd/change/operation_growth.go", r'''package change

import (
    "github.com/hvritual/yunka.io/pkg/contract"
    "yunka.io/app/cmd/boundarycore"
)

const SemanticBoundary = "boundary"

func operationGrowthSemanticDeltas(value ChangeSet, before, after contract.Manifest) []SemanticDelta {
    result := []SemanticDelta{}
    for _, event := range boundarycore.EvaluateGrowth(value.BaseSHA, before, after) {
        result = append(result, SemanticDelta{
            Category: SemanticBoundary,
            Subject: "operation:" + event.OperationID,
            Field: event.Kind,
            Before: jsonValue(struct { Application, Service string }{event.BeforeApplication, event.BeforeService}),
            After: jsonValue(struct { Application, Service, Outcome, Reason string }{event.AfterApplication, event.AfterService, event.Outcome, event.Reason}),
            Allowed: event.Outcome == boundarycore.ReuseExistingService,
        })
    }
    // Bind the stored create-plan BoundaryIntent even when another current fact
    // would still happen to produce a reusable policy outcome.
    current := operationBoundaryIndex(after)
    for _, subject := range value.Subjects {
        if subject.Create == nil { continue }
        expected := subject.Create.Expected.Semantics.Boundary
        actual := current[subject.Create.Operation.OperationID]
        if jsonValue(expected) != jsonValue(actual) {
            result = append(result, SemanticDelta{Category: SemanticBoundary, Subject: "operation:" + subject.Create.Operation.OperationID, Field: "boundary_intent", Before: jsonValue(expected), After: jsonValue(actual), Allowed: false})
        }
    }
    return result
}

func operationBoundaryIndex(manifest contract.Manifest) map[string]*OperationBoundarySemantics {
    result := map[string]*OperationBoundarySemantics{}
    convert := func(value *contract.BoundaryIntent) *OperationBoundarySemantics {
        if value == nil { return nil }
        return &OperationBoundarySemantics{Context: value.Context, Aggregate: value.Aggregate, AggregateNotApplicableReason: value.AggregateNotApplicableReason}
    }
    for _, service := range manifest.Services {
        for _, method := range service.Methods { if method.Operation != nil { result[method.Operation.ID] = convert(method.Operation.Boundary) } }
        if service.Application != nil { for _, operation := range service.Application.Operations { result[operation.ID] = convert(operation.Boundary) } }
    }
    return result
}

type OperationBoundarySemantics struct {
    Context string `json:"context"`
    Aggregate string `json:"aggregate,omitempty"`
    AggregateNotApplicableReason string `json:"aggregateNotApplicableReason,omitempty"`
}
''')

replace_once(
    "app/cmd/change/changeset_reconcile.go",
    "\treport.Deltas = append(report.Deltas, compareChangeSetApplications(base, current, applicationAllowances)...)\n",
    "\treport.Deltas = append(report.Deltas, compareChangeSetApplications(base, current, applicationAllowances)...)\n"
    "\treport.Deltas = append(report.Deltas, operationGrowthSemanticDeltas(value, base.Manifest, current.Manifest)...)\n",
)

# Tests for the missing final invariant: current/base growth and direct .proto bypass.
write("app/cmd/boundarycore/growth_test.go", r'''package boundarycore

import (
    "testing"

    "github.com/hvritual/yunka.io/pkg/contract"
)

func TestEvaluateGrowthRejectsContextChangeAndUnknownAddition(t *testing.T) {
    base := growthManifest(&contract.BoundaryIntent{Context: "sales.orders", Aggregate: "order"})
    current := detachedManifest(base)
    current.Services[0].Methods[0].Operation.Boundary.Context = "billing.invoices"
    current.Services[0].Methods = append(current.Services[0].Methods, contract.Method{
        Name: "List", FullName: "sales.v1.Orders.List", SourceFile: "sales.proto", Request: "sales.v1.ReadRequest", Response: "sales.v1.ReadResponse",
        Operation: &contract.OperationDeclaration{ID: "orders.list", UseCase: "list_orders", Public: true, PermissionMode: "all", Execution: &contract.ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}},
    })
    events := EvaluateGrowth("0123456789012345678901234567890123456789", base, current)
    foundContext, foundAddition := false, false
    for _, event := range events {
        if event.Kind == GrowthContextChanged && event.Outcome == CreateNewService { foundContext = true }
        if event.Kind == GrowthOperationAdded && event.Outcome == ArchitectureReviewRequired { foundAddition = true }
    }
    if !foundContext || !foundAddition { t.Fatalf("events=%+v", events) }
}

func growthManifest(boundary *contract.BoundaryIntent) contract.Manifest {
    return contract.Manifest{SchemaVersion: contract.ManifestVersion,
        Files: []contract.File{{Name: "sales.proto", Package: "sales.v1", Domain: &contract.DomainDeclaration{Name: "sales"}}},
        Messages: []contract.Message{
            {Name: "ReadRequest", FullName: "sales.v1.ReadRequest", SourceFile: "sales.proto", DTO: &contract.DTODeclaration{Kind: "input"}, Fields: []contract.Field{}},
            {Name: "ReadResponse", FullName: "sales.v1.ReadResponse", SourceFile: "sales.proto", DTO: &contract.DTODeclaration{Kind: "output"}, Fields: []contract.Field{}},
        },
        Services: []contract.Service{{Name: "Orders", FullName: "sales.v1.Orders", SourceFile: "sales.proto", Domain: "sales", Application: &contract.ApplicationDeclaration{Name: "orders"}, Methods: []contract.Method{{
            Name: "Read", FullName: "sales.v1.Orders.Read", SourceFile: "sales.proto", Request: "sales.v1.ReadRequest", Response: "sales.v1.ReadResponse",
            Operation: &contract.OperationDeclaration{ID: "orders.read", UseCase: "read_order", Public: true, PermissionMode: "all", Execution: &contract.ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}, Boundary: boundary},
        }}}},
    }
}
''')

write("app/cmd/audit/boundary_growth_test.go", r'''package audit

import (
    "encoding/json"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/hvritual/yunka.io/pkg/contract"
    "yunka.io/app/cmd/auditcore"
)

func TestBuildWithBaseBlocksDirectOperationGrowthEvenWithStaleGeneratedManifest(t *testing.T) {
    root := t.TempDir()
    writeAuditProjectFile(t, filepath.Join(root, "go.mod"), "module example.com/demo\n\ngo 1.25.0\n")
    support, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "proto", "yunka", "dsl", "v1", "options.proto"))
    if err != nil { t.Fatal(err) }
    writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "yunka", "dsl", "v1", "options.proto"), string(support))
    baselineProto := boundaryAuditProto(false)
    writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "sales.proto"), baselineProto)
    manifest := contract.Manifest{SchemaVersion: contract.ManifestVersion, Files: []contract.File{{Name: "sales.proto", Domain: &contract.DomainDeclaration{Name: "sales"}}}}
    data, _ := json.MarshalIndent(manifest, "", "  ")
    writeAuditProjectFile(t, filepath.Join(root, "contracts", "generated", contract.ManifestFilename), string(append(data, '\n')))
    writeAuditProjectFile(t, filepath.Join(root, "internal", "sales", "application", "service.go"), "// Package application owns the fixture.\npackage application\n")
    gitAudit(t, root, "init")
    gitAudit(t, root, "config", "user.email", "audit@example.invalid")
    gitAudit(t, root, "config", "user.name", "Yunka Audit Test")
    gitAudit(t, root, "add", ".")
    gitAudit(t, root, "commit", "-m", "baseline")

    // Bypass `yunka add operation`: change canonical source only and deliberately
    // leave generated manifest stale. Base-aware audit must still see the growth.
    writeAuditProjectFile(t, filepath.Join(root, "contracts", "proto", "sales.proto"), boundaryAuditProto(true))
    report, err := BuildWithBase(root, "HEAD")
    if err != nil { t.Fatal(err) }
    found := false
    for _, finding := range report.Debt.New {
        if finding.Rule == RuleOperationGrowthBoundary && finding.Blocking && finding.Subject == "orders.list" { found = true }
    }
    if !found { t.Fatalf("new debt=%#v", report.Debt.New) }
    if len(auditcore.BlockingNewFindings(report)) == 0 { t.Fatal("direct growth was not blocking") }
}

func boundaryAuditProto(withGrowth bool) string {
    extra := ""
    if withGrowth {
        extra = `
  rpc List(ReadRequest) returns (ReadResponse) {
    option (yunka.dsl.v1.operation) = {
      id: "orders.list" use_case: "list_orders" public: true
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
    };
  }
`
    }
    return `syntax = "proto3";
package sales.v1;
import "yunka/dsl/v1/options.proto";
option go_package = "example.com/demo/contracts/sales;salesv1";
option (yunka.dsl.v1.domain) = { name: "sales" version: "v1" };
message ReadRequest { option (yunka.dsl.v1.dto) = { kind: DTO_INPUT }; }
message ReadResponse { option (yunka.dsl.v1.dto) = { kind: DTO_OUTPUT }; }
service Orders {
  option (yunka.dsl.v1.application) = { name: "orders" };
  rpc Read(ReadRequest) returns (ReadResponse) {
    option (yunka.dsl.v1.operation) = {
      id: "orders.read" use_case: "read_order" public: true
      execution: { transaction: TRANSACTION_READ_ONLY idempotency: IDEMPOTENCY_NONE }
      boundary: { context: "sales.orders" aggregate: "order" }
    };
  }
` + extra + "}\n"
}

var _ = strings.Contains
''')

# Add authoring regression: first operation initialization is allowed, subsequent
# growth without boundary evidence is blocked before source mutation.
write("app/cmd/add/boundary_gate_test.go", r'''package add

import (
    "path/filepath"
    "strings"
    "testing"
)

func TestOperationGrowthGateBlocksSecondOperationWithoutBoundaryIntent(t *testing.T) {
    root := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")})
    first := OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.read", UseCase: "read_tenant", Access: "public", Tenant: "optional", Transaction: "read_only", Idempotency: "none", Composition: "none", BoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant"}
    if _, err := AddOperation(first); err != nil { t.Fatal(err) }
    before := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
    second := OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.list", UseCase: "list_tenants", Access: "public", Tenant: "optional", Transaction: "read_only", Idempotency: "none", Composition: "none"}
    plan, err := PlanOperation(second)
    if err != nil { t.Fatal(err) }
    if plan.BoundaryDecision == nil || plan.BoundaryDecision.Outcome != "architecture_review_required" { t.Fatalf("decision=%#v", plan.BoundaryDecision) }
    if _, err := AddOperation(second); err == nil || !strings.Contains(err.Error(), "Operation Growth boundary") { t.Fatalf("expected boundary block, got %v", err) }
    after := readFile(t, filepath.Join(root, "contracts", "proto", "tenant.proto"))
    if before != after { t.Fatal("blocked Operation Growth mutated protobuf source") }
}
''')

# Keep generated outputs derived; the control workflow runs rpc-generate.
print("operation-growth native patch prepared")
