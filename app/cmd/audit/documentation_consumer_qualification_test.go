package audit

import (
	"os"
	"strings"
	"testing"

	"yunka.io/app/cmd/auditcore"
)

func TestConsumerDocumentationQualification(t *testing.T) {
	bizRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_BIZ_ROOT"))
	iotRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_IOT_ROOT"))
	if bizRoot == "" && iotRoot == "" {
		return
	}
	if bizRoot == "" || iotRoot == "" {
		t.Fatal("both YUNKA_QUALIFY_BIZ_ROOT and YUNKA_QUALIFY_IOT_ROOT are required")
	}

	qualifyDocumentationConsumer(t, "biz", bizRoot, "internal/commercial/domain/subscription", auditcore.RuleMissingPackageDocumentation)
	qualifyDocumentationConsumer(t, "iot-delivery", iotRoot, "backend-yunka/internal/delivery", auditcore.RuleMissingPackageDocumentation)
}

func qualifyDocumentationConsumer(t *testing.T, name, root, expectedPath, expectedRule string) {
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
		t.Fatalf("%s documentation audit is not deterministic", name)
	}
	if len(first.Source.Files) == 0 {
		t.Fatalf("%s qualification has empty source snapshot", name)
	}

	var documentationFindings int
	var expected bool
	for _, finding := range first.Findings {
		if !strings.HasPrefix(finding.Rule, "AUDIT-DOC-") {
			continue
		}
		documentationFindings++
		if finding.Path == "" || finding.Symbol == "" || finding.Reason == "" || finding.Remediation == "" {
			t.Fatalf("%s documentation finding lacks actionable fields: %#v", name, finding)
		}
		if finding.Rule == expectedRule && finding.Path == expectedPath {
			expected = true
		}
	}
	if documentationFindings == 0 {
		t.Fatalf("%s qualification did not exercise documentation policy", name)
	}
	if !expected {
		t.Fatalf("%s expected %s at %s; findings=%#v", name, expectedRule, expectedPath, first.Findings)
	}
	t.Logf("%s DOCUMENTATION files=%d findings=%d", name, len(first.Source.Files), documentationFindings)
}
