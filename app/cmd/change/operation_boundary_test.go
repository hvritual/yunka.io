package change

import (
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/add"
)

func TestIssue161GateChangeSetCreateEntryRejectsBlockedStaleAndLegacyPlans(t *testing.T) {
	for _, kind := range []string{"blocked", "different-base", "legacy"} {
		t.Run(kind, func(t *testing.T) {
			f := newPressureFixture(t)
			oldBase := gitPressure(t, f.Root, "rev-parse", "HEAD")
			opts := apiKeyArchiveOptions(f.Root)
			if kind == "blocked" {
				opts.Boundary.Context = "other.capability"
			}
			if kind == "different-base" {
				gitPressure(t, f.Root, "commit", "--allow-empty", "-m", "new plan baseline")
			}
			plan, err := add.PlanOperation(opts)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "legacy" {
				plan.SchemaVersion = 1
			}
			payload, err := add.Render(plan, add.FormatAgentJSON)
			if err != nil {
				t.Fatal(err)
			}
			name := ".yunka/boundary-create-plan.json"
			writePressureFile(t, filepath.Join(f.Root, name), payload)
			original := readPressureFile(t, f.protoFile())
			_, _, err = BuildChangeSet(f.Root, oldBase, nil, []string{name})
			if err == nil {
				t.Fatalf("accepted %s create plan", kind)
			}
			want := "STALE_BOUNDARY_PROOF"
			if kind == "blocked" {
				want = "OPERATION_BOUNDARY_BLOCKED"
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("wrong failure %v", err)
			}
			if readPressureFile(t, f.protoFile()) != original {
				t.Fatal("revalidation mutated source")
			}
		})
	}
}
