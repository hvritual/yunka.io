# Typed Use-Case Policy Authoring

Yunka keeps transport, authorization plumbing, transaction scope, repository filtering, and protocol error mapping inside the framework. Application developers supply only the policy differences and business rules that are specific to a bounded context.

## Generated contract

For every PO object, `yunka domain generate` creates one typed use-case contract and two extension slots:

```go
type DeviceUseCases interface {
    CreateDevice(context.Context, CreateDeviceInput) (DeviceOutput, error)
    GetDevice(context.Context, GetDeviceInput) (DeviceOutput, error)
    ListDevices(context.Context, ListDevicesInput) (ListDevicesOutput, error)
    UpdateDevice(context.Context, UpdateDeviceInput) (DeviceOutput, error)
    DeleteDevice(context.Context, DeleteDeviceInput) error
}

type DevicePolicy interface {
    AuthorizeCreate(context.Context, identity.Principal, CreateDeviceInput) error
    ListScope(context.Context, identity.Principal, ListDevicesInput) (policy.Filter, error)
    AuthorizeGet(context.Context, identity.Principal, domain.Device) error
    AuthorizeUpdate(context.Context, identity.Principal, domain.Device, UpdateDeviceInput) error
    AuthorizeDelete(context.Context, identity.Principal, domain.Device) error
}

type DeviceRules interface {
    ValidateCreate(context.Context, CreateDeviceInput) error
    ValidateUpdate(context.Context, domain.Device, UpdateDeviceInput) error
    ValidateDelete(context.Context, domain.Device) error
}
```

REST and gRPC both delegate to this same application service. Authorization must not be duplicated in transport adapters.

## Standard RBAC/DataScope declaration

Objects with conventional `site_id` and `created_by`/`owner_id` fields receive a standard access declaration. For the common case, developers only name permissions:

```go
accessPolicy := application.StandardDeviceAccess(
    policy.ContextResolver{},
    application.DevicePermissions{
        Create: "device.create",
        Read:   "device.read",
        Update: "device.update",
        Delete: "device.delete",
    },
)

bundle, err := wire.Build(database,
    application.WithDevicePolicy(accessPolicy),
)
```

The generated standard policy implements the existing Yunka data-scope semantics:

- `All`: all rows inside the trusted tenant.
- `Sites`: rows whose `site_id` is one of the grant's site IDs.
- `Self`: rows whose `created_by` or `owner_id` equals the trusted principal user ID.
- `Sites + Self`: the union of those two scopes.

Create with a site-scoped permission requires the requested site to be granted. Update first authorizes the current object; when the target site changes, the new site must independently be permitted. A self-only grant does not allow moving an object to an arbitrary site.

`created_by` and `owner_id` are trusted ownership conventions when used by the standard `Self` scope. On create, the generated application service derives that field from `identity.Principal.UserID`; it does not trust the transport payload. On update, the generated service preserves the existing ownership value instead of allowing reassignment through the generic CRUD input. Applications that need ownership transfer should model it as an explicit use case with its own policy and business rules.

The persistence layer receives a `policy.Filter` from the application service and applies it after trusted-tenant scoping. If a policy asks for a scope dimension that the object cannot represent, the generated repository fails closed instead of broadening access.

## Authentication middleware contract

Authentication remains an application concern because token/session verification differs between products. The middleware must place both the trusted principal and resolved grants into the request context:

```go
func Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        principal, grants, err := authenticate(r.Context(), r)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        ctx := identity.WithPrincipal(r.Context(), principal)
        ctx = policy.WithGrants(ctx, grants)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

Generated REST registration accepts typed middleware:

```go
bundle.RegisterREST(mux, Authenticate)
```

The same principal/grant context can be established by gRPC interceptors before the generated bridge invokes the application service.

## Complex 20% policy

Do not expand `domain.json` into a business-rule DSL. When a policy depends on domain state, cross-object relationships, time windows, field-level decisions, or external facts, implement the generated Go interface directly:

```go
type DevicePolicy struct {
    access AccessReader
}

func (p DevicePolicy) AuthorizeUpdate(
    ctx context.Context,
    principal identity.Principal,
    current domain.Device,
    input application.UpdateDeviceInput,
) error {
    // Product-specific typed Go policy.
    return nil
}
```

Inject it with `application.WithDevicePolicy(...)`. The REST/gRPC/repository pipeline does not change.

## Business rules are separate

Authorization answers **who may perform the use case against which data**. Business rules answer **whether the business transition itself is valid**. Keep those concerns separate by implementing the generated `DeviceRules` slot and injecting it with `application.WithDeviceRules(...)`.

This keeps transport code generated, application orchestration deterministic, and product-specific logic small, typed, testable, and independent of HTTP or gRPC.
