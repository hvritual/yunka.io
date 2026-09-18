package change

import (
	"os"
	"strings"
	"testing"
	"time"

	"yunka.io/app/cmd/audit"
	"yunka.io/app/cmd/auditcore"
	"yunka.io/app/cmd/projectflow"
)

func TestQualityDebtConsumerQualification(t *testing.T) {
	bizRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALITY_DEBT_BIZ_ROOT"))
	iotRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALITY_DEBT_IOT_ROOT"))
	iotProtoPath := strings.TrimSpace(os.Getenv("YUNKA_QUALITY_DEBT_IOT_PROTO_PATH"))
	if bizRoot == "" || iotRoot == "" {
		return
	}
	cases := []struct {
		name       string
		root       string
		protoPaths []string
	}{
		{name: "biz", root: bizRoot},
		{name: "iot", root: iotRoot, protoPaths: []string{iotProtoPath}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.name == "iot" && iotProtoPath == "" {
				t.Fatal("YUNKA_QUALITY_DEBT_IOT_PROTO_PATH is required for the pinned IoT DSL include")
			}
			qualifyQualityDebtConsumer(t, testCase.root, testCase.protoPaths)
		})
	}
}

func qualifyQualityDebtConsumer(t *testing.T, root string, protoPaths []string) {
	t.Helper()
	options := projectflow.Options{Root: root, ProtoPaths: append([]string(nil), protoPaths...)}
	first, err := audit.BuildWithBaseOptions(options, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	second, err := audit.BuildWithBaseOptions(options, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := auditcore.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBytes, err := auditcore.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Fatal("same consumer HEAD did not produce deterministic Audit debt evidence")
	}
	if first.Debt == nil || len(first.Debt.New) != 0 || len(first.Debt.Fixed) != 0 {
		t.Fatalf("HEAD baseline debt=%#v", first.Debt)
	}

	sourcePath := ""
	for _, file := range first.Source.Files {
		if !file.Generated && !file.Test {
			sourcePath = file.Path
			break
		}
	}
	if sourcePath == "" {
		t.Fatal("consumer has no developer-owned production Go source")
	}
	headSHA, err := auditcore.ResolveGitCommit(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	finding := auditcore.Finding{
		ID: "consumer-quality:" + sourcePath,
		Rule: auditcore.RuleFileLineLimit,
		Class: auditcore.FindingProvenViolation,
		Blocking: true,
		Subject: sourcePath,
		Summary: "consumer qualification blocking finding",
		Invariant: "qualification-only finding proves generic debt handling",
		Path: sourcePath,
		Symbol: sourcePath,
		Reason: "qualification fixture",
		Remediation: "qualification fixture",
		Evidence: []auditcore.Evidence{{Kind: auditcore.EvidenceSource, Source: "consumer-qualification", Path: sourcePath, Detail: "qualification fixture"}},
	}
	debt := auditcore.DebtDelta{BaseRef: "HEAD", BaseSHA: headSHA, New: []auditcore.Finding{finding}}
	now := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	unwaived, err := BuildQualityDebtProof(debt, nil, nil, headSHA, headSHA, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(unwaived.UnwaivedBlocking) != 1 {
		t.Fatalf("unwaived proof=%#v", unwaived)
	}
	waiverSet, err := NewQualityWaiverSet(headSHA, headSHA, "qualification-owner", "consumer qualification", "2026-09-20T10:00:00Z", "review after qualification", debt.New, []string{finding.ID}, now)
	if err != nil {
		t.Fatal(err)
	}
	waived, err := BuildQualityDebtProof(debt, nil, &waiverSet, headSHA, headSHA, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(waived.WaivedBlocking) != 1 || len(waived.UnwaivedBlocking) != 0 {
		t.Fatalf("waived proof=%#v", waived)
	}
}
