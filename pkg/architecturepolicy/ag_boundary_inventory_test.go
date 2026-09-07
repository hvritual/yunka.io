package architecturepolicy

import (
	"fmt"
	"strings"
	"testing"
)

// This inventory fixes the scope of AG-01's mechanism claim. Updating it is an
// explicit test-policy change, not evidence that arbitrary consumers are safe.
// false means a buildable/executable characterization, true a compiler rejection.
type agBoundaryFixtureContract struct {
	reject bool
	digest string
}

var agBoundaryRequiredCases = map[string]agBoundaryFixtureContract{
	"owner_factory":                            {false, "e3ad3022df7952a5e8f351bdb5ea6c9af5b45fa386e61a7a9a2488dd89e6bc69"},
	"root_internal_allows_sibling":             {false, "0118dc5eb96b7ab52248debea51112256ba7e7e79f5287c97002c2dedea21606"},
	"narrow_interface_wide_object":             {false, "494795947a843a88323f02f238f771f5e84d45ae11d41c9894f209019439bd5c"},
	"private_explicit_wrapper":                 {false, "6db0b9cc17d69d95ec62d5c9e6fabaa7678b2f336b2e03c8c1fae933e9096df3"},
	"concrete_return_leaks_without_import":     {false, "ad50d04d8f4d37abf86930983b0aeb8aaa42dd5637e6ae3af23a397a536b4cc7"},
	"embedding_promotes_extra_method":          {false, "b7f4fc2e545282b8bca06189a992d15ce140c569fcf5475b0f18b0673c3b813b"},
	"public_unwrap_leaks":                      {false, "82cff2b11a59bc7cef909e83f24c25160b4cff94777719c3b126ac5a33ec16c5"},
	"same_package_cross_file_access":           {false, "72f45da4425c37545263e4531245bb78212de59ce9f9b10df6ba425c9185777d"},
	"different_package_private_field_rejected": {true, "e4acd4be3dbc1d7ccf6d2f1c55b44f17614e0063c73a31f38c4cbb9235ecdda6"},
	"nested_internal_rejects_store":            {true, "a4360bcdbacff55327d62dc7561a413e74046b6aa21f998b5f63b9ea51fef4e1"},
	"nested_internal_rejects_renamed":          {true, "553ee53e3db37d05b70c5aa3c130cfe0f8f1a9ae2eedd803c4551a63aac950ff"},
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
		required, ok := agBoundaryRequiredCases[tc.name]
		if !ok || seen[tc.name] {
			return fmt.Errorf("AG-01 inventory: unknown or duplicate case %q", tc.name)
		}
		seen[tc.name] = true
		if required.reject != (tc.diagnostic != "") || (tc.output == "") != required.reject {
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
		digest, err := agBoundaryCaseDigest(tc)
		if err != nil {
			return fmt.Errorf("AG-01 inventory: digest %q: %w", tc.name, err)
		}
		if digest != required.digest {
			return fmt.Errorf("AG-01 inventory: fixture definition drift for %q", tc.name)
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
