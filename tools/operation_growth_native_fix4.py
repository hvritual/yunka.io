#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()

def read(path): return (ROOT / path).read_text()
def write(path, text):
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text)
def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:180]!r}")
    write(path, text.replace(old, new, 1))

# The pressure fixture models Yunka DSL as an external compiler include. Remove
# the temporary vendoring introduced by fix3; immutable-base Audit must consume
# the same explicit include profile instead of altering consumer source roots.
replace_once(
    "app/cmd/change/pressure_qualification_test.go",
    '''\tdsl, err := os.ReadFile(filepath.Join(protoPath, "yunka", "dsl", "v1", "options.proto"))
\tif err != nil { t.Fatal(err) }
\twritePressureFile(t, filepath.Join(root, "contracts", "proto", "yunka", "dsl", "v1", "options.proto"), string(dsl))

''',
    '')

# The remaining positive add fixture should establish the first boundary
# explicitly; this is initialization, not silent growth.
replace_once(
    "app/cmd/add/main_test.go",
    '''\t\tTransaction:    "none",
\t\tIdempotency:    "none",
\t\tComposition:    "none",
\t})''',
    '''\t\tTransaction:       "none",
\t\tIdempotency:       "none",
\t\tComposition:       "none",
\t\tBoundaryContext:   "tenant.lifecycle",
\t\tBoundaryAggregate: "tenant",
\t})''')

# Audit boundary growth compilation is source-based and therefore must receive
# the exact compiler/include profile already supplied to Change verification.
# CLI keeps the old root-only API as a compatibility wrapper.
replace_once(
    "app/cmd/audit/command.go",
    '''\t\t\tcli.StringFlag{Name: "base", Usage: "optional Git ref used to classify proven findings as existing, new, or fixed debt"},
\t\t\tcli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},''',
    '''\t\t\tcli.StringFlag{Name: "base", Usage: "optional Git ref used to classify proven findings as existing, new, or fixed debt"},
\t\t\tcli.StringFlag{Name: "protoc", EnvVar: "PROTOC", Usage: "protoc binary used for base/current Operation Growth evidence"},
\t\t\tcli.StringSliceFlag{Name: "proto-path", Usage: "additional protobuf include path used for base/current Operation Growth evidence; may be repeated"},
\t\t\tcli.StringFlag{Name: "format", Value: "text", Usage: "output format: text, json, or agent-json"},''')
replace_once(
    "app/cmd/audit/command.go",
    '''\t\t\t} else {
\t\t\t\treport, err = BuildWithBase(c.String("root"), c.String("base"))
\t\t\t}''',
    '''\t\t\t} else {
\t\t\t\treport, err = BuildWithBaseOptions(projectflow.Options{Root: c.String("root"), Protoc: c.String("protoc"), ProtoPaths: c.StringSlice("proto-path")}, c.String("base"))
\t\t\t}''')
replace_once(
    "app/cmd/audit/command.go",
    '''func BuildWithBase(root, baseRef string) (auditcore.Report, error) {
\tcurrent, descriptor, err := buildCurrent(root)''',
    '''func BuildWithBase(root, baseRef string) (auditcore.Report, error) {
\treturn BuildWithBaseOptions(projectflow.Options{Root: root}, baseRef)
}

func BuildWithBaseOptions(options projectflow.Options, baseRef string) (auditcore.Report, error) {
\tcurrent, descriptor, err := buildCurrent(options.Root)''')
replace_once(
    "app/cmd/audit/command.go",
    '''\tgrowth, err := boundaryGrowthFindings(descriptor.Root, baselineRoot, baseSHA)
\tif err != nil { return auditcore.Report{}, err }
\tcurrent.Findings = append(current.Findings, growth...)
\tdebt := auditcore.CompareProvenFindings(baseline.Findings, current.Findings)''',
    '''\tgrowth, err := boundaryGrowthFindings(projectflow.Options{Root: descriptor.Root, Protoc: options.Protoc, ProtoPaths: append([]string(nil), options.ProtoPaths...)}, baselineRoot, baseSHA)
\tif err != nil { return auditcore.Report{}, err }
\tcurrent.Findings = append(current.Findings, growth...)
\tdebt := auditcore.CompareProvenFindings(baseline.Findings, current.Findings)''')

# Rebuild boundary_growth.go with compiler-option rebasing. Relative include paths
# inside the consumer project are rebound to the immutable baseline checkout;
# external includes remain the same physical caller-supplied source. This avoids
# current-worktree bytes masquerading as historical project-owned evidence.
write("app/cmd/audit/boundary_growth.go", r'''package audit

import (
    "context"
    "fmt"
    "path/filepath"
    "strings"

    "yunka.io/app/cmd/auditcore"
    "yunka.io/app/cmd/boundarycore"
    "yunka.io/app/cmd/projectflow"
)

const RuleOperationGrowthBoundary = "AUDIT-BOUNDARY-001"

func boundaryGrowthFindings(currentOptions projectflow.Options, baselineRoot, baseSHA string) ([]auditcore.Finding, error) {
    currentRoot, err := filepath.Abs(strings.TrimSpace(currentOptions.Root))
    if err != nil { return nil, fmt.Errorf("audit boundary growth: current root: %w", err) }
    baselineOptions, err := rebaseBoundaryCompilerOptions(currentOptions, currentRoot, baselineRoot)
    if err != nil { return nil, err }
    currentOptions.Root = currentRoot

    baseline, err := projectflow.DescribeContractSourceSnapshot(context.Background(), baselineOptions)
    if err != nil { return nil, fmt.Errorf("audit boundary growth: compile immutable baseline %s: %w", baseSHA, err) }
    current, err := projectflow.DescribeContractSourceSnapshot(context.Background(), currentOptions)
    if err != nil { return nil, fmt.Errorf("audit boundary growth: compile current canonical source: %w", err) }
    events := boundarycore.EvaluateGrowth(baseSHA, baseline.Manifest, current.Manifest)
    result := []auditcore.Finding{}
    for _, event := range events {
        if event.Outcome == boundarycore.ReuseExistingService { continue }
        result = append(result, auditcore.Finding{
            ID: RuleOperationGrowthBoundary + ":" + event.OperationID + ":" + event.Kind,
            Rule: RuleOperationGrowthBoundary, Class: auditcore.FindingProvenViolation, Blocking: true,
            Subject: event.OperationID,
            Summary: "Operation Growth is not proven to remain inside the existing Service Boundary",
            Invariant: "new or boundary-changing Operations must recompute canonical base/current Service Boundary evidence; non-reuse outcomes cannot enter silently",
            Reason: event.Reason + "; outcome=" + event.Outcome,
            Remediation: "review the boundary decision; split the Application/Service projection when contradicted, or supply explicit compatible canonical boundary evidence before growth",
            Evidence: []auditcore.Evidence{
                {Kind: auditcore.EvidenceGit, Source: "git.base", Detail: baseSHA},
                {Kind: auditcore.EvidenceCanonical, Source: "contract.source", Detail: event.Kind + " outcome=" + event.Outcome},
            },
        })
    }
    return result, nil
}

func rebaseBoundaryCompilerOptions(current projectflow.Options, currentRoot, baselineRoot string) (projectflow.Options, error) {
    baselineAbs, err := filepath.Abs(baselineRoot)
    if err != nil { return projectflow.Options{}, fmt.Errorf("audit boundary growth: baseline root: %w", err) }
    currentRoot = filepath.Clean(currentRoot)
    result := projectflow.Options{Root: baselineAbs, Protoc: current.Protoc, ProtoPaths: make([]string, 0, len(current.ProtoPaths))}
    for _, raw := range current.ProtoPaths {
        value := strings.TrimSpace(raw)
        if value == "" { return projectflow.Options{}, fmt.Errorf("audit boundary growth: proto-path must not be blank") }
        absolute := value
        if !filepath.IsAbs(absolute) { absolute = filepath.Join(currentRoot, absolute) }
        absolute = filepath.Clean(absolute)
        rel, relErr := filepath.Rel(currentRoot, absolute)
        inside := relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
        if inside {
            result.ProtoPaths = append(result.ProtoPaths, filepath.Join(baselineAbs, rel))
        } else {
            result.ProtoPaths = append(result.ProtoPaths, absolute)
        }
    }
    return result, nil
}
''')

# Thread the verifier's exact compiler/include profile through architecture debt.
replace_once(
    "app/cmd/change/architecture_debt.go",
    '''\t"yunka.io/app/cmd/auditcore"
)''',
    '''\t"yunka.io/app/cmd/auditcore"
\t"yunka.io/app/cmd/projectflow"
)''')
replace_once(
    "app/cmd/change/architecture_debt.go",
    '''func collectArchitectureDebt(root, baseSHA string) (auditcore.DebtDelta, error) {
\tbaseSHA = strings.TrimSpace(baseSHA)''',
    '''func collectArchitectureDebt(root, baseSHA string) (auditcore.DebtDelta, error) {
\treturn collectArchitectureDebtWithOptions(projectflow.Options{Root: root}, baseSHA)
}

func collectArchitectureDebtWithOptions(options projectflow.Options, baseSHA string) (auditcore.DebtDelta, error) {
\tbaseSHA = strings.TrimSpace(baseSHA)''')
replace_once(
    "app/cmd/change/architecture_debt.go",
    '''\treport, err := audit.BuildWithBase(root, baseSHA)''',
    '''\treport, err := audit.BuildWithBaseOptions(options, baseSHA)''')
replace_once(
    "app/cmd/change/verify.go",
    '''\t\tarchitectureDebt, architectureDebtErr := collectArchitectureDebt(descriptor.Root, contractValue.BaseSHA)''',
    '''\t\tarchitectureDebt, architectureDebtErr := collectArchitectureDebtWithOptions(projectflow.Options{Root: descriptor.Root, Protoc: options.Protoc, ProtoPaths: append([]string(nil), options.ProtoPaths...)}, contractValue.BaseSHA)''')

print("audit include-profile binding and final positive fixture prepared")
