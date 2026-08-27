from pathlib import Path
import sys

root = Path(sys.argv[1]).resolve()
path = root / "Makefile"
text = path.read_text(encoding="utf-8")
old = "YUNKA_ROOT ?= ../yunka.io\n"
new = "YUNKA_ROOT ?= $(abspath ../yunka.io)\n"
if old not in text:
    raise SystemExit("expected YUNKA_ROOT declaration not found")
path.write_text(text.replace(old, new, 1), encoding="utf-8")
