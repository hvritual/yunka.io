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
    '''\t"fmt"\n\t"strings"''',
    '''\t"fmt"\n\t"os/exec"\n\t"strings"''')
replace_once(
    "app/cmd/add/boundary_gate.go",
    '''func evaluateOperationBoundary(root, sourcePath, domain, application, packageName, rpcName, requestType, responseType string, options OperationOptions) (*OperationBoundaryDecision, error) {\n\tsnapshot, err := projectflow.DescribeContractSourceSnapshot''',
    '''func evaluateOperationBoundary(root, sourcePath, domain, application, packageName, rpcName, requestType, responseType string, options OperationOptions) (*OperationBoundaryDecision, error) {\n\tbaseSHA, err := resolveBoundaryBaseSHA(root)\n\tif err != nil {\n\t\treturn nil, fmt.Errorf("add operation: bind boundary decision to Git HEAD: %w", err)\n\t}\n\tsnapshot, err := projectflow.DescribeContractSourceSnapshot''')
replace_once(
    "app/cmd/add/boundary_gate.go",
    '''\tdecision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{Application: applicationKey, OperationID: options.OperationID}, before, after)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn &decision, nil\n}''',
    '''\tcurrentSHA, err := resolveBoundaryBaseSHA(root)\n\tif err != nil {\n\t\treturn nil, fmt.Errorf("add operation: re-read Git HEAD before boundary decision: %w", err)\n\t}\n\tif currentSHA != baseSHA {\n\t\treturn nil, fmt.Errorf("add operation: Git HEAD changed while boundary evidence was being evaluated; base=%s current=%s", baseSHA, currentSHA)\n\t}\n\tdecision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{BaseSHA: baseSHA, Application: applicationKey, OperationID: options.OperationID}, before, after)\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\treturn &decision, nil\n}\n\nfunc resolveBoundaryBaseSHA(root string) (string, error) {\n\tcommand := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD^{commit}")\n\toutput, err := command.CombinedOutput()\n\tif err != nil {\n\t\tdetail := strings.TrimSpace(string(output))\n\t\tif detail == "" {\n\t\t\tdetail = err.Error()\n\t\t}\n\t\treturn "", fmt.Errorf("resolve exact Git HEAD: %s", detail)\n\t}\n\tsha := strings.TrimSpace(string(output))\n\tif !exactBoundaryCommitSHA(sha) {\n\t\treturn "", fmt.Errorf("Git HEAD is not an exact lowercase commit SHA: %q", sha)\n\t}\n\treturn sha, nil\n}\n\nfunc exactBoundaryCommitSHA(value string) bool {\n\tif len(value) != 40 && len(value) != 64 {\n\t\treturn false\n\t}\n\tif strings.Trim(value, "0") == "" {\n\t\treturn false\n\t}\n\tfor _, c := range value {\n\t\tif (c < '0' || c > '9') && (c < 'a' || c > 'f') {\n\t\t\treturn false\n\t\t}\n\t}\n\treturn true\n}''')

# Reject stale plan evidence explicitly before rebuilding. Exact full-plan
# equality remains the second line of defense for same-HEAD worktree drift.
replace_once(
    "app/cmd/add/plan_validation.go",
    '''\tif candidate.ExplicitSemantics == nil {\n\t\treturn Report{}, fmt.Errorf("add operation plan: explicitSemantics are required")\n\t}\n\tidentity := candidate.Identity''',
    '''\tif candidate.ExplicitSemantics == nil {\n\t\treturn Report{}, fmt.Errorf("add operation plan: explicitSemantics are required")\n\t}\n\tif candidate.BoundaryDecision == nil {\n\t\treturn Report{}, fmt.Errorf("add operation plan: boundaryDecision is required")\n\t}\n\tcurrentSHA, err := resolveBoundaryBaseSHA(root)\n\tif err != nil {\n\t\treturn Report{}, fmt.Errorf("add operation plan: resolve current Git HEAD: %w", err)\n\t}\n\tif candidate.BoundaryDecision.BaseSHA != currentSHA {\n\t\treturn Report{}, fmt.Errorf("add operation plan: stale boundary decision base; plan=%s current=%s", candidate.BoundaryDecision.BaseSHA, currentSHA)\n\t}\n\tidentity := candidate.Identity''')

# A create plan may be fresh against the current worktree yet target a different
# ChangeSet baseline. Require exact equality to the authoritative ChangeSet base.
replace_once(
    "app/cmd/change/changeset.go",
    '''\t\tplan, err = add.RevalidateOperationPlan(descriptor.Root, plan)\n\t\tif err != nil {\n\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: revalidate create plan %s: %w", input, err)}\n\t\t}\n\t\toperationID := strings.TrimSpace(plan.Identity["operationId"])''',
    '''\t\tplan, err = add.RevalidateOperationPlan(descriptor.Root, plan)\n\t\tif err != nil {\n\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: revalidate create plan %s: %w", input, err)}\n\t\t}\n\t\tif plan.BoundaryDecision == nil || plan.BoundaryDecision.BaseSHA != baseSHA {\n\t\t\tplanBase := "<missing>"\n\t\t\tif plan.BoundaryDecision != nil {\n\t\t\t\tplanBase = plan.BoundaryDecision.BaseSHA\n\t\t\t}\n\t\t\treturn ChangeSet{}, "", &Failure{Kind: FailureEvidence, Err: fmt.Errorf("change set begin: create plan %s boundary decision base %s differs from ChangeSet base %s", input, planBase, baseSHA)}\n\t\t}\n\t\toperationID := strings.TrimSpace(plan.Identity["operationId"])''')

# Add test projects now model the real authoring precondition: an exact Git HEAD.
replace_once(
    "app/cmd/add/main_test.go",
    '''\t"os"\n\t"path/filepath"''',
    '''\t"os"\n\t"os/exec"\n\t"path/filepath"''')
replace_once(
    "app/cmd/add/main_test.go",
    '''\tfor relative, contents := range files {\n\t\tmustWriteFile(t, filepath.Join(root, filepath.FromSlash(relative)), contents)\n\t}\n\treturn root\n}''',
    '''\tfor relative, contents := range files {\n\t\tmustWriteFile(t, filepath.Join(root, filepath.FromSlash(relative)), contents)\n\t}\n\tmustGit(t, root, "init", "-q")\n\tmustGit(t, root, "config", "user.name", "Yunka Test")\n\tmustGit(t, root, "config", "user.email", "yunka-test@example.invalid")\n\tmustGit(t, root, "add", "-A")\n\tmustGit(t, root, "commit", "-q", "-m", "baseline")\n\treturn root\n}\n\nfunc mustGit(t *testing.T, root string, args ...string) string {\n\tt.Helper()\n\tcommand := exec.Command("git", append([]string{"-C", root}, args...)...)\n\toutput, err := command.CombinedOutput()\n\tif err != nil {\n\t\tt.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))\n\t}\n\treturn strings.TrimSpace(string(output))\n}''')

# Explicit stale-HEAD regression. An empty commit changes identity without changing
# project bytes, proving the decision is Git-base-bound rather than tree-only.
replace_once(
    "app/cmd/add/boundary_gate_test.go",
    '''func TestOperationGrowthGateBlocksSecondOperationWithoutBoundaryIntent''',
    '''func TestOperationPlanBoundaryDecisionIsBoundToExactGitHead(t *testing.T) {\n\troot := scaffoldProject(t, map[string]string{"contracts/proto/tenant.proto": typedApplicationProto("tenant", "tenant.v1", "lifecycle", "TenantLifecycleApplication")})\n\toptions := OperationOptions{Root: root, ApplicationKey: "tenant/lifecycle", OperationID: "tenant.read", UseCase: "read_tenant", Access: "public", Tenant: "optional", Transaction: "read_only", Idempotency: "none", Composition: "none", BoundaryContext: "tenant.lifecycle", BoundaryAggregate: "tenant"}\n\tplan, err := PlanOperation(options)\n\tif err != nil { t.Fatal(err) }\n\twant := mustGit(t, root, "rev-parse", "HEAD")\n\tif plan.BoundaryDecision == nil || plan.BoundaryDecision.BaseSHA != want {\n\t\tt.Fatalf("boundary decision base=%#v want=%s", plan.BoundaryDecision, want)\n\t}\n\tmustGit(t, root, "commit", "--allow-empty", "-q", "-m", "advance head without changing tree")\n\tif _, err := RevalidateOperationPlan(root, plan); err == nil || !strings.Contains(err.Error(), "stale boundary decision base") {\n\t\tt.Fatalf("stale HEAD-bound plan escaped revalidation: %v", err)\n\t}\n}\n\nfunc TestOperationGrowthGateBlocksSecondOperationWithoutBoundaryIntent''')

# A fresh plan on current HEAD must not be accepted into a ChangeSet whose
# authoritative base is an earlier commit.
replace_once(
    "app/cmd/change/changeset_test.go",
    '''func TestBuildChangeSetRejectsDuplicateSubjectOperation''',
    '''func TestBuildChangeSetRejectsCreatePlanBoundToDifferentBase(t *testing.T) {\n\tfixture := newPressureFixture(t)\n\tplan := writeCreatePlan(t, fixture, "tenant.archive", "archive_tenant")\n\t_, _, err := BuildChangeSet(fixture.Root, "HEAD^", nil, []string{plan})\n\tif err == nil || !strings.Contains(err.Error(), "boundary decision base") || !strings.Contains(err.Error(), "differs from ChangeSet base") {\n\t\tt.Fatalf("mismatched create-plan base escaped ChangeSet binding: %v", err)\n\t}\n}\n\nfunc TestBuildChangeSetRejectsDuplicateSubjectOperation''')

print("hardclose1 exact Git base binding prepared")
