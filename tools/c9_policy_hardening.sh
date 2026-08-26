#!/usr/bin/env bash
set -euo pipefail
python3 - <<'PY'
from pathlib import Path
p = Path('framework/policy/policy.go')
s = p.read_text()
s = s.replace('allow: func(identity.Principal, Grant, T) bool { return true },\n\t\tfilter: func(identity.Principal, Grant) Filter { return Filter{All: true} },', 'allow: func(_ identity.Principal, grant Grant, _ T) bool { return grant.All },\n\t\tfilter: func(_ identity.Principal, grant Grant) Filter { return Filter{All: grant.All} },', 1)
p.write_text(s)

p = Path('app/cmd/domain/multi_policy_templates.go')
s = p.read_text()
old = 'for _, object := range spec.Objects {\n\t\twriteMultiRESTObject(&b, object)\n\t}'
new = 'for _, object := range spec.Objects {\n\t\twritePolicyRESTObject(&b, object)\n\t}'
if old not in s:
    raise SystemExit('REST renderer call not found')
p.write_text(s.replace(old, new, 1))
PY

gofmt -w framework/policy app/cmd/domain/multi_policy_templates.go app/cmd/domain/multi_policy_rest_object.go

go test ./framework/policy ./app/cmd/domain
