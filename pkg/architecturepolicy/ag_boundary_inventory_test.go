package architecturepolicy

import (
	"fmt"
	"strings"
	"testing"
)

// This inventory fixes the scope of AG-01's mechanism claim. Updating it is an
// explicit test-policy change, not evidence that arbitrary consumers are safe.
// false means a buildable/executable characterization, true a compiler rejection.
var agBoundaryRequiredCases = map[string]bool{
	"owner_factory":                            false,
	"root_internal_allows_sibling":             false,
	"narrow_interface_wide_object":             false,
	"private_explicit_wrapper":                 false,
	"concrete_return_leaks_without_import":     false,
	"embedding_promotes_extra_method":          false,
	"public_unwrap_leaks":                      false,
	"same_package_cross_file_access":           false,
	"different_package_private_field_rejected": true,
	"nested_internal_rejects_store":            true,
	"nested_internal_rejects_renamed":          true,
}

func agBoundaryValidateCases(cases []agBoundaryCase) error {
	if len(cases) != len(agBoundaryRequiredCases) {
		return fmt.Errorf("AG-01 inventory: have %d cases; require %d", len(cases), len(agBoundaryRequiredCases))
	}
	if cases[0].name != "owner_factory" {
		return fmt.Errorf("AG-01 inventory: legal owner_factory control must run first")
	}
	seen := make(map[string]bool, len(cases))
	for _, tc := range cases {
		reject, ok := agBoundaryRequiredCases[tc.name]
		if !ok || seen[tc.name] {
			return fmt.Errorf("AG-01 inventory: unknown or duplicate case %q", tc.name)
		}
		seen[tc.name] = true
		if reject != (tc.diagnostic != "") || (tc.output == "") != reject {
			return fmt.Errorf("AG-01 inventory: wrong outcome contract for %q", tc.name)
		}
		if strings.TrimSpace(tc.files["cmd/probe/main.go"]) == "" {
			return fmt.Errorf("AG-01 inventory: missing executable probe for %q", tc.name)
		}
		for name, body := range tc.files {
			if _, err := agBoundaryPath("fixture", name); err != nil {
				return fmt.Errorf("AG-01 inventory: %s: %w", tc.name, err)
			}
			if !strings.HasSuffix(name, ".go") || strings.TrimSpace(body) == "" {
				return fmt.Errorf("AG-01 inventory: %s has invalid source %q", tc.name, name)
			}
		}
	}
	return nil
}

func TestAGBoundaryInventory(t *testing.T) {
	if err := agBoundaryValidateCases(agBoundaryCases()); err != nil {
		t.Fatal(err)
	}
}

func TestAGBoundaryInventoryRejectsSilentCoverageLoss(t *testing.T) {
	for _, mutate := range []struct {
		name  string
		apply func([]agBoundaryCase) []agBoundaryCase
	}{
		{"empty", func(_ []agBoundaryCase) []agBoundaryCase { return nil }},
		{"remove_last_negative", func(c []agBoundaryCase) []agBoundaryCase { return c[:len(c)-1] }},
		{"duplicate_replaces_negative", func(c []agBoundaryCase) []agBoundaryCase { c[len(c)-1] = c[0]; return c }},
		{"unknown_replaces_negative", func(c []agBoundaryCase) []agBoundaryCase { c[len(c)-1].name = "unreviewed"; return c }},
		{"control_not_first", func(c []agBoundaryCase) []agBoundaryCase { c[0], c[1] = c[1], c[0]; return c }},
		{"negative_becomes_positive", func(c []agBoundaryCase) []agBoundaryCase {
			c[len(c)-1].diagnostic = ""
			c[len(c)-1].output = "ok"
			return c
		}},
		{"positive_becomes_negative", func(c []agBoundaryCase) []agBoundaryCase { c[1].diagnostic = "rejected"; c[1].output = ""; return c }},
		{"ambiguous_outcome", func(c []agBoundaryCase) []agBoundaryCase { c[len(c)-1].output = "ok"; return c }},
		{"empty_outcome", func(c []agBoundaryCase) []agBoundaryCase { c[1].output = ""; return c }},
		{"missing_probe", func(c []agBoundaryCase) []agBoundaryCase { delete(c[1].files, "cmd/probe/main.go"); return c }},
		{"override_locked_module", func(c []agBoundaryCase) []agBoundaryCase { c[1].files["go.mod"] = "module other"; return c }},
		{"escaping_source", func(c []agBoundaryCase) []agBoundaryCase { c[1].files["../escape.go"] = "package escape"; return c }},
		{"empty_source", func(c []agBoundaryCase) []agBoundaryCase { c[1].files["internal/a/empty.go"] = " "; return c }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			if err := agBoundaryValidateCases(mutate.apply(agBoundaryCases())); err == nil {
				t.Fatal("silent coverage loss accepted")
			}
		})
	}
}
