package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConsumerDomainCoverageQualification(t *testing.T) {
	bizRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_BIZ_ROOT"))
	iotRoot := strings.TrimSpace(os.Getenv("YUNKA_QUALIFY_IOT_ROOT"))
	if bizRoot == "" && iotRoot == "" {
		t.Skip("qualification-only consumer roots are not configured")
	}
	if bizRoot == "" || iotRoot == "" {
		t.Fatal("both YUNKA_QUALIFY_BIZ_ROOT and YUNKA_QUALIFY_IOT_ROOT are required")
	}

	bizEntries, bizErr := ValidateCoverage(filepath.Join(bizRoot, "internal"))
	if bizErr == nil || !strings.Contains(bizErr.Error(), "UNMANAGED_DOMAIN_TOPOLOGY") {
		t.Fatalf("Biz coverage error=%v, want UNMANAGED_DOMAIN_TOPOLOGY", bizErr)
	}
	var accessUnknown bool
	for _, entry := range bizEntries {
		if entry.Domain == "access" && entry.State == CoverageUnknown {
			accessUnknown = true
			break
		}
	}
	if !accessUnknown {
		t.Fatalf("Biz access domain was not classified UNKNOWN: %#v", bizEntries)
	}

	iotEntries, err := ValidateCoverage(filepath.Join(iotRoot, "backend", "internal"))
	if err != nil {
		t.Fatalf("IoT Delivery coverage unexpectedly failed: %v", err)
	}
	for _, entry := range iotEntries {
		if entry.State == CoverageUnknown {
			t.Fatalf("IoT Delivery contains unexpected UNKNOWN coverage entry: %#v", entry)
		}
	}

	t.Logf("BIZ_COVERAGE=EXPECTED_FAIL entries=%d error=%q", len(bizEntries), bizErr.Error())
	t.Logf("IOT_COVERAGE=PASS entries=%d", len(iotEntries))
}
