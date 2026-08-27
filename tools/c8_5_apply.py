from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]

def write(path, content):
    p = root / path
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(content, encoding='utf-8')

# C8.5.2-C8.5.4: unified grant decision, operation runtime, pre-application guard, authorized context.
write('gateway/authz/operation_runtime.go', r'''package authz

import (
    "context"
    "errors"
    "fmt"
    "sort"
    "strings"

    "yunka.io/framework/core/identity"
)

// Grant is an IAM-owned permission grant projected into the framework security runtime.
// Scope is opaque to the framework and is interpreted only by a domain OperationGuard.
type Grant struct {
    Permission PermissionKey
    RoleID     string
    Scope      string
}

// GrantChecker returns grants that actually authorize the requested permissions.
// Implementations MUST bind Scope to the same role/permission grant; unrelated role scope
// rows must never be returned.
type GrantChecker interface {
    ResolveGrants(context.Context, string, []string, []PermissionKey) ([]Grant, error)
}

type GrantAuthorizer struct{ checker GrantChecker }

func NewGrantAuthorizer(checker GrantChecker) (*GrantAuthorizer, error) {
    if checker == nil {
        return nil, errors.New("gateway authz: grant checker is required")
    }
    return &GrantAuthorizer{checker: checker}, nil
}

func (a *GrantAuthorizer) Authorize(ctx context.Context, principal identity.Principal, policy Policy) (Decision, error) {
    policy = Normalize(policy)
    decision := Decision{Operation: policy.Operation, Permissions: append([]PermissionKey(nil), policy.Permissions...)}
    if len(policy.Authentication) > 0 {
        if !principal.Authenticated {
            decision.Reason = ReasonUnauthenticated
            return decision, nil
        }
        if !containsString(policy.Authentication, principal.AuthMethod) {
            decision.Reason = ReasonAuthenticationMethod
            return decision, nil
        }
    }
    if len(policy.Permissions) > 0 && !principal.Authenticated {
        decision.Reason = ReasonUnauthenticated
        return decision, nil
    }
    if policy.TenantRequired && strings.TrimSpace(principal.TenantID) == "" {
        decision.Reason = ReasonTenantRequired
        return decision, nil
    }
    if len(policy.Permissions) == 0 {
        decision.Allowed, decision.Reason = true, ReasonAllowed
        return decision, nil
    }
    if strings.TrimSpace(principal.TenantID) == "" {
        decision.Reason = ReasonTenantRequired
        return decision, nil
    }
    if len(principal.Roles) == 0 {
        decision.Reason = ReasonRoleRequired
        return decision, nil
    }
    grants, err := a.checker.ResolveGrants(ctx, principal.TenantID, principal.Roles, policy.Permissions)
    if err != nil {
        return decision, fmt.Errorf("gateway authz: grant resolution: %w", err)
    }
    requested := make(map[PermissionKey]struct{}, len(policy.Permissions))
    for _, permission := range policy.Permissions {
        requested[permission] = struct{}{}
    }
    present := make(map[PermissionKey]struct{}, len(policy.Permissions))
    normalized := make([]Grant, 0, len(grants))
    for _, grant := range grants {
        grant.Permission = PermissionKey(strings.TrimSpace(string(grant.Permission)))
        grant.RoleID = strings.TrimSpace(grant.RoleID)
        grant.Scope = strings.TrimSpace(grant.Scope)
        if _, ok := requested[grant.Permission]; !ok {
            continue
        }
        present[grant.Permission] = struct{}{}
        normalized = append(normalized, grant)
    }
    sort.Slice(normalized, func(i, j int) bool {
        if normalized[i].Permission != normalized[j].Permission {
            return normalized[i].Permission < normalized[j].Permission
        }
        if normalized[i].RoleID != normalized[j].RoleID {
            return normalized[i].RoleID < normalized[j].RoleID
        }
        return normalized[i].Scope < normalized[j].Scope
    })
    decision.Grants = normalized
    if policy.Mode == PermissionAny {
        decision.Allowed = len(present) > 0
    } else {
        decision.Allowed = len(present) == len(policy.Permissions)
    }
    if decision.Allowed {
        decision.Reason = ReasonAllowed
    } else {
        decision.Reason = ReasonPermissionDenied
    }
    return decision, nil
}

// AuthorizedOperation is written by OperationRuntime immediately before the
// Application boundary. Application/domain code may inspect actor metadata for
// business/audit purposes, but MUST NOT repeat permission evaluation.
type AuthorizedOperation struct {
    Principal identity.Principal
    Policy    Policy
    Decision  Decision
}

type authorizedOperationKey struct{}

func WithAuthorizedOperation(ctx context.Context, value AuthorizedOperation) context.Context {
    return context.WithValue(ctx, authorizedOperationKey{}, value)
}

func AuthorizedOperationFromContext(ctx context.Context) (AuthorizedOperation, bool) {
    if ctx == nil {
        return AuthorizedOperation{}, false
    }
    value, ok := ctx.Value(authorizedOperationKey{}).(AuthorizedOperation)
    return value, ok
}

var (
    ErrOperationRuntimeUnavailable = errors.New("gateway authz: operation runtime unavailable")
    ErrOperationPolicyNotFound     = errors.New("gateway authz: operation policy not found")
    ErrAuthorizedOperationMissing  = errors.New("gateway authz: authorized operation missing")
)

// OperationGuard performs domain-specific resource-scope preparation after the
// IAM decision but before Application code is invoked. The framework does not
// interpret Grant.Scope.
type OperationGuard interface {
    Prepare(context.Context, AuthorizedOperation, any) (context.Context, error)
}

type GuardResolver interface {
    ResolveGuard(OperationID) (OperationGuard, bool)
}

type StaticGuardResolver map[OperationID]OperationGuard

func NewStaticGuardResolver(values map[OperationID]OperationGuard) StaticGuardResolver {
    result := make(StaticGuardResolver, len(values))
    for operation, guard := range values {
        operation = OperationID(strings.TrimSpace(string(operation)))
        if operation == "" || guard == nil {
            continue
        }
        result[operation] = guard
    }
    return result
}

func (resolver StaticGuardResolver) ResolveGuard(operation OperationID) (OperationGuard, bool) {
    guard, ok := resolver[OperationID(strings.TrimSpace(string(operation)))]
    return guard, ok
}

// OperationRuntime is the single pre-Application security boundary shared by
// REST and gRPC generated transports.
type OperationRuntime interface {
    Prepare(context.Context, string, any) (context.Context, error)
}

type operationRuntime struct {
    resolver   PolicyResolver
    authorizer Authorizer
    guards     GuardResolver
}

func NewOperationRuntime(resolver PolicyResolver, authorizer Authorizer, guards GuardResolver) (OperationRuntime, error) {
    if resolver == nil {
        return nil, errors.New("gateway authz: policy resolver is required")
    }
    if authorizer == nil {
        return nil, errors.New("gateway authz: authorizer is required")
    }
    return &operationRuntime{resolver: resolver, authorizer: authorizer, guards: guards}, nil
}

func (runtime *operationRuntime) Prepare(ctx context.Context, key string, input any) (context.Context, error) {
    if runtime == nil || runtime.resolver == nil || runtime.authorizer == nil {
        return nil, ErrOperationRuntimeUnavailable
    }
    policy, ok := runtime.resolver.ResolvePolicy(ctx, key)
    if !ok {
        return nil, fmt.Errorf("%w: %s", ErrOperationPolicyNotFound, strings.TrimSpace(key))
    }
    principal, _ := identity.FromContext(ctx)
    decision, err := runtime.authorizer.Authorize(ctx, principal, policy)
    if err != nil {
        return nil, err
    }
    if !decision.Allowed {
        return nil, Denied(decision)
    }
    authorized := AuthorizedOperation{Principal: principal, Policy: policy, Decision: decision}
    secured := WithAuthorizedOperation(ctx, authorized)
    if runtime.guards != nil {
        if guard, exists := runtime.guards.ResolveGuard(policy.Operation); exists {
            secured, err = guard.Prepare(secured, authorized, input)
            if err != nil {
                return nil, err
            }
            if secured == nil {
                return nil, errors.New("gateway authz: operation guard returned nil context")
            }
        }
    }
    return secured, nil
}

func RequireAuthorizedOperation(ctx context.Context, operation OperationID) (AuthorizedOperation, error) {
    value, ok := AuthorizedOperationFromContext(ctx)
    if !ok || value.Decision.Allowed == false {
        return AuthorizedOperation{}, ErrAuthorizedOperationMissing
    }
    if operation != "" && value.Policy.Operation != operation {
        return AuthorizedOperation{}, fmt.Errorf("%w: expected=%s actual=%s", ErrAuthorizedOperationMissing, operation, value.Policy.Operation)
    }
    return value, nil
}
''')

# Extend Decision with grant evidence.
types = root / 'gateway/authz/types.go'
s = types.read_text()
old = '''type Decision struct {\n\tAllowed     bool\n\tOperation   OperationID\n\tPermissions []PermissionKey\n\tReason      Reason\n}'''
new = '''type Decision struct {\n\tAllowed     bool\n\tOperation   OperationID\n\tPermissions []PermissionKey\n\tGrants      []Grant\n\tReason      Reason\n}'''
if old not in s:
    raise SystemExit('Decision block not found')
types.write_text(s.replace(old, new), encoding='utf-8')

write('gateway/authz/operation_runtime_test.go', r'''package authz

import (
    "context"
    "errors"
    "testing"

    "yunka.io/framework/core/identity"
)

type grantCheckerFunc func(context.Context, string, []string, []PermissionKey) ([]Grant, error)
func (f grantCheckerFunc) ResolveGrants(ctx context.Context, tenant string, roles []string, permissions []PermissionKey) ([]Grant, error) {
    return f(ctx, tenant, roles, permissions)
}

type guardFunc func(context.Context, AuthorizedOperation, any) (context.Context, error)
func (f guardFunc) Prepare(ctx context.Context, authorized AuthorizedOperation, input any) (context.Context, error) {
    return f(ctx, authorized, input)
}

type testGuardKey struct{}

func TestGrantAuthorizerBindsScopeToPermissionGrant(t *testing.T) {
    checker := grantCheckerFunc(func(_ context.Context, tenant string, roles []string, permissions []PermissionKey) ([]Grant, error) {
        if tenant != "tenant-a" || len(roles) != 2 || len(permissions) != 1 || permissions[0] != "device.read" {
            t.Fatalf("unexpected grant query tenant=%q roles=%v permissions=%v", tenant, roles, permissions)
        }
        return []Grant{{Permission: "device.read", RoleID: "reader", Scope: "self"}}, nil
    })
    authorizer, err := NewGrantAuthorizer(checker)
    if err != nil { t.Fatal(err) }
    principal := identity.Principal{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"reader", "other"}, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}
    decision, err := authorizer.Authorize(context.Background(), principal, Policy{Operation: "device.list", Permissions: []PermissionKey{"device.read"}, Mode: PermissionAll, TenantRequired: true, Authentication: []string{identity.AuthMethodAPIKey}})
    if err != nil { t.Fatal(err) }
    if !decision.Allowed || len(decision.Grants) != 1 || decision.Grants[0].RoleID != "reader" || decision.Grants[0].Scope != "self" {
        t.Fatalf("decision=%#v", decision)
    }
}

func TestOperationRuntimeRunsGuardBeforeApplicationBoundary(t *testing.T) {
    authorizer, err := NewGrantAuthorizer(grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) {
        return []Grant{{Permission: "device.read", RoleID: "reader", Scope: "sites"}}, nil
    }))
    if err != nil { t.Fatal(err) }
    resolver := NewStaticResolver(map[string]Policy{"/svc/List": {Operation: "device.list", Permissions: []PermissionKey{"device.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodAPIKey}}})
    guardCalled := false
    guards := NewStaticGuardResolver(map[OperationID]OperationGuard{"device.list": guardFunc(func(ctx context.Context, authorized AuthorizedOperation, input any) (context.Context, error) {
        guardCalled = true
        if authorized.Decision.Allowed != true || len(authorized.Decision.Grants) != 1 || input != "request" { t.Fatalf("authorized=%#v input=%v", authorized, input) }
        if _, ok := AuthorizedOperationFromContext(ctx); !ok { t.Fatal("authorized context missing inside guard") }
        return context.WithValue(ctx, testGuardKey{}, "scoped"), nil
    })})
    runtime, err := NewOperationRuntime(resolver, authorizer, guards)
    if err != nil { t.Fatal(err) }
    principal := identity.Principal{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"reader"}, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}
    ctx := identity.WithPrincipal(context.Background(), principal)
    secured, err := runtime.Prepare(ctx, "/svc/List", "request")
    if err != nil { t.Fatal(err) }
    if !guardCalled { t.Fatal("guard not called") }
    if secured.Value(testGuardKey{}) != "scoped" { t.Fatal("guard context not propagated") }
    if _, err := RequireAuthorizedOperation(secured, "device.list"); err != nil { t.Fatal(err) }
}

func TestOperationRuntimeDenialNeverRunsGuard(t *testing.T) {
    authorizer, err := NewGrantAuthorizer(grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) { return nil, nil }))
    if err != nil { t.Fatal(err) }
    resolver := NewStaticResolver(map[string]Policy{"/svc/List": {Operation: "device.list", Permissions: []PermissionKey{"device.read"}, TenantRequired: true, Authentication: []string{identity.AuthMethodAPIKey}}})
    guardCalled := false
    runtime, err := NewOperationRuntime(resolver, authorizer, NewStaticGuardResolver(map[OperationID]OperationGuard{"device.list": guardFunc(func(ctx context.Context, _ AuthorizedOperation, _ any) (context.Context, error) { guardCalled = true; return ctx, nil })}))
    if err != nil { t.Fatal(err) }
    principal := identity.Principal{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"reader"}, AuthMethod: identity.AuthMethodAPIKey, Authenticated: true}
    _, err = runtime.Prepare(identity.WithPrincipal(context.Background(), principal), "/svc/List", nil)
    if !IsDenied(err) { t.Fatalf("err=%v", err) }
    if guardCalled { t.Fatal("denied request reached guard") }
}

func TestOperationRuntimeGuardErrorStopsBoundary(t *testing.T) {
    sentinel := errors.New("scope denied")
    authorizer, _ := NewGrantAuthorizer(grantCheckerFunc(func(context.Context, string, []string, []PermissionKey) ([]Grant, error) { return []Grant{{Permission: "p"}}, nil }))
    resolver := NewStaticResolver(map[string]Policy{"/svc/Get": {Operation: "get", Permissions: []PermissionKey{"p"}}})
    runtime, _ := NewOperationRuntime(resolver, authorizer, NewStaticGuardResolver(map[OperationID]OperationGuard{"get": guardFunc(func(context.Context, AuthorizedOperation, any) (context.Context, error) { return nil, sentinel })}))
    principal := identity.Principal{TenantID: "tenant", UserID: "u", Roles: []string{"r"}, Authenticated: true}
    _, err := runtime.Prepare(identity.WithPrincipal(context.Background(), principal), "/svc/Get", nil)
    if !errors.Is(err, sentinel) { t.Fatalf("err=%v", err) }
}
''')

# C8.5.6: gRPC uses the same OperationRuntime as REST. Keep legacy helpers.
write('gateway/rpc/transport/grpc/authz.go', r'''package grpc

import (
    "context"
    "errors"

    grpcgo "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "yunka.io/gateway/authz"
)

var (
    ErrGRPCAuthorizerUnavailable     = errors.New("grpc transport authz: authorizer is required")
    ErrGRPCPolicyResolverUnavailable = errors.New("grpc transport authz: policy resolver is required")
    ErrGRPCOperationRuntimeUnavailable = errors.New("grpc transport authz: operation runtime is required")
)

func securityStatus(err error) error {
    if !authz.IsDenied(err) {
        return status.Error(codes.Internal, "authorization unavailable")
    }
    var denied *authz.DeniedError
    if errors.As(err, &denied) && (denied.Decision.Reason == authz.ReasonUnauthenticated || denied.Decision.Reason == authz.ReasonAuthenticationMethod) {
        return status.Error(codes.Unauthenticated, "authentication required")
    }
    return status.Error(codes.PermissionDenied, "permission denied")
}

// SecuredUnaryServerInterceptor is the C8.5 pre-Application security boundary.
func SecuredUnaryServerInterceptor(runtime authz.OperationRuntime) (grpcgo.UnaryServerInterceptor, error) {
    if runtime == nil {
        return nil, ErrGRPCOperationRuntimeUnavailable
    }
    return func(ctx context.Context, req interface{}, info *grpcgo.UnaryServerInfo, handler grpcgo.UnaryHandler) (interface{}, error) {
        if info == nil {
            return nil, status.Error(codes.Internal, "missing gRPC method metadata")
        }
        secured, err := runtime.Prepare(ctx, info.FullMethod, req)
        if err != nil {
            return nil, securityStatus(err)
        }
        return handler(secured, req)
    }, nil
}

// AuthorizedUnaryServerInterceptor is retained for C8.4 compatibility and is
// implemented through the C8.5 OperationRuntime without domain guards.
func AuthorizedUnaryServerInterceptor(authorizer authz.Authorizer, resolver authz.PolicyResolver) (grpcgo.UnaryServerInterceptor, error) {
    if authorizer == nil { return nil, ErrGRPCAuthorizerUnavailable }
    if resolver == nil { return nil, ErrGRPCPolicyResolverUnavailable }
    runtime, err := authz.NewOperationRuntime(resolver, authorizer, nil)
    if err != nil { return nil, err }
    return SecuredUnaryServerInterceptor(runtime)
}

func NewSecuredServer(runtime authz.OperationRuntime, options ...grpcgo.ServerOption) (*GrpcServer, error) {
    interceptor, err := SecuredUnaryServerInterceptor(runtime)
    if err != nil { return nil, err }
    opts := make([]grpcgo.ServerOption, 0, len(options)+1)
    opts = append(opts, grpcgo.ChainUnaryInterceptor(interceptor))
    opts = append(opts, options...)
    return NewTypedGrpcServer(grpcgo.NewServer(opts...))
}

func NewAuthorizedServer(authorizer authz.Authorizer, resolver authz.PolicyResolver, options ...grpcgo.ServerOption) (*GrpcServer, error) {
    interceptor, err := AuthorizedUnaryServerInterceptor(authorizer, resolver)
    if err != nil { return nil, err }
    opts := make([]grpcgo.ServerOption, 0, len(options)+1)
    opts = append(opts, grpcgo.ChainUnaryInterceptor(interceptor))
    opts = append(opts, options...)
    return NewTypedGrpcServer(grpcgo.NewServer(opts...))
}
''')

# Replace REST generator with OperationRuntime-based adapter.
p = root / 'pkg/contract/application_codegen.go'
s = p.read_text()
start = s.index('func renderRESTAdapter(')
next_func = s.index('\nfunc ', start + 10)
new_rest = r'''func renderRESTAdapter(service Service, packages []protoGoPackage, messages map[string]Message, rootImport string) (string, error) {
    imports := newImportSet()
    applicationAlias := imports.add(rootImport+"/"+service.Domain+"/application", "application")
    imports.add("yunka.io/gateway/authz", "authz")
    imports.add("google.golang.org/protobuf/encoding/protojson", "protojson")

    var handlers strings.Builder
    var registrations strings.Builder
    bindingCount := 0
    for _, method := range service.Methods {
        requestRef, err := resolveGoType(method.Request, packages, imports)
        if err != nil { return "", err }
        requestMessage := messages[method.Request]
        for index, binding := range method.HTTP {
            bindingCount++
            handlerName := "handle" + method.Name
            if len(method.HTTP) > 1 { handlerName += strconv.Itoa(index + 1) }
            pattern := strings.ToUpper(binding.Method) + " " + binding.Path
            fmt.Fprintf(&registrations, "\tmux.HandleFunc(%q, handler.%s)\n", pattern, handlerName)
            fmt.Fprintf(&handlers, "func (handler *Handler) %s(writer http.ResponseWriter, request *http.Request) {\n", handlerName)
            fmt.Fprintf(&handlers, "\twire := &%s.%s{}\n", requestRef.Alias, requestRef.Type)
            pathFields, _ := simplePathFields(binding.Path)
            if binding.Body == "*" {
                imports.add("io", "io")
                handlers.WriteString("\tbody, err := io.ReadAll(request.Body)\n\tif err != nil { http.Error(writer, \"invalid request body\", http.StatusBadRequest); return }\n\tif len(body) > 0 { if err := protojson.Unmarshal(body, wire); err != nil { http.Error(writer, \"invalid request body\", http.StatusBadRequest); return } }\n")
            } else {
                pathSet := make(map[string]struct{}, len(pathFields))
                for _, value := range pathFields { pathSet[value] = struct{}{} }
                for _, field := range requestMessage.Fields {
                    if _, pathField := pathSet[field.Name]; pathField || field.Repeated || field.Map || field.Kind == "message" || field.Kind == "enum" { continue }
                    queryExpr := "request.URL.Query().Get(" + strconv.Quote(field.Name) + ")"
                    fmt.Fprintf(&handlers, "\tif raw := %s; raw != \"\" {\n", queryExpr)
                    if scalarAssignmentNeedsStrconv(field) { imports.add("strconv", "strconv") }
                    if err := writeScalarAssignment(&handlers, "wire", field, "raw", false); err != nil { return "", err }
                    handlers.WriteString("\t}\n")
                }
            }
            for _, fieldName := range pathFields {
                field, ok := findMessageField(requestMessage, fieldName)
                if !ok { return "", fmt.Errorf("contract application codegen: %s path field %q not found in %s", method.FullName, fieldName, method.Request) }
                if scalarAssignmentNeedsStrconv(field) { imports.add("strconv", "strconv") }
                if err := writeScalarAssignment(&handlers, "wire", field, "request.PathValue("+strconv.Quote(fieldName)+")", true); err != nil { return "", fmt.Errorf("contract application codegen: %s: %w", method.FullName, err) }
            }
            fullMethod := "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name
            fmt.Fprintf(&handlers, "\tsecured, err := handler.runtime.Prepare(request.Context(), %q, wire)\n", fullMethod)
            handlers.WriteString("\tif err != nil { writeSecurityError(writer, err); return }\n")
            fmt.Fprintf(&handlers, "\toutput, err := handler.application.%s(secured, wire)\n", method.Name)
            handlers.WriteString("\tif err != nil { http.Error(writer, \"application request failed\", http.StatusBadRequest); return }\n\tpayload, err := protojson.Marshal(output)\n\tif err != nil { http.Error(writer, \"response encoding failed\", http.StatusInternalServerError); return }\n\twriter.Header().Set(\"Content-Type\", \"application/json\")\n\t_, _ = writer.Write(payload)\n}\n\n")
        }
    }
    if bindingCount == 0 { return GeneratedApplicationMarker + "\n\npackage rest\n", nil }

    var b strings.Builder
    b.WriteString(GeneratedApplicationMarker + "\n\npackage rest\n\nimport (\n\t\"errors\"\n\t\"net/http\"\n")
    b.WriteString(imports.render())
    b.WriteString(")\n\n")
    fmt.Fprintf(&b, "type Handler struct { application %s.%s; runtime authz.OperationRuntime }\n\n", applicationAlias, service.Name)
    fmt.Fprintf(&b, "func Register(mux *http.ServeMux, application %s.%s, runtime authz.OperationRuntime) error {\n", applicationAlias, service.Name)
    b.WriteString("\tif mux == nil { return errors.New(\"contract REST adapter: mux is required\") }\n")
    b.WriteString("\tif application == nil { return errors.New(\"contract REST adapter: application is required\") }\n")
    b.WriteString("\tif runtime == nil { return errors.New(\"contract REST adapter: operation runtime is required\") }\n")
    b.WriteString("\thandler := &Handler{application: application, runtime: runtime}\n")
    b.WriteString(registrations.String())
    b.WriteString("\treturn nil\n}\n\n")
    b.WriteString("func writeSecurityError(writer http.ResponseWriter, err error) {\n")
    b.WriteString("\tif !authz.IsDenied(err) { http.Error(writer, \"authorization unavailable\", http.StatusInternalServerError); return }\n")
    b.WriteString("\tstatusCode := http.StatusForbidden\n\tvar denied *authz.DeniedError\n")
    b.WriteString("\tif errors.As(err, &denied) && (denied.Decision.Reason == authz.ReasonUnauthenticated || denied.Decision.Reason == authz.ReasonAuthenticationMethod) { statusCode = http.StatusUnauthorized }\n")
    b.WriteString("\thttp.Error(writer, http.StatusText(statusCode), statusCode)\n}\n\n")
    b.WriteString(handlers.String())
    return b.String(), nil
}
'''
p.write_text(s[:start] + new_rest + s[next_func:], encoding='utf-8')

# Generated policy exposes the canonical PB permission inventory so business code never repeats literals.
p = root / 'pkg/contract/application_codegen.go'
s = p.read_text()
needle = '\tb.WriteString("\\nfunc Resolver() authz.StaticResolver {\\n\\treturn authz.NewStaticResolver(map[string]authz.Policy{\\n")\n'
if needle not in s:
    raise SystemExit('renderOperationPolicy resolver insertion point not found')
insert = '''\tpermissionSet := map[string]struct{}{}\n\tfor _, method := range service.Methods {\n\t\tfor _, permission := range method.Operation.Permissions {\n\t\t\tif permission = strings.TrimSpace(permission); permission != "" { permissionSet[permission] = struct{}{} }\n\t\t}\n\t}\n\tpermissionValues := make([]string, 0, len(permissionSet))\n\tfor permission := range permissionSet { permissionValues = append(permissionValues, permission) }\n\tsort.Strings(permissionValues)\n\tb.WriteString("\\nfunc Permissions() []authz.PermissionKey {\\n\\treturn []authz.PermissionKey{")\n\tfor _, permission := range permissionValues { fmt.Fprintf(&b, "%q,", permission) }\n\tb.WriteString("}\\n}\\n")\n'''
s = s.replace(needle, insert + needle, 1)
p.write_text(s, encoding='utf-8')

# Update C8.4 runtime fixture to assert shared C8.5 OperationRuntime path.
p = root / 'pkg/contract/application_runtime_test.go'
s = p.read_text()
s = s.replace('authInterceptor, err := authzgrpc.AuthorizedUnaryServerInterceptor(authorizer, policy.Resolver())\n    if err != nil { t.Fatal(err) }', 'securityRuntime, err := authz.NewOperationRuntime(policy.Resolver(), authorizer, nil)\n    if err != nil { t.Fatal(err) }\n    authInterceptor, err := authzgrpc.SecuredUnaryServerInterceptor(securityRuntime)\n    if err != nil { t.Fatal(err) }')
s = s.replace('if err := rest.Register(mux, application, authorizer); err != nil { t.Fatal(err) }', 'if err := rest.Register(mux, application, securityRuntime); err != nil { t.Fatal(err) }')
p.write_text(s, encoding='utf-8')

write('docs/waves/C8.5-operation-security-boundary.md', r'''# C8.5 — Operation Security Boundary

State: Implemented on feature branch; merge requires standard CI and production verification.

## Invariant

```text
PB owns Operation/Auth/Tenant/Permission declarations.
Access/IAM owns identity, role, credential, PermissionGrant and ScopeGrant business data.
Yunka owns the pre-Application security execution pipeline.
Business domains own interpretation of opaque scope grants.
Application/Domain code never repeats permission evaluation.
```

## C8.5.1 Access/IAM Domain Extraction

Implemented in the reference `biz` repository after this framework wave: IAM remains a business/platform domain and does not move into Yunka framework persistence.

## C8.5.2 Unified Authorization Decision

`authz.GrantAuthorizer` resolves permission + role + opaque scope as one grant decision. Scope cannot be sourced from a role that did not grant the requested permission.

## C8.5.3 Pre-Application Operation Guard

`authz.OperationRuntime` resolves PB policy, authorizes, runs an optional domain `OperationGuard`, then and only then returns the context passed to Application.

## C8.5.4 Authorized Scope Context

The framework writes `AuthorizedOperation` to context. A domain guard converts decision grants into a typed domain scope context; the framework never interprets resource fields such as site/customer/device IDs.

## C8.5.5 Remove Authorization From Application

Generated policy now exposes canonical `Permissions()` from PB. Reference biz Application code must contain no permission literals, `Authorize`, `HasPermissions` or permission-driven scope resolver calls.

## C8.5.6 REST/gRPC Same Security Pipeline

Generated REST adapters call `OperationRuntime.Prepare` after request decoding and before Application. gRPC `SecuredUnaryServerInterceptor` calls the exact same runtime. The legacy C8.4 interceptor remains as a compatibility wrapper.
''')
