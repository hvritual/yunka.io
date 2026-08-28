from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content)


def edit(path: str, old: str, new: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if old not in text:
        raise SystemExit(f"expected fragment not found in {path}: {old[:180]!r}")
    target.write_text(text.replace(old, new, 1))


def append(path: str, content: str) -> None:
    target = ROOT / path
    text = target.read_text()
    if content.strip() not in text:
        target.write_text(text.rstrip() + "\n\n" + content.lstrip())


# C9.7.6: replace raw C8 application capability exposure with typed child
# Operation wrappers backed by the shared Executor.
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\tmessages := messageIndex(manifest)\n\ttypedByDomain := make(map[string][]Service)\n\tfor _, service := range manifest.Services {\n\t\tif service.Application != nil {\n\t\t\ttypedByDomain[service.Domain] = append(typedByDomain[service.Domain], service)\n\t\t}\n\t}\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tmessages := messageIndex(manifest)\n\ttypedByDomain := make(map[string][]Service)\n\tserviceByApplication := make(map[string]Service)\n\tfor _, service := range manifest.Services {\n\t\tif service.Application != nil {\n\t\t\ttypedByDomain[service.Domain] = append(typedByDomain[service.Domain], service)\n\t\t\tserviceByApplication[service.Domain+"/"+service.Application.Name] = service\n\t\t}\n\t}\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\t\t\t}\n\t\t\tfor _, artifact := range generated {\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\t\t}\n\t\t\tif len(service.Application.Requires) > 0 {\n\t\t\t\tgenerated = append(generated, struct {\n\t\t\t\t\tpath   string\n\t\t\t\t\trender func() (string, error)\n\t\t\t\t}{\n\t\t\t\t\tfilepath.ToSlash(filepath.Join(domain, "application", "zz_yunka_"+naming.FileStem+"_capability_ports_gen.go")),\n\t\t\t\t\tfunc() (string, error) {\n\t\t\t\t\t\treturn renderC9CapabilityPorts(service, naming, serviceByApplication, typedByDomain, packages, rootImport)\n\t\t\t\t\t},\n\t\t\t\t})\n\t\t\t}\n\t\t\tfor _, artifact := range generated {\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\t\tif strings.Contains(path, "/transport/rest/") || strings.Contains(path, "/transport/rpc/") {\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\tif strings.Contains(path, "/transport/rest/") || strings.Contains(path, "/transport/rpc/") || strings.HasSuffix(path, "_capability_ports_gen.go") {\n'''.replace('\\t','\t').replace('\\n','\n'),
)

marker = "func renderC9OperationPlans(service Service, naming serviceCodegenNaming, plans map[string]operationplan.Plan) (string, error) {\n"
helper = r'''func renderC9CapabilityPorts(service Service, naming serviceCodegenNaming, services map[string]Service, typedByDomain map[string][]Service, packages []protoGoPackage, rootImport string) (string, error) {
	imports := newImportSet()
	imports.add("context", "context")
	imports.add("errors", "errors")
	imports.add("yunka.io/framework/operation", "operation")
	var declarations strings.Builder
	var providerMethods strings.Builder
	seen := map[string]string{}
	for _, dependency := range stableStrings(service.Application.Requires) {
		target, ok := services[dependency]
		if !ok || target.Application == nil {
			return "", fmt.Errorf("contract C9 application codegen: unknown capability dependency %s for %s", dependency, service.FullName)
		}
		targetNaming := namingForService(target, len(typedByDomain[target.Domain]) > 1)
		dependencySymbol := exportedApplicationSymbol(strings.ReplaceAll(dependency, "/", "_"))
		if owner, duplicate := seen[dependencySymbol]; duplicate && owner != dependency {
			return "", fmt.Errorf("contract C9 application codegen: capability dependencies %s and %s collapse to symbol %s", owner, dependency, dependencySymbol)
		}
		seen[dependencySymbol] = dependency
		interfaceName := dependencySymbol + "ChildCapability"
		implementationName := lowerFirstIdentifier(dependencySymbol) + "ChildCapability"
		constructorName := "New" + dependencySymbol + "ChildCapability"

		targetApplicationType := targetNaming.ApplicationInterface
		if target.Domain != service.Domain {
			alias := imports.add(rootImport+"/"+target.Domain+"/application", safeFileName(target.Domain)+"application")
			targetApplicationType = alias + "." + targetNaming.ApplicationInterface
		}
		policyAlias := imports.add(rootImport+"/"+target.Domain+"/policy", safeFileName(target.Domain)+"policy")

		var interfaceMethods strings.Builder
		var wrapperMethods strings.Builder
		for _, method := range target.Methods {
			request, err := resolveGoType(method.Request, packages, imports)
			if err != nil {
				return "", err
			}
			response, err := resolveGoType(method.Response, packages, imports)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&interfaceMethods, "\t%s(context.Context, *%s.%s) (*%s.%s, error)\n", method.Name, request.Alias, request.Type, response.Alias, response.Type)
			fmt.Fprintf(&wrapperMethods, "func (capability *%s) %s(ctx context.Context, request *%s.%s) (*%s.%s, error) {\n", implementationName, method.Name, request.Alias, request.Type, response.Alias, response.Type)
			fmt.Fprintf(&wrapperMethods, "\treturn operation.ExecuteChildTyped(ctx, capability.executor, %s.%s(), request, capability.application.%s)\n}\n\n", policyAlias, c9PlanFunction(targetNaming, method), method.Name)
		}
		fmt.Fprintf(&declarations, "type %s interface {\n%s}\n\n", interfaceName, interfaceMethods.String())
		fmt.Fprintf(&declarations, "type %s struct { application %s; executor operation.Executor }\n\n", implementationName, targetApplicationType)
		fmt.Fprintf(&declarations, "func %s(application %s, executor operation.Executor) (%s, error) {\n", constructorName, targetApplicationType, interfaceName)
		fmt.Fprintf(&declarations, "\tif application == nil { return nil, errors.New(%q) }\n", "contract C9 child capability: target application is required")
		fmt.Fprintf(&declarations, "\tif executor == nil { return nil, errors.New(%q) }\n", "contract C9 child capability: operation executor is required")
		fmt.Fprintf(&declarations, "\treturn &%s{application: application, executor: executor}, nil\n}\n\n", implementationName)
		declarations.WriteString(wrapperMethods.String())
		fmt.Fprintf(&providerMethods, "\t%s() %s\n", dependencySymbol, interfaceName)
	}
	var b strings.Builder
	b.WriteString(GeneratedApplicationMarker + "\n\npackage application\n\nimport (\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	b.WriteString(declarations.String())
	fmt.Fprintf(&b, "// %sCapabilities exposes only C9 child-Operation wrappers for declared application dependencies.\n", naming.Symbol)
	fmt.Fprintf(&b, "type %sCapabilities interface {\n%s}\n", naming.Symbol, providerMethods.String())
	return b.String(), nil
}

'''
edit("pkg/contract/c9_application_codegen.go", marker, helper + marker)

# Establish transport idempotency metadata without deciding policy in transport.
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\timports.add("google.golang.org/grpc", "grpc")\n\timports.add("yunka.io/framework/operation", "operation")\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\timports.add("google.golang.org/grpc", "grpc")\n\timports.add("google.golang.org/grpc/metadata", "grpcmetadata")\n\timports.add("yunka.io/framework/execution", "execution")\n\timports.add("yunka.io/framework/operation", "operation")\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\t\tfmt.Fprintf(&methods, "\\tresponse, err := operation.ExecuteTyped(ctx, server.executor, %s.%s(), request, server.application.%s)\\n", policyAlias, c9PlanFunction(naming, method), method.Name)\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\tmethods.WriteString("\\tif metadata, ok := grpcmetadata.FromIncomingContext(ctx); ok { if values := metadata.Get(\\\"idempotency-key\\\"); len(values) > 0 { ctx = execution.WithIdempotencyKey(ctx, values[0]) } }\\n")\n\t\tfmt.Fprintf(&methods, "\\tresponse, err := operation.ExecuteTyped(ctx, server.executor, %s.%s(), request, server.application.%s)\\n", policyAlias, c9PlanFunction(naming, method), method.Name)\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\timports.add("yunka.io/framework/operation", "operation")\n\timports.add("yunka.io/gateway/authz", "authz")\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\timports.add("yunka.io/framework/execution", "execution")\n\timports.add("yunka.io/framework/operation", "operation")\n\timports.add("yunka.io/gateway/authz", "authz")\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\t\t\tfmt.Fprintf(&handlers, "\\toutput, err := operation.ExecuteTyped(request.Context(), handler.executor, %s.%s(), wire, handler.application.%s)\\n", policyAlias, c9PlanFunction(naming, method), method.Name)\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\t\thandlers.WriteString("\\tcallContext := execution.WithIdempotencyKey(request.Context(), request.Header.Get(\\\"Idempotency-Key\\\"))\\n")\n\t\t\tfmt.Fprintf(&handlers, "\\toutput, err := operation.ExecuteTyped(callContext, handler.executor, %s.%s(), wire, handler.application.%s)\\n", policyAlias, c9PlanFunction(naming, method), method.Name)\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/contract/c9_application_codegen.go",
    '''\tb.WriteString("\\tif errors.Is(err, operation.ErrExecutorUnavailable) || errors.Is(err, operation.ErrSecurityUnavailable) || errors.Is(err, operation.ErrSecurityNilContext) { http.Error(writer, \\\"operation execution unavailable\\\", http.StatusInternalServerError); return }\\n")\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tb.WriteString("\\tif errors.Is(err, execution.ErrIdempotencyKeyRequired) { http.Error(writer, \\\"idempotency key required\\\", http.StatusBadRequest); return }\\n")\n\tb.WriteString("\\tif errors.Is(err, execution.ErrIdempotencyInProgress) || errors.Is(err, execution.ErrIdempotencyCompleted) { http.Error(writer, \\\"idempotency conflict\\\", http.StatusConflict); return }\\n")\n\tb.WriteString("\\tif errors.Is(err, operation.ErrExecutorUnavailable) || errors.Is(err, operation.ErrSecurityUnavailable) || errors.Is(err, operation.ErrSecurityNilContext) || errors.Is(err, operation.ErrIdempotencyUnavailable) { http.Error(writer, \\\"operation execution unavailable\\\", http.StatusInternalServerError); return }\\n")\n'''.replace('\\t','\t').replace('\\n','\n'),
)

# C9.7.7: Saga/Outbox joins current ExecutionScope via a typed Stager.
write("framework/workflow/saga/stage.go", r'''package saga

import (
	"context"
	"errors"

	"yunka.io/framework/event/outbox"
	"yunka.io/framework/execution"
)

type Stager interface {
	Stage(context.Context, Plan) error
	StageCompensations(context.Context, Plan, int) error
}

type stager struct{ store outbox.TransactionalStore }

func NewStager(store outbox.TransactionalStore) (Stager, error) {
	if store == nil {
		return nil, errors.New("saga: transactional outbox store is required")
	}
	return &stager{store: store}, nil
}

func (stager *stager) Stage(ctx context.Context, plan Plan) error {
	if stager == nil || stager.store == nil {
		return errors.New("saga: stager unavailable")
	}
	transaction, err := execution.TransactionHandleFrom(ctx)
	if err != nil {
		return err
	}
	return EnqueueTx(ctx, stager.store, transaction, plan)
}

func (stager *stager) StageCompensations(ctx context.Context, plan Plan, completed int) error {
	if stager == nil || stager.store == nil {
		return errors.New("saga: stager unavailable")
	}
	transaction, err := execution.TransactionHandleFrom(ctx)
	if err != nil {
		return err
	}
	return EnqueueCompensationsTx(ctx, stager.store, transaction, plan, completed)
}
''')
write("framework/workflow/saga/stage_test.go", r'''package saga

import (
	"context"
	"encoding/json"
	"testing"

	"yunka.io/framework/event"
	"yunka.io/framework/execution"
)

type stageUnit struct{ handle any }
func (*stageUnit) Commit(context.Context) error { return nil }
func (*stageUnit) Rollback(context.Context) error { return nil }
func (*stageUnit) Close() error { return nil }
func (unit *stageUnit) TransactionHandle() any { return unit.handle }

type stageFactory struct{ unit *stageUnit }
func (factory stageFactory) Begin(context.Context, execution.TransactionMode) (execution.UnitOfWork, error) { return factory.unit, nil }

type stageStore struct{ handles []any; events []event.Envelope }
func (store *stageStore) EnqueueTx(_ context.Context, tx any, envelope event.Envelope) error {
	store.handles = append(store.handles, tx)
	store.events = append(store.events, envelope)
	return nil
}

func TestStagerUsesExactExecutionScopeTransaction(t *testing.T) {
	handle := &struct{ id string }{"tx-1"}
	ctx, root, err := execution.BeginRoot(context.Background(), "device.provision", execution.TransactionLocal, nil, stageFactory{unit: &stageUnit{handle: handle}})
	if err != nil { t.Fatal(err) }
	defer root.Rollback(ctx)
	store := &stageStore{}
	stager, err := NewStager(store)
	if err != nil { t.Fatal(err) }
	plan := Plan{ID: "saga-1", IdempotencyKey: "request-1", Steps: []Step{{ID: "reserve", Command: Command{Topic: "inventory", Type: "reserve", Payload: json.RawMessage(`{"id":"d1"}`)}}}}
	if err := stager.Stage(ctx, plan); err != nil { t.Fatal(err) }
	if len(store.handles) != 1 || store.handles[0] != handle || len(store.events) != 1 {
		t.Fatalf("handles=%v events=%v", store.handles, store.events)
	}
}

func TestStagerFailsOutsideTransactionalExecutionScope(t *testing.T) {
	stager, err := NewStager(&stageStore{})
	if err != nil { t.Fatal(err) }
	plan := Plan{ID: "saga-1", IdempotencyKey: "request-1", Steps: []Step{{ID: "x", Command: Command{Topic: "t", Type: "x"}}}}
	if err := stager.Stage(context.Background(), plan); err == nil { t.Fatal("expected missing execution scope failure") }
}
''')

# Make failed GORM commit rollback-eligible, matching the stated requestscope invariant.
edit(
    "framework/requestscope/gorm.go",
    '''\tresult := unit.transaction.WithContext(normalizeContext(ctx)).Commit()\n\tunit.finished = true\n\tunit.finishErr = result.Error\n\treturn unit.finishErr\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tresult := unit.transaction.WithContext(normalizeContext(ctx)).Commit()\n\tunit.finishErr = result.Error\n\tif result.Error == nil {\n\t\tunit.finished = true\n\t}\n\treturn unit.finishErr\n'''.replace('\\t','\t').replace('\\n','\n'),
)

# C9.7.8: normalize idempotency transport errors in gRPC.
edit(
    "gateway/rpc/transport/grpc/operation_error.go",
    '''\t"google.golang.org/grpc/status"\n\t"yunka.io/framework/operation"\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t"google.golang.org/grpc/status"\n\t"yunka.io/framework/execution"\n\t"yunka.io/framework/operation"\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "gateway/rpc/transport/grpc/operation_error.go",
    '''\tif errors.Is(err, operation.ErrExecutorUnavailable) ||\n\t\terrors.Is(err, operation.ErrSecurityUnavailable) ||\n\t\terrors.Is(err, operation.ErrSecurityNilContext) {\n\t\treturn status.Error(codes.Internal, "operation execution unavailable")\n\t}\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tif errors.Is(err, execution.ErrIdempotencyKeyRequired) {\n\t\treturn status.Error(codes.InvalidArgument, "idempotency key required")\n\t}\n\tif errors.Is(err, execution.ErrIdempotencyInProgress) {\n\t\treturn status.Error(codes.Aborted, "idempotent operation in progress")\n\t}\n\tif errors.Is(err, execution.ErrIdempotencyCompleted) {\n\t\treturn status.Error(codes.AlreadyExists, "idempotent operation already completed")\n\t}\n\tif errors.Is(err, operation.ErrExecutorUnavailable) ||\n\t\terrors.Is(err, operation.ErrSecurityUnavailable) ||\n\t\terrors.Is(err, operation.ErrSecurityNilContext) ||\n\t\terrors.Is(err, operation.ErrIdempotencyUnavailable) {\n\t\treturn status.Error(codes.Internal, "operation execution unavailable")\n\t}\n'''.replace('\\t','\t').replace('\\n','\n'),
)

# C9.7.10: safe execution policy evidence in graph and diagnostics.
edit(
    "pkg/applicationgraph/operationplan.go",
    '''\t\t\t"permissionMode":      plan.Security.PermissionMode,\n\t\t\t"operationPlanSchema":  strconv.Itoa(set.SchemaVersion),\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\t\t"permissionMode":      plan.Security.PermissionMode,\n\t\t\t"transaction":         plan.Execution.Transaction,\n\t\t\t"idempotency":         plan.Execution.Idempotency,\n\t\t\t"operationPlanSchema":  strconv.Itoa(set.SchemaVersion),\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/applicationgraph/operationplan_test.go",
    '''\tif operation.ID == "" || operation.Attributes["operationPlanSchema"] != "1" || operation.Attributes["operationPlanDigest"] == "" {\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tif operation.ID == "" || operation.Attributes["operationPlanSchema"] != strconv.Itoa(operationplan.SchemaVersion) || operation.Attributes["operationPlanDigest"] == "" {\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/applicationgraph/operationplan_test.go",
    '''import (\n\t"testing"\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''import (\n\t"strconv"\n\t"testing"\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/applicationgraph/operationplan_test.go",
    '''\t\tSecurity: operationplan.Security{TenantRequired: true, Permissions: []string{"device.read"}, PermissionMode: "all"},\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\tSecurity: operationplan.Security{TenantRequired: true, Permissions: []string{"device.read"}, PermissionMode: "all"},\n\t\tExecution: operationplan.Execution{Transaction: "read_only", Idempotency: "none"},\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "pkg/applicationgraph/operationplan_test.go",
    '''\tif operation.ID == "" || operation.Attributes["operationPlanSchema"] != strconv.Itoa(operationplan.SchemaVersion) || operation.Attributes["operationPlanDigest"] == "" {\n\t\tt.Fatalf("operation=%+v", operation)\n\t}\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tif operation.ID == "" || operation.Attributes["operationPlanSchema"] != strconv.Itoa(operationplan.SchemaVersion) || operation.Attributes["operationPlanDigest"] == "" {\n\t\tt.Fatalf("operation=%+v", operation)\n\t}\n\tif operation.Attributes["transaction"] != "read_only" || operation.Attributes["idempotency"] != "none" {\n\t\tt.Fatalf("execution evidence=%+v", operation.Attributes)\n\t}\n'''.replace('\\t','\t').replace('\\n','\n'),
)

edit(
    "framework/diagnostics/operation.go",
    '''\tComposition string `json:"composition,omitempty"`\n\tProtected   bool   `json:"protected"`\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tComposition string `json:"composition,omitempty"`\n\tTransaction string `json:"transaction"`\n\tIdempotency string `json:"idempotency"`\n\tProtected   bool   `json:"protected"`\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "framework/diagnostics/operation.go",
    '''\t\t\tComposition: plan.Composition.Boundary,\n\t\t\tProtected:   frameworkoperation.Protected(plan),\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\t\tComposition: plan.Composition.Boundary,\n\t\t\tTransaction: plan.Execution.Transaction,\n\t\t\tIdempotency: plan.Execution.Idempotency,\n\t\t\tProtected:   frameworkoperation.Protected(plan),\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "framework/diagnostics/operation_test.go",
    '''\t\tSecurity: operationplan.Security{TenantRequired: true, Authentication: []string{"jwt"}, Permissions: []string{"device.secret.read"}, PermissionMode: "all"},\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\t\tSecurity: operationplan.Security{TenantRequired: true, Authentication: []string{"jwt"}, Permissions: []string{"device.secret.read"}, PermissionMode: "all"},\n\t\tExecution: operationplan.Execution{Transaction: "read_only", Idempotency: "none"},\n'''.replace('\\t','\t').replace('\\n','\n'),
)
edit(
    "framework/diagnostics/operation_test.go",
    '''\tif !strings.Contains(text, `"operationId":"device.get"`) || !strings.Contains(text, `"digest":"`) {\n'''.replace('\\t','\t').replace('\\n','\n'),
    '''\tif !strings.Contains(text, `"operationId":"device.get"`) || !strings.Contains(text, `"digest":"`) || !strings.Contains(text, `"transaction":"read_only"`) {\n'''.replace('\\t','\t').replace('\\n','\n'),
)

# Codegen regression tests for child wrapper and transport idempotency context.
append("pkg/contract/c9_application_codegen_test.go", r'''
func TestRenderC9ApplicationCodeGeneratesChildOperationCapability(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{
			{Name: "site.proto", Package: "site.v1", GoPackage: "example.com/biz/contracts/site/v1;sitev1", Domain: &DomainDeclaration{Name: "site", Version: "v1"}},
			{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}},
		},
		Messages: []Message{
			{Name: "ValidateRequest", FullName: "site.v1.ValidateRequest"}, {Name: "ValidateResponse", FullName: "site.v1.ValidateResponse"},
			{Name: "TransferRequest", FullName: "device.v1.TransferRequest"}, {Name: "TransferResponse", FullName: "device.v1.TransferResponse"},
		},
		Services: []Service{
			{Name: "SiteApplication", FullName: "site.v1.SiteApplication", Domain: "site", Application: &ApplicationDeclaration{Name: "validation"}, Methods: []Method{{Name: "Validate", FullName: "site.v1.SiteApplication.Validate", Request: "site.v1.ValidateRequest", Response: "site.v1.ValidateResponse", Operation: &OperationDeclaration{ID: "site.validate", UseCase: "validate_site", Permissions: []string{"site.read"}, PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}}}}},
			{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "transfer", Requires: []string{"site/validation"}}, Methods: []Method{{Name: "Transfer", FullName: "device.v1.DeviceApplication.Transfer", Request: "device.v1.TransferRequest", Response: "device.v1.TransferResponse", Operation: &OperationDeclaration{ID: "device.transfer", UseCase: "transfer_device", Permissions: []string{"device.update", "site.read"}, PermissionMode: "all", RequiresOperations: []string{"site.validate"}, Composition: "local", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}}},
		},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil { t.Fatal(err) }
	byPath := map[string]string{}
	for _, file := range files { byPath[file.Path] = string(file.Content) }
	capability := byPath["device/application/zz_yunka_transfer_capability_ports_gen.go"]
	if !strings.Contains(capability, "operation.ExecuteChildTyped") || !strings.Contains(capability, "sitepolicy.OperationPlanValidate()") || !strings.Contains(capability, "NewSiteValidationChildCapability") {
		t.Fatalf("C9 child capability is not executor-backed:\n%s", capability)
	}
	if strings.Contains(capability, "Resolve(") || strings.Contains(capability, "map[string]any") {
		t.Fatalf("service locator leaked into child capability:\n%s", capability)
	}
}

func TestRenderC9TransportsEstablishIdempotencyContext(t *testing.T) {
	manifest := Manifest{SchemaVersion: ManifestVersion,
		Files: []File{{Name: "device.proto", Package: "device.v1", GoPackage: "example.com/biz/contracts/device/v1;devicev1", Domain: &DomainDeclaration{Name: "device", Version: "v1"}}},
		Messages: []Message{{Name: "CreateRequest", FullName: "device.v1.CreateRequest"}, {Name: "CreateResponse", FullName: "device.v1.CreateResponse"}},
		Services: []Service{{Name: "DeviceApplication", FullName: "device.v1.DeviceApplication", Domain: "device", Application: &ApplicationDeclaration{Name: "management"}, Methods: []Method{{Name: "Create", FullName: "device.v1.DeviceApplication.Create", Request: "device.v1.CreateRequest", Response: "device.v1.CreateResponse", HTTP: []HTTPBinding{{Method: "POST", Path: "/v1/devices", Body: "*"}}, Operation: &OperationDeclaration{ID: "device.create", UseCase: "create_device", PermissionMode: "all", Execution: &ExecutionPolicy{Transaction: "local", Idempotency: "required"}}}}}},
	}
	files, err := RenderC9ApplicationCode(manifest, ApplicationCodeOptions{RootImport: "example.com/biz/internal"})
	if err != nil { t.Fatal(err) }
	byPath := map[string]string{}
	for _, file := range files { byPath[file.Path] = string(file.Content) }
	rest := byPath["device/transport/rest/zz_yunka_management_operation_executor_gen.go"]
	rpc := byPath["device/transport/rpc/zz_yunka_management_operation_executor_gen.go"]
	if !strings.Contains(rest, `request.Header.Get("Idempotency-Key")`) || !strings.Contains(rest, "execution.WithIdempotencyKey") {
		t.Fatalf("REST idempotency context missing:\n%s", rest)
	}
	if !strings.Contains(rpc, `metadata.Get("idempotency-key")`) || !strings.Contains(rpc, "execution.WithIdempotencyKey") {
		t.Fatalf("gRPC idempotency context missing:\n%s", rpc)
	}
}
''')

print("C9.7.6-10 composition/runtime convergence staged")
