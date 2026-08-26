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

## Ownership

- PB/API Operation references Permission.
- UI Button/Menu references Permission only for visibility/navigation.
- Tenant Role owns Permission grants.
- Gateway Authorizer is the backend enforcement point.
- Resource/data scope is a separate evaluator seam; PB must not contain SQL predicates.

The historical `AuthBit` and API/Button join remain compatibility inputs during C8.3 migration only. New authorization code must use typed policies and Permission keys.
