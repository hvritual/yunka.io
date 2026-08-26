#!/usr/bin/env bash
set -euo pipefail
python3 - <<'PY'
from pathlib import Path
p = Path('app/cmd/domain/multi_policy_templates.go')
s = p.read_text()
old = 'fmt.Fprintf(b, "type %sAccessPolicy struct { Resolver policy.Resolver; Create policy.Rule[Create%sInput]; Read policy.Rule[domain.%s]; Update policy.Rule[domain.%s]; Delete policy.Rule[domain.%s] }\\n", entity, entity, entity, entity, entity)'
new = 'fmt.Fprintf(b, "type %sUpdateTarget struct { Current domain.%s; Input Update%sInput }\\n", entity, entity, entity)\n\tfmt.Fprintf(b, "type %sAccessPolicy struct { Resolver policy.Resolver; Create policy.Rule[Create%sInput]; Read policy.Rule[domain.%s]; Update policy.Rule[%sUpdateTarget]; Delete policy.Rule[domain.%s] }\\n", entity, entity, entity, entity, entity)'
if old not in s:
    raise SystemExit('AccessPolicy declaration pattern not found')
s = s.replace(old, new, 1)
old = 'fmt.Fprintf(b, "func(value %sAccessPolicy)AuthorizeUpdate(ctx context.Context,principal identity.Principal,current domain.%s,_ Update%sInput)error{return value.Update.Authorize(ctx,value.Resolver,principal,current)}\\n", entity, entity, entity)'
new = 'fmt.Fprintf(b, "func(value %sAccessPolicy)AuthorizeUpdate(ctx context.Context,principal identity.Principal,current domain.%s,input Update%sInput)error{return value.Update.Authorize(ctx,value.Resolver,principal,%sUpdateTarget{Current:current,Input:input})}\\n", entity, entity, entity, entity)\n\twriteStandardPolicyHelpers(b, object)'
if old not in s:
    raise SystemExit('AuthorizeUpdate pattern not found')
s = s.replace(old, new, 1)
p.write_text(s)

p = Path('app/cmd/domain/policy_generation_test.go')
s = p.read_text()
s = s.replace('"type DeviceAccessPolicy struct",\n\t\t"WithDevicePolicy",', '"type DeviceAccessPolicy struct",\n\t\t"type DeviceUpdateTarget struct",\n\t\t"type DevicePermissions struct",\n\t\t"StandardDeviceAccess",\n\t\t"standardDeviceUpdateMatcher",\n\t\t"WithDevicePolicy",')
p.write_text(s)
PY

gofmt -w app/cmd/domain/multi_policy_templates.go app/cmd/domain/multi_policy_standard_helpers.go app/cmd/domain/policy_generation_test.go

go test ./framework/policy ./app/cmd/domain
