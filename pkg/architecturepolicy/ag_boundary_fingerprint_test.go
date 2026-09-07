package architecturepolicy

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// The golden inventory binds the exact reviewed fixture bytes and expectations.
// It is not a signature or a guarantee against changing the verifier itself.
// Intentional fixture edits require a reviewed update of the golden entry;
// expected digests must never be refreshed from candidate fixtures at test time.
func agBoundaryCaseDigest(tc agBoundaryCase) (string, error) {
	type source struct {
		Path string `json:"path"`
		Body string `json:"body"`
	}
	paths := make([]string, 0, len(tc.files))
	for path := range tc.files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	definition := struct {
		Version    int      `json:"version"`
		Name       string   `json:"name"`
		Output     string   `json:"output"`
		Diagnostic string   `json:"diagnostic"`
		Sources    []source `json:"sources"`
	}{Version: 1, Name: tc.name, Output: tc.output, Diagnostic: tc.diagnostic, Sources: make([]source, 0, len(paths))}
	for _, path := range paths {
		definition.Sources = append(definition.Sources, source{Path: path, Body: tc.files[path]})
	}
	data, err := json.Marshal(definition)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func TestAGBoundaryFixtureDefinitions(t *testing.T) {
	for _, mutate := range []struct {
		name  string
		apply func([]agBoundaryCase)
	}{
		{"same_name_fixture_swap", func(c []agBoundaryCase) { c[1].files, c[1].output = c[0].files, c[0].output }},
		{"same_category_negative_swap", func(c []agBoundaryCase) { c[9].files = c[10].files }},
		{"source_body", func(c []agBoundaryCase) { c[1].files["internal/b/b.go"] += "// changed fixture\n" }},
		{"file_path", func(c []agBoundaryCase) {
			c[1].files["internal/b/renamed.go"] = c[1].files["internal/b/b.go"]
			delete(c[1].files, "internal/b/b.go")
		}},
		{"added_source", func(c []agBoundaryCase) { c[1].files["internal/b/extra.go"] = "package b\n" }},
		{"removed_source", func(c []agBoundaryCase) { delete(c[1].files, "internal/b/b.go") }},
		{"output", func(c []agBoundaryCase) { c[1].output = "different-output" }},
		{"diagnostic", func(c []agBoundaryCase) { c[9].diagnostic = "^.*$" }},
		{"same_name_stub", func(c []agBoundaryCase) {
			c[1].files = map[string]string{"cmd/probe/main.go": "package main\nimport \"fmt\"\nfunc main(){fmt.Println(\"read-ok\")}\n"}
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			cases := agBoundaryCases()
			mutate.apply(cases)
			err := agBoundaryValidateCases(cases)
			if err == nil {
				t.Fatal("AG_REPLACEMENT_FALSE_PASS: fixture content changed without inventory rejection")
			}
			if !strings.Contains(err.Error(), "fixture definition drift") {
				t.Fatalf("wrong rejection reason: %v", err)
			}
		})
	}
	t.Run("map_insertion_order", func(t *testing.T) {
		cases := agBoundaryCases()
		for i := range cases {
			paths := make([]string, 0, len(cases[i].files))
			for path := range cases[i].files {
				paths = append(paths, path)
			}
			sort.Sort(sort.Reverse(sort.StringSlice(paths)))
			files := make(map[string]string, len(paths))
			for _, path := range paths {
				files[path] = cases[i].files[path]
			}
			cases[i].files = files
		}
		if err := agBoundaryValidateCases(cases); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("case_order", func(t *testing.T) {
		cases := agBoundaryCases()
		cases[1], cases[2] = cases[2], cases[1]
		if err := agBoundaryValidateCases(cases); err != nil {
			t.Fatal(err)
		}
	})
}
