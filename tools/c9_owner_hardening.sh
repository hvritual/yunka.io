#!/usr/bin/env bash
set -euo pipefail
python3 - <<'PY'
from pathlib import Path
p = Path('app/cmd/domain/multi_policy_templates.go')
s = p.read_text()
old = '''\tfor _, current := range object.Fields {
\t\tname := fieldGoName(current)
\t\tfmt.Fprintf(b, "%s:input.%s,", name, name)
\t}
\tfmt.Fprintf(b, "};created,err:='''
new = '''\tfor _, current := range object.Fields {
\t\tname := fieldGoName(current)
\t\tif isPolicyOwnerField(current) {
\t\t\tfmt.Fprintf(b, "%s:principal.UserID,", name)
\t\t\tcontinue
\t\t}
\t\tfmt.Fprintf(b, "%s:input.%s,", name, name)
\t}
\tfmt.Fprintf(b, "};created,err:='''
if old not in s:
    raise SystemExit('create assignment loop not found')
s = s.replace(old, new, 1)
old = '''\tfor _, current := range object.Fields {
\t\tname := fieldGoName(current)
\t\tfmt.Fprintf(b, "value.%s=input.%s;", name, name)
\t}
\tfmt.Fprintf(b, "if err:=repository.Update'''
new = '''\tfor _, current := range object.Fields {
\t\tif isPolicyOwnerField(current) {
\t\t\tcontinue
\t\t}
\t\tname := fieldGoName(current)
\t\tfmt.Fprintf(b, "value.%s=input.%s;", name, name)
\t}
\tfmt.Fprintf(b, "if err:=repository.Update'''
if old not in s:
    raise SystemExit('update assignment loop not found')
s = s.replace(old, new, 1)
p.write_text(s)

p = Path('app/cmd/domain/multi_policy_standard_helpers.go')
s = p.read_text()
s += '''
func isPolicyOwnerField(field Field) bool {
\tswitch field.Column {
\tcase "created_by", "owner_id":
\t\treturn true
\tdefault:
\t\treturn false
\t}
}
'''
p.write_text(s)

p = Path('app/cmd/domain/policy_generation_test.go')
s = p.read_text()
needle = '''\tfor _, expected := range []string{
\t\t"type DeviceUseCases interface",'''
if needle not in s:
    raise SystemExit('policy test marker not found')
# Append ownership assertions after generated application expected-loop block using stable next statement.
needle2 = '''\trepositories := mustReadPolicyGenerated(t, filepath.Join(domainRoot, "infrastructure", "persistence", "zz_yunka_repositories_gen.go"))'''
insert = '''\tif !strings.Contains(application, "CreatedBy: principal.UserID") {
\t\tt.Fatal("generated create must derive ownership from trusted principal")
\t}
\tif strings.Contains(application, "value.CreatedBy = input.CreatedBy") {
\t\tt.Fatal("generated update must not allow ownership reassignment")
\t}
'''
if needle2 not in s:
    raise SystemExit('repository assertion marker not found')
s = s.replace(needle2, insert + needle2, 1)
p.write_text(s)
PY

gofmt -w app/cmd/domain/multi_policy_templates.go app/cmd/domain/multi_policy_standard_helpers.go app/cmd/domain/policy_generation_test.go

go test ./framework/policy ./app/cmd/domain
