#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

def read(path): return (ROOT / path).read_text()
def write(path, text): (ROOT / path).write_text(text)
def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:200]!r}")
    write(path, text.replace(old, new, 1))

# The pressure fixture previously created its canonical baseline before Git existed.
# Hardclose1 intentionally makes Git identity part of authoring evidence, so create
# a seed commit immediately after the structural Application exists, before the
# first Operation is planned/applied. The later commit remains the full fixture
# baseline used by Change Contract / ChangeSet tests.
replace_once(
    "app/cmd/change/pressure_qualification_test.go",
    '''\tif _, err := add.AddApplication(add.ApplicationOptions{Root: root, Key: "tenant/lifecycle"}); err != nil {
\t\tt.Fatalf("add application: %v", err)
\t}
\tfor _, operation := range []struct {''',
    '''\tif _, err := add.AddApplication(add.ApplicationOptions{Root: root, Key: "tenant/lifecycle"}); err != nil {
\t\tt.Fatalf("add application: %v", err)
\t}
\tgitPressure(t, root, "init")
\tgitPressure(t, root, "config", "user.email", "ax7-pressure@example.invalid")
\tgitPressure(t, root, "config", "user.name", "AX7 Pressure")
\tgitPressure(t, root, "add", "-A")
\tgitPressure(t, root, "commit", "-m", "AX7 pressure authoring seed")
\tfor _, operation := range []struct {''')
replace_once(
    "app/cmd/change/pressure_qualification_test.go",
    '''\tfixture := pressureFixture{Root: root, ProtoPath: protoPath, ContractPath: DefaultChangeContractPath}
\tgeneratePressureProject(t, fixture)
\tgitPressure(t, root, "init")
\tgitPressure(t, root, "config", "user.email", "ax7-pressure@example.invalid")
\tgitPressure(t, root, "config", "user.name", "AX7 Pressure")
\tgitPressure(t, root, "add", "-A")
\tgitPressure(t, root, "commit", "-m", "AX7 pressure baseline")''',
    '''\tfixture := pressureFixture{Root: root, ProtoPath: protoPath, ContractPath: DefaultChangeContractPath}
\tgeneratePressureProject(t, fixture)
\tgitPressure(t, root, "add", "-A")
\tgitPressure(t, root, "commit", "-m", "AX7 pressure baseline")''')

print("hardclose1 pressure fixture Git ordering prepared")
