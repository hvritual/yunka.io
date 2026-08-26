#!/usr/bin/env python3
from pathlib import Path

path = Path("/tmp/c83-wave34-retry.sh")
text = path.read_text(encoding="utf-8")
old = "gofmt -w gateway/dispatcher/intercept gateway/dispatcher/intercept/role/db"
new = (
    'python3 "$C83_PATCH_ROLE_MIGRATION"\n'
    "find gateway/dispatcher/intercept -type f -name '*.go' -print0 | sort -z | xargs -0 -r gofmt -w\n"
    'python3 "$C83_REFRESH_CONSUMER_ABI"'
)
count = text.count(old)
if count != 1:
    raise SystemExit(f"C8.3: expected exactly one overlapping gofmt command, found {count}")
text = text.replace(old, new)
path.write_text(text, encoding="utf-8")
