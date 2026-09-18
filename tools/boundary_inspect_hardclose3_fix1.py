#!/usr/bin/env python3
from pathlib import Path

path = Path("app/cmd/boundary/command_test.go")
text = path.read_text()
old = '''\tif !reflect.DeepEqual(decoded, first) {
\t\tt.Fatalf("JSON projection changed inspection facts: %#v %#v", decoded, first)
\t}
\ttext, err := Render(first, "text")'''
new = '''\treencoded, err := Render(decoded, "json")
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif reencoded != jsonOne {
\t\tt.Fatalf("JSON projection is not canonical after round-trip:\\nfirst=%s\\nagain=%s", jsonOne, reencoded)
\t}
\ttext, err := Render(first, "text")'''
if text.count(old) != 1:
    raise SystemExit("command_test.go: JSON round-trip anchor mismatch")
path.write_text(text.replace(old, new, 1))
print("Hardclose 3 JSON canonical round-trip qualification fixed")
