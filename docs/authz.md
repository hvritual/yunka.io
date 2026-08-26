# Gateway authorization

`gateway/authz` is the canonical authorization boundary. Authentication produces a trusted `identity.Principal`; authorization evaluates a stable operation policy against tenant-scoped role permissions. UI buttons are never authorization principals or grants.

## PB declaration

Protobuf methods are the contract source of truth. Until typed protobuf options are introduced, the compiler consumes structured `@yunka.*` method directives and normalizes them into `manifest.json.authorization`:

```proto
// @yunka.operation device.machine.get
// @yunka.authentication jwt
// @yunka.permission device.machine.read
// @yunka.permission_mode all
// @yunka.tenant_required true
rpc GetMachine(GetMachineRequest) returns (MachineDTO);
```

`operation` and permission keys are stable business identities. They must not be derived from HTTP paths, API UUIDs, button UUIDs, or database primary keys.

## C8.3 ownership model

The authorization graph is deliberately small:

```text
PB Method / Operation ──requires──> Permission <──granted── Tenant Role
                                      ^
                                      │ represents
                                  UI Button
```

- PB/API Operation references Permission through `RuntimeAuthorization.permissions`.
- UI Button references Permission through `RuntimeApiModuleButton.permissions` only for visibility/navigation and compatibility translation.
- Tenant Role owns Permission grants in `role_permission`.
- Button metadata is stored in `button_permission`.
- Gateway `Authorizer` is the only backend enforcement point.
- Resource/data scope remains a separate evaluator seam; PB must not contain SQL predicates.

There is intentionally no Role -> Button grant and no API -> Button authorization relationship.

## Compatibility migration

C8.3 preserves wire compatibility without preserving the old authorization model:

1. `RoleModuleBtn.moduleBtnUUID` and `deleteModuleBtnUUID` keep their historical field numbers but are deprecated.
2. New clients use `permissions` and `deletePermissions` directly.
3. Old Button-based requests are translated at the Gateway control-plane boundary by resolving Button -> Permission and then mutating Role -> Permission.
4. Existing `role_module_button` rows are read only for idempotent backfill after Button -> Permission metadata becomes available.
5. C8.3 does not AutoMigrate, write, or authorize through `role_module_button` or `api_module_button`.
6. Deleting an API never revokes role permissions.

This is an expand-and-cutover migration: stored legacy rows may remain until a later data-retention wave, but they are no longer part of the live authorization path.

## Execution boundary

Typed authorization is enforced immediately before the business executor. `NewAuthorizedHandleMiddleware` wraps the configured Gateway executor with `bridge.AuthorizedExecutor`, so every composite child operation is authorized independently. Direct gRPC servers use `grpc.NewAuthorizedServer` (or `AuthorizedUnaryServerInterceptor`) with a `gateway/authz.PolicyResolver`; HTTP and gRPC therefore share the same `Authorizer` contract and `Principal` semantics. Protected typed operations fail closed if a Gateway handle is constructed without an Authorizer.

The legacy `EnterpriseRoleMiddleware` is retained only as a source-compatible pass-through chain element. It must not grant or deny access. `intercept.Intercept` similarly exposes only the typed `GatewayServiceServer`; authorization is not a business service method.

## Framework composition

`role.NewAuthorizerWithDatabase` and `role.NewAuthorizerFromBuildContext` compose the DB-backed permission checker into the Gateway `RBACAuthorizer`. Applications provide the App-owned database capability; they do not construct SQL joins, button relationships, or permission matching logic themselves.
