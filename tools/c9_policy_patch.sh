#!/usr/bin/env bash
set -euo pipefail

python3 - <<'PY'
from pathlib import Path
path = Path('app/cmd/domain/generator.go')
text = path.read_text()
old = 'files := renderMultiStructural(spec, packageImport)'
new = 'files := renderMultiPolicyStructural(spec, packageImport)'
if old not in text:
    raise SystemExit('renderMultiStructural call not found')
path.write_text(text.replace(old, new, 1))
PY

gofmt -w framework/policy app/cmd/domain/multi_policy_templates.go app/cmd/domain/multi_policy_structural.go app/cmd/domain/policy_generation_test.go app/cmd/domain/generator.go

go test ./framework/policy ./app/cmd/domain
