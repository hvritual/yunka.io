package audit

import (
	"os"
	"strings"
	"testing"

	"yunka.io/app/cmd/auditcore"
)

func TestConsumerCodeSemanticsQualification(t *testing.T) {
	bizRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_BIZ_ROOT"))
	iotRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_IOT_ROOT"))
	if bizRoot == "" && iotRoot == "" {
		return
	}
	if bizRoot == "" || iotRoot == "" {
		t.Fatal("both YUNKA_QUALIFY_BIZ_ROOT and YUNKA_QUALIFY_IOT_ROOT are required")
	}

	qualifyCodeSemanticsConsumer(t, "biz", bizRoot, auditcore.RuleMissingPackageDocumentation)
	qualifyCodeSemanticsConsumer(t, "iot-delivery", iotRoot, auditcore.RuleMissingPackageDocumentation)
}

func qualifyCodeSemanticsConsumer(t *testing.T, name, root, expectedRule string) {
	t.Helper()
	first, err := Build(root)
	if err != nil {
		t.Fatalf("%s first audit: %v", name, err)
	}
	second, err := Build(root)
	if err != nil {
		t.Fatalf("%s second audit: %v", name, err)
	}
	firstJSON, err := Render(first, "agent-json")
	if err != nil {
		t.Fatalf("%s first render: %v", name, err)
	}
	secondJSON, err := Render(second, "agent-json")
	if err != nil {
		t.Fatalf("%s second render: %v", name, err)
	}
	if firstJSON != secondJSON {
		t.Fatalf("%s deterministic code-semantics report is not byte-stable", name)
	}
	if len(first.Source.Files) == 0 {
		t.Fatalf("%s qualification has empty source snapshot", name)
	}
	if first.QualityPolicy.Path != auditcore.QualityPolicyRelativePath {
		t.Fatalf("%s quality policy path=%q", name, first.QualityPolicy.Path)
	}

	var expected bool
	for _, finding := range first.Findings {
		if finding.Rule == expectedRule {
			expected = true
		}
		if finding.Class != auditcore.FindingProvenViolation {
			continue
		}
		if finding.Path == "" || finding.Symbol == "" || finding.Reason == "" || finding.Remediation == "" {
			t.Fatalf("%s proven deterministic finding lacks actionable fields: %#v", name, finding)
		}
		if finding.Blocking {
			t.Fatalf("%s consumer has no local quality policy but finding became blocking: %#v", name, finding)
		}
	}
	if !expected {
		t.Fatalf("%s did not exercise expected rule %s; findings=%#v", name, expectedRule, first.Findings)
	}
	t.Logf("%s CODE-SEMANTICS files=%d findings=%d policy_present=%t", name, len(first.Source.Files), len(first.Findings), first.QualityPolicy.Present)
}
