package change

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yunka.io/app/cmd/add"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

func proofSetFixture(t *testing.T) (pressureFixture, ChangeSet, add.OperationOptions) {
	t.Helper()
	fixture := newPressureFixture(t)
	options := changeSetCreateOptions(fixture.Root, true)
	path := writePlanReport(t, fixture.Root, options, "boundary-plan.json")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	return fixture, value, options
}

func cloneProofSet(t *testing.T, value ChangeSet) ChangeSet {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result ChangeSet
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func applyProofFixture(t *testing.T, fixture pressureFixture, options add.OperationOptions) {
	t.Helper()
	if _, err := add.AddOperation(options); err != nil {
		t.Fatal(err)
	}
	generatePressureProject(t, fixture)
}

func requireBoundaryFailure(t *testing.T, options projectflow.Options, value ChangeSet) {
	t.Helper()
	report, err := ReconcileChangeSetWithOptions(context.Background(), options, value)
	if err != nil {
		if !strings.Contains(err.Error(), "STALE_BOUNDARY_PROOF") {
			t.Fatalf("unexpected failure: %v", err)
		}
		return
	}
	if report.Conformant || report.Boundary == nil || len(report.Boundary.Violations) == 0 {
		t.Fatalf("missing boundary nonconformance: %#v", report)
	}
}

func TestIssue161SetBoundaryProofRoundTripAndDetached(t *testing.T) {
	fixture, value, options := proofSetFixture(t)
	proof := value.Subjects[0].Create.BoundaryProof
	if value.SchemaVersion != 3 || proof == nil || proof.SchemaVersion != 1 || !isBoundaryDigest(proof.BaseInputsDigest) || len(proof.Plan.BoundaryDecision.Evidence) == 0 {
		t.Fatalf("incomplete persisted evidence: %#v", value)
	}
	first, err := RenderChangeSet(value, DefaultChangeSetPath, FormatJSON)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := WriteChangeSet(fixture.Root, "", value); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadChangeSet(fixture.Root, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderChangeSet(loaded, DefaultChangeSetPath, FormatJSON)
	if err != nil || first != second {
		t.Fatalf("proof roundtrip changed: %v", err)
	}
	applyProofFixture(t, fixture, options)
	before := gitPressure(t, fixture.Root, "status", "--porcelain")
	a, err := ReconcileChangeSetWithOptions(context.Background(), fixture.compilerOptions(), loaded)
	if err != nil || !a.Conformant || a.Boundary == nil || len(a.Boundary.Entries) != 1 || !a.Boundary.Entries[0].Validated {
		t.Fatalf("valid proof rejected: %v %#v", err, a)
	}
	b, err := ReconcileChangeSetWithOptions(context.Background(), fixture.compilerOptions(), loaded)
	if err != nil || jsonValue(a) != jsonValue(b) {
		t.Fatalf("nondeterministic check: %v", err)
	}
	if before != gitPressure(t, fixture.Root, "status", "--porcelain") {
		t.Fatal("check changed Git state")
	}
	// Conversion must not share nested plan memory with a caller's mutable input.
	original := proof.Plan
	derived, err := createOperationChange(original)
	if err != nil {
		t.Fatal(err)
	}
	saved := derived.BoundaryProof.Plan.BoundaryDecision.Evidence[0].Digest
	original.BoundaryDecision.Evidence[0].Digest = strings.Repeat("0", 64)
	if derived.BoundaryProof.Plan.BoundaryDecision.Evidence[0].Digest != saved {
		t.Fatal("persisted evidence aliases caller-owned plan memory")
	}
}

func TestIssue161SetBoundaryRecomputesForgedProof(t *testing.T) {
	fixture, value, options := proofSetFixture(t)
	applyProofFixture(t, fixture, options)
	cases := []struct {
		name string
		edit func(*CreateOperationChange)
	}{
		{"missing", func(c *CreateOperationChange) { c.BoundaryProof = nil }},
		{"policy", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BoundaryDecision.PolicyVersion = "forged-policy" }},
		{"non-reuse", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BoundaryDecision.Outcome = boundarycore.ArchitectureReviewRequired }},
		{"plan-digest", func(c *CreateOperationChange) { c.PlanDigest = strings.Repeat("a", 64) }},
		{"base-input", func(c *CreateOperationChange) { c.BoundaryProof.BaseInputsDigest = strings.Repeat("b", 64) }},
		{"base-sha", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BaseSHA = strings.Repeat("c", 40) }},
		{"before-fingerprint", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BoundaryDecision.BeforeFingerprint = strings.Repeat("d", 64) }},
		{"candidate-plan", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BoundaryDecision.OperationPlanDigest = strings.Repeat("e", 64) }},
		{"omitted-counter-evidence", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BoundaryDecision.CounterEvidence = nil }},
		{"altered-dimension", func(c *CreateOperationChange) { c.BoundaryProof.Plan.BoundaryDecision.Dimensions[0].Reason = "invented supporting evidence" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := cloneProofSet(t, value)
			create := copy.Subjects[0].Create
			tc.edit(create)
			// Forge both unkeyed hashes to prove that hash consistency alone is
			// insufficient. The original canonical facts must still be recomputed.
			if create.BoundaryProof != nil && tc.name != "plan-digest" {
				d := create.BoundaryProof.Plan.BoundaryDecision
				d.DecisionDigest = ""
				data, _ := json.Marshal(d)
				sum := sha256.Sum256(append([]byte("service-boundary-decision/v1\n"), data...))
				d.DecisionDigest = hex.EncodeToString(sum[:])
				rendered, err := add.Render(create.BoundaryProof.Plan, add.FormatAgentJSON)
				if err != nil {
					t.Fatal(err)
				}
				sum = sha256.Sum256([]byte(rendered))
				create.PlanDigest = hex.EncodeToString(sum[:])
			}
			requireBoundaryFailure(t, fixture.compilerOptions(), copy)
		})
	}
}

func TestIssue161SetBoundaryLegacyExistingOnlyAndMissingAddition(t *testing.T) {
	fixture, value, _ := proofSetFixture(t)
	legacy := cloneProofSet(t, value)
	legacy.SchemaVersion = 2
	legacy.Subjects[0].Create.BoundaryProof = nil
	if err := validateChangeSet(legacy); err == nil || !errors.Is(err, boundarycore.ErrStaleBoundaryProof) {
		t.Fatalf("legacy creation bypassed proof requirements: %v", err)
	}
	// Existing-only v2 workflows keep their no-protoc quick check behavior.
	existing, _, err := BuildChangeSet(fixture.Root, "HEAD", []string{fixture.ContractPath}, nil)
	if err != nil {
		t.Fatal(err)
	}
	existing.SchemaVersion = 2
	if _, err := WriteChangeSet(fixture.Root, "", existing); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := LoadChangeSet(fixture.Root, "")
	if err != nil {
		t.Fatal(err)
	}
	report, err := ReconcileChangeSetWithOptions(context.Background(), projectflow.Options{Root: fixture.Root, Protoc: "definitely-not-a-protoc"}, loaded)
	if err != nil || !report.Conformant || report.Boundary != nil {
		t.Fatalf("unchanged legacy existing-only set was forced through growth verification: %v %#v", err, report)
	}
	// A declared create must be checked even without a protobuf Git delta.
	requireBoundaryFailure(t, fixture.compilerOptions(), value)
}

func TestIssue161SetBoundaryIndependentCreatesCannotHidePeerDrift(t *testing.T) {
	fixture := newPressureFixture(t)
	firstOptions := changeSetCreateOptions(fixture.Root, true)
	firstPath := writePlanReport(t, fixture.Root, firstOptions, "boundary-first.json")
	firstPlan, err := add.PlanOperation(firstOptions)
	if err != nil {
		t.Fatal(err)
	}
	secondOptions := firstOptions
	secondOptions.OperationID, secondOptions.UseCase, secondOptions.RPCName = "tenant.archive-again", "archive_again", "ArchiveAgain"
	secondOptions.RequestType, secondOptions.ResponseType = firstPlan.Identity["requestType"], firstPlan.Identity["responseType"]
	secondPath := writePlanReport(t, fixture.Root, secondOptions, "boundary-second.json")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{firstPath, secondPath})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := add.AddOperation(firstOptions); err != nil {
		t.Fatal(err)
	}
	if _, err := add.AddOperation(secondOptions); err != nil {
		t.Fatal(err)
	}
	generatePressureProject(t, fixture)
	report, err := ReconcileChangeSetWithOptions(context.Background(), fixture.compilerOptions(), value)
	if err != nil || !report.Conformant || report.Boundary == nil || len(report.Boundary.Entries) != 2 {
		t.Fatalf("independent parallel plans rejected: %v %#v", err, report)
	}
	mutateRPCOption(t, fixture, "ArchiveAgain", fmt.Sprintf("context: %q", pressureBoundary().Context), `context: "other.billing"`)
	requireBoundaryFailure(t, fixture.compilerOptions(), value)
	// Changing an original peer cannot be erased by declaring other additions.
	mutateRPCOption(t, fixture, "Resume", fmt.Sprintf("context: %q", pressureBoundary().Context), `context: "other.billing"`)
	report, err = ReconcileChangeSetWithOptions(context.Background(), fixture.compilerOptions(), value)
	if err != nil || report.Conformant || report.Boundary == nil {
		t.Fatalf("peer drift result: %v %#v", err, report)
	}
	for _, entry := range report.Boundary.Entries {
		if entry.Validated {
			t.Fatalf("changed base peer hidden from %s", entry.OperationID)
		}
	}
}

func TestIssue161SetBoundaryExplicitIncludeBytesAndProfiles(t *testing.T) {
	fixture := newPressureFixture(t)
	include := t.TempDir()
	err := filepath.WalkDir(fixture.ProtoPath, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".proto" {
			return err
		}
		rel, err := filepath.Rel(fixture.ProtoPath, path)
		if err != nil {
			return err
		}
		writePressureFile(t, filepath.Join(include, rel), readPressureFile(t, path))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	options := changeSetCreateOptions(fixture.Root, true)
	options.ProtoPaths = []string{include}
	path := writePlanReport(t, fixture.Root, options, "boundary-include.json")
	value, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	applyProofFixture(t, fixture, options)
	compiler := projectflow.Options{Root: fixture.Root, ProtoPaths: []string{include}}
	report, err := ReconcileChangeSetWithOptions(context.Background(), compiler, value)
	if err != nil || !report.Conformant {
		t.Fatalf("copied explicit include failed: %v %#v", err, report)
	}
	requireBoundaryFailure(t, fixture.compilerOptions(), value)
	file := filepath.Join(include, "yunka", "dsl", "v1", "options.proto")
	writePressureFile(t, file, readPressureFile(t, file)+"\n// external compiler input changed\n")
	requireBoundaryFailure(t, compiler, value)
}

func TestIssue161SetBoundaryRejectsIgnoredBaselineDriftAndCancellation(t *testing.T) {
	fixture := newPressureFixture(t)
	writePressureFile(t, filepath.Join(fixture.Root, ".yunka", "project.json"), "{\"schemaVersion\":1}\n")
	// An ignored resolution file is still canonical input; even when its values
	// equal defaults it cannot be silently attributed to the historical commit.
	options := changeSetCreateOptions(fixture.Root, true)
	path := writePlanReport(t, fixture.Root, options, "ignored-profile-plan.json")
	if _, _, err := BuildChangeSet(fixture.Root, "HEAD", nil, []string{path}); err == nil {
		t.Fatal("ignored baseline metadata was promoted to immutable proof")
	}
	clean, value, _ := proofSetFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReconcileChangeSetWithOptions(ctx, clean.compilerOptions(), value); err == nil {
		t.Fatal("cancelled proof reconstruction succeeded")
	}
}

func TestIssue161SetBoundarySurvivesCommittedCheckoutRelocation(t *testing.T) {
	fixture, value, options := proofSetFixture(t)
	applyProofFixture(t, fixture, options)
	gitPressure(t, fixture.Root, "add", "-A")
	gitPressure(t, fixture.Root, "commit", "-m", "qualified independent addition")
	clone := filepath.Join(t.TempDir(), "clone")
	gitPressure(t, fixture.Root, "clone", "--shared", fixture.Root, clone)
	report, err := ReconcileChangeSetWithOptions(context.Background(), projectflow.Options{Root: clone, ProtoPaths: options.ProtoPaths}, value)
	if err != nil || !report.Conformant || report.Boundary == nil || !report.Boundary.Entries[0].Validated {
		t.Fatalf("proof coupled to temporary baseline or original checkout path: %v %#v", err, report)
	}
}
