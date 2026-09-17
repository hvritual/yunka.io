#!/usr/bin/env python3
from pathlib import Path

ROOT = Path.cwd()


def read(path):
    return (ROOT / path).read_text()


def write(path, text):
    (ROOT / path).write_text(text)


def replace_once(path, old, new):
    text = read(path)
    count = text.count(old)
    if count != 1:
        raise SystemExit(f"{path}: expected one match, found {count}: {old[:120]!r}")
    write(path, text.replace(old, new, 1))

# Protobuf singular embedded-message fields merge repeated wire occurrences.
# Parse each chunk without prematurely validating a partial message; validate the
# merged BoundaryIntent exactly once after the parent Operation is decoded.
write("pkg/contract/boundary_intent.go", '''package contract

import (
    "fmt"
    "strings"
)

// ValidateBoundaryIntent validates explicit data only. Nil is legacy UNKNOWN,
// not a successful boundary decision. This does not infer business ownership.
func ValidateBoundaryIntent(intent *BoundaryIntent) error {
    if intent == nil {
        return nil
    }
    if !validPolicyKey(intent.Context) {
        return fmt.Errorf("boundary intent: context must be a stable lowercase key")
    }
    aggregate := strings.TrimSpace(intent.Aggregate)
    reason := strings.TrimSpace(intent.AggregateNotApplicableReason)
    if (aggregate == "") == (reason == "") {
        return fmt.Errorf("boundary intent: exactly one of aggregate or aggregate_not_applicable_reason is required")
    }
    if aggregate != "" && !validPolicyKey(aggregate) {
        return fmt.Errorf("boundary intent: aggregate must be a stable lowercase key")
    }
    if strings.ContainsRune(reason, '\\x00') {
        return fmt.Errorf("boundary intent: aggregate explanation contains NUL")
    }
    return nil
}

func mergeBoundaryIntent(result *BoundaryIntent, data []byte) error {
    if result == nil {
        return fmt.Errorf("boundary intent: merge target is required")
    }
    return scanWire(data, func(field wireField) error {
        if field.Type != 2 {
            return fmt.Errorf("boundary intent: field %d must be a string", field.Number)
        }
        switch field.Number {
        case 1:
            result.Context = string(field.Bytes)
        case 2:
            result.Aggregate = string(field.Bytes)
        case 3:
            result.AggregateNotApplicableReason = string(field.Bytes)
        default:
            return fmt.Errorf("boundary intent: unsupported field %d", field.Number)
        }
        return nil
    })
}

func parseBoundaryIntent(data []byte) (*BoundaryIntent, error) {
    result := &BoundaryIntent{}
    if err := mergeBoundaryIntent(result, data); err != nil {
        return nil, err
    }
    if err := ValidateBoundaryIntent(result); err != nil {
        return nil, err
    }
    return result, nil
}

func validateManifestBoundaryIntents(manifest Manifest) error {
    for _, service := range manifest.Services {
        if service.Application != nil {
            for _, op := range service.Application.Operations {
                if err := ValidateBoundaryIntent(op.Boundary); err != nil {
                    return fmt.Errorf("%s: %w", op.ID, err)
                }
            }
        }
        for _, method := range service.Methods {
            if method.Operation != nil {
                if err := ValidateBoundaryIntent(method.Operation.Boundary); err != nil {
                    return fmt.Errorf("%s: %w", method.FullName, err)
                }
            }
        }
    }
    return nil
}
''')

replace_once(
    "pkg/contract/dsl_descriptor.go",
    '''\t\tcase 14:
\t\t\tif field.Type == 2 {
\t\t\t\tboundary, err := parseBoundaryIntent(field.Bytes)
\t\t\t\tif err != nil {
\t\t\t\t\treturn err
\t\t\t\t}
\t\t\t\tresult.Boundary = boundary
\t\t\t}
''',
    '''\t\tcase 14:
\t\t\tif field.Type != 2 {
\t\t\t\treturn fmt.Errorf("boundary intent: operation field 14 must be a message")
\t\t\t}
\t\t\tif result.Boundary == nil {
\t\t\t\tresult.Boundary = &BoundaryIntent{}
\t\t\t}
\t\t\tif err := mergeBoundaryIntent(result.Boundary, field.Bytes); err != nil {
\t\t\t\treturn err
\t\t\t}
''')
replace_once(
    "pkg/contract/dsl_descriptor.go",
    '''\tresult.Permissions = stableStrings(result.Permissions)
\tresult.Authentication = stableStrings(result.Authentication)
\tresult.RequiresOperations = stableStrings(result.RequiresOperations)
\treturn result, nil
}

func parseExecutionPolicy''',
    '''\tresult.Permissions = stableStrings(result.Permissions)
\tresult.Authentication = stableStrings(result.Authentication)
\tresult.RequiresOperations = stableStrings(result.RequiresOperations)
\tif err := ValidateBoundaryIntent(result.Boundary); err != nil {
\t\treturn nil, err
\t}
\treturn result, nil
}

func parseExecutionPolicy''')

# JSON-loaded and programmatically supplied manifests must obey the same explicit
# BoundaryIntent invariant as descriptor-compiled manifests.
replace_once(
    "pkg/contract/artifact.go",
    '''\tmanifest.Normalize()
\treturn manifest, nil
}

func writeFileAtomic''',
    '''\tmanifest.Normalize()
\tif err := validateManifestBoundaryIntents(manifest); err != nil {
\t\treturn Manifest{}, fmt.Errorf("contract: manifest boundary intent: %w", err)
\t}
\treturn manifest, nil
}

func writeFileAtomic''')
replace_once(
    "pkg/contract/operation_plan.go",
    '''func CompileOperationPlans(manifest Manifest) (operationplan.Set, error) {
\tmanifest.Normalize()
''',
    '''func CompileOperationPlans(manifest Manifest) (operationplan.Set, error) {
\tmanifest.Normalize()
\tif err := validateManifestBoundaryIntents(manifest); err != nil {
\t\treturn operationplan.Set{}, fmt.Errorf("contract operation plan: %w", err)
\t}
''')

# The provenance regression is about path/closure determinism, not pinning the
# previous manifest schema number. BoundaryIntent advances typed manifests to v5.
replace_once(
    "pkg/contract/inventory_provenance_test.go",
    '''\tif !bytes.Contains(one.Manifest, []byte(`"schemaVersion": 4`)) {
\t\tt.Fatalf("typed manifest not v4: %s", one.Manifest)
\t}
''',
    '''\tif !bytes.Contains(one.Manifest, []byte(`"schemaVersion": 5`)) {
\t\tt.Fatalf("typed manifest not v5: %s", one.Manifest)
\t}
''')

print("boundary validation compatibility fixes prepared")
