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

# Authoring boundary decisions are correctness evidence: bind them to the exact
# repository HEAD, and verify HEAD did not move while canonical facts were read.
replace_once(
    "app/cmd/add/boundary_gate.go",
    '''\t"fmt"
\t"strings"''',
    '''\t"fmt"
\t"os/exec"
\t"strings"''')
replace_once(
    "app/cmd/add/boundary_gate.go",
    '''func evaluateOperationBoundary(root, sourcePath, domain, application, packageName, rpcName, requestType, responseType string, options OperationOptions) (*OperationBoundaryDecision, error) {
\tsnapshot, err := projectflow.DescribeContractSourceSnapshot''',
    '''func evaluateOperationBoundary(root, sourcePath, domain, application, packageName, rpcName, requestType, responseType string, options OperationOptions) (*OperationBoundaryDecision, error) {
\tbaseSHA, err := resolveBoundaryBaseSHA(root)
\tif err != nil {
\t\treturn nil, fmt.Errorf("add operation: bind boundary decision to Git HEAD: %w", err)
\t}
\tsnapshot, err := projectflow.DescribeContractSourceSnapshot''')
replace_once(
    "app/cmd/add/boundary_gate.go",
    '''\tdecision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{Application: applicationKey, OperationID: options.OperationID}, before, after)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn &decision, nil
}''',
    '''\tcurrentSHA, err := resolveBoundaryBaseSHA(root)
\tif err != nil {
\t\treturn nil, fmt.Errorf("add operation: re-read Git HEAD before boundary decision: %w", err)
\t}
\tif currentSHA != baseSHA {
\t\treturn nil, fmt.Errorf("add operation: Git HEAD changed while boundary evidence was being evaluated; base=%s current=%s", baseSHA, currentSHA)
\t}
\tdecision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{BaseSHA: baseSHA, Application: applicationKey, OperationID: options.OperationID}, before, after)
\tif err != nil {
\t\treturn nil, err
\t}
\treturn &decision, nil
}

func resolveBoundaryBaseSHA(root string) (string, error) {
\tcommand := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD^{commit}")
\toutput, err := command.CombinedOutput()
\tif err != nil {
\t\tdetail := strings.TrimSpace(string(output))
\t\tif detail == "" {
\t\t\tdetail = err.Error()
\t\t}
\t\treturn "", fmt.Errorf("resolve exact Git HEAD: %s", detail)
\t}
\tsha := strings.TrimSpace(string(output))
\tif !exactBoundaryCommitSHA(sha) {
\t\treturn "", fmt.Errorf("Git HEAD is not an exact lowercase commit SHA: %q", sha)
\t}
\treturn sha, nil
}

func exactBoundaryCommitSHA(value string) bool {
\tif len(value) != 40 && len(value) != 64 {
\t\treturn false
\t}
\tif strings.Trim(value, "0") == "" {
\t\treturn false
\t}
\tfor _, c := range value {
\t\tif (c < '0' || c > '9') && (c < 'a' || c > 'f') {
\t\t\treturn false
\t\t}
\t}
\treturn true
}''')

# Reject stale plan evidence explicitly before rebuilding. Exact full-plan
# equality remains the second line of defense for same-HEAD worktree drift.
replace_once(
    "app/cmd/add/plan_validation.go",
    '''\tif candidate.ExplicitSemantics == nil {
\t\treturn Report{}, fmt.Errorf("add operation plan: explicitSemantics are required")
\t}
\tidentity := candidate.Identity''',
    '''\tif candidate.ExplicitSemantics == nil {
\t\treturn Report{}, fmt.Errorf("add operation plan: explicitSemantics are required")
\t}
\tif candidate.BoundaryDecision == nil {
\t\treturn Report{}, fmt.Errorf("add operation plan: boundaryDecision is required")
\t}
\tcurrentSHA, err := resolveBoundaryBaseSHA(root)
\tif err != nil {
\t\treturn Report{}, fmt.Errorf("add operation plan: resolve current Git HEAD: %w", err)
\t}
\tif candidate.BoundaryDecision.BaseSHA != currentSHA {
\t\treturn Report{}, fmt.Errorf("add operation plan: stale boundary decision base; plan=%s current=%s", candidate.BoundaryDecision.BaseSHA, currentSHA)
\t}
\tidentity := candidate.Identity''')

# A create plan may be fresh against the current worktree yet target a different
# ChangeSet baseline. Require exact equality to the authoritative ChangeSet base.
replace_once(
    "app/cmd/change/changeset.go",
    '''\t\tplan, err := add.RevalidateOperationPlan(root, raw)
\t\tif err != nil {
\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: create plan %s: %w", path, err)}
\t\t}
\t\tcreate, err := createOperationChangeFromPlan(path, plan)''',
    '''\t\tplan, err := add.RevalidateOperationPlan(root, raw)
\t\tif err != nil {
\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: create plan %s: %w", path, err)}
\t\t}
\t\tif plan.BoundaryDecision == nil || plan.BoundaryDecision.BaseSHA != baseSHA {
\t\t\tplanBase := "<missing>"
\t\t\tif plan.BoundaryDecision != nil {
\t\t\t\tplanBase = plan.BoundaryDecision.BaseSHA
\t\t\t}
\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: create plan %s boundary decision base %s differs from ChangeSet base %s", path, planBase, baseSHA)}
\t\t}
\t\tcreate, err := createOperationChangeFromPlan(path, plan)''')

# Add test projects now model the real authoring precondition: an exact Git HEAD.
replace_once(
    "app/cmd/add/main_test.go",
    '''\t"os"
\t"path/filepath"''',
    '''\t"os"
\t"os/exec"
\t"path/filepath"''')
replace_once(
    "app/cmd/add/main_test.go",
    '''\tfor relative, contents := range files {
\t\tmustWriteFile(t, filepath.Join(root, filepath.FromSlash(relative)), contents)
\t}
\treturn root
}''',
    '''\tfor relative, contents := range files {
\t\tmustWriteFile(t, filepath.Join(root, filepath.FromSlash(relative)), contents)
\t}
\tmustGit(t, root, "init", "-q")
\tmustGit(t, root, "config", "user.name", "Yunka Test")
\tmustGit(t, root, "config", "user.email", "yunka-test@example.invalid")
\tmustGit(t, root, "add", "-A")
\tmustGit(t, root, "commit", "-q", "-m", "baseline")
\treturn root
}

func mustGit(t *testing.T, root string, args ...string) string {
\tt.Helper()
\tcommand := exec.Command("git", append([]string{"-C", root}, args...)...)
\toutput, err := command.CombinedOutput()
\tif err != nil {
\t\tt.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
\t}
\treturn strings.TrimSpace(string(output))
}''')

# Explicit stale-HEAD regression. An empty commit changes identity without changing
# project bytes, proving the decision is Git-base-bound rather than tree-only.
replace_once(
    "app/cmd/add/boundary_gate_test.go",
    '''func TestOperationGrowthGateBlocksSecondOperationWithoutBoundaryIntent''',
    '''func TestOperationPlanBoundaryDecisionIsBoundToExactGitHead(t *testing.T) {
\troot := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")})
\toptions := OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.read", UseCase: "read_tenant", Access: "public", Tenant: "optional", Transaction: "read_only", Idempotency: "none", Composition: "none", BoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant"}
\tplan, err := PlanOperation(options)
\tif err != nil { t.Fatal(err) }
\twant := mustGit(t, root, "rev-parse", "HEAD")
\tif plan.BoundaryDecision == nil || plan.BoundaryDecision.BaseSHA != want {
\t\tt.Fatalf("boundary decision base=%#v want=%s", plan.BoundaryDecision, want)
\t}
\tmustGit(t, root, "commit", "--allow-empty", "-q", "-m", "advance head without changing tree")
\tif _, err := RevalidateOperationPlan(root, plan); err == nil || !strings.Contains(err.Error(), "stale boundary decision base") {
\t\tt.Fatalf("stale HEAD-bound plan escaped revalidation: %v", err)
\t}
}

func TestOperationGrowthGateBlocksSecondOperationWithoutBoundaryIntent''')

# A fresh plan on current HEAD must not be accepted into a ChangeSet whose
# authoritative base is an earlier commit.
replace_once(
    "app/cmd/change/changeset_test.go",
    '''func TestBuildChangeSetRejectsDuplicateSubjectOperation''',
    '''func TestBuildChangeSetRejectsCreatePlanBoundToDifferentBase(t *testing.T) {
\tfixture := newPressureFixture(t)
\tplan := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")
\t_, _, err := BuildChangeSet(fixture.Root, "HEAD^", nil, []string{plan})
\tif err == nil || !strings.Contains(err.Error(), "boundary decision base") || !strings.Contains(err.Error(), "differs from ChangeSet base") {
\t\tt.Fatalf("mismatched create-plan base escaped ChangeSet binding: %v", err)
\t}
}

func TestBuildChangeSetRejectsDuplicateSubjectOperation''')

print("hardclose1 exact Git base binding prepared")
