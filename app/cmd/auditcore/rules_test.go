package auditcore

import "testing"

func TestEvaluateSourceProducesOnlyCanonicalDirectImportViolations(t *testing.T) {
	snapshot := SourceSnapshot{
		SourceRoot: "internal",
		Files: []GoSourceFile{
			{
				Path:    "internal/tenant/application/service.go",
				Package: "application",
				Imports: []string{
					"example.com/demo/internal/device/ports",
					"github.com/hvritual/yunka.io/framework/platform",
					"github.com/hvritual/yunka.io/gateway/authz",
				},
			},
			{
				Path:    "internal/tenant/application/local.go",
				Package: "application",
				Imports: []string{"example.com/demo/internal/tenant/ports"},
			},
			{
				Path:      "internal/tenant/application/generated.go",
				Package:   "application",
				Generated: true,
				Imports:   []string{"github.com/hvritual/yunka.io/framework/platform"},
			},
			{
				Path:    "internal/tenant/application/service_test.go",
				Package: "application",
				Test:    true,
				Imports: []string{"github.com/hvritual/yunka.io/gateway/authz"},
			},
			{
				Path:    "internal/unknown/application/service.go",
				Package: "application",
				Imports: []string{"github.com/hvritual/yunka.io/framework/platform"},
			},
		},
	}

	findings := EvaluateSource(snapshot, RuleOptions{
		GoModule:        "example.com/demo",
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant", "device"},
	})
	if len(findings) != 3 {
		t.Fatalf("findings=%d want=3: %#v", len(findings), findings)
	}
	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.Rule] = true
		if finding.Class != FindingProvenViolation {
			t.Fatalf("finding %s class=%s", finding.ID, finding.Class)
		}
		if len(finding.Evidence) < 2 {
			t.Fatalf("finding %s missing canonical/source evidence: %#v", finding.ID, finding.Evidence)
		}
	}
	for _, rule := range []string{RuleCrossDomainRepositoryBypass, RulePlatformProviderBypass, RuleAuthorizationBypass} {
		if !seen[rule] {
			t.Fatalf("missing rule %s: %#v", rule, findings)
		}
	}
}

func TestEvaluateSourceRecognizesQualifiedConsumerModuleAliases(t *testing.T) {
	snapshot := SourceSnapshot{
		SourceRoot: "internal",
		Files: []GoSourceFile{{
			Path:    "internal/access/application/service.go",
			Package: "application",
			Imports: []string{
				"github.com/hvritual/biz/internal/deviceops/ports",
				"yunka.io/framework/platform",
				"yunka.io/gateway/authz",
			},
		}},
	}
	findings := EvaluateSource(snapshot, RuleOptions{
		GoModule:        "github.com/hvritual/biz",
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"access", "deviceops"},
	})
	if len(findings) != 3 {
		t.Fatalf("findings=%d want=3: %#v", len(findings), findings)
	}
	seen := map[string]bool{}
	for _, finding := range findings {
		seen[finding.Rule] = true
	}
	for _, rule := range []string{RuleCrossDomainRepositoryBypass, RulePlatformProviderBypass, RuleAuthorizationBypass} {
		if !seen[rule] {
			t.Fatalf("qualified consumer alias did not produce %s: %#v", rule, findings)
		}
	}
}

func TestEvaluateSourceDoesNotInferArbitrarySuffixAsYunkaModule(t *testing.T) {
	snapshot := SourceSnapshot{
		SourceRoot: "internal",
		Files: []GoSourceFile{{
			Path:    "internal/tenant/application/service.go",
			Package: "application",
			Imports: []string{
				"example.org/not-yunka/framework/platform",
				"example.org/not-yunka/gateway/authz",
			},
		}},
	}
	findings := EvaluateSource(snapshot, RuleOptions{
		GoModule:        "example.com/demo",
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant"},
	})
	if len(findings) != 0 {
		t.Fatalf("arbitrary import suffix was inferred as Yunka architecture evidence: %#v", findings)
	}
}

func TestEvaluateSourceDoesNotInferUndeclaredDomainBoundary(t *testing.T) {
	snapshot := SourceSnapshot{
		SourceRoot: "internal",
		Files: []GoSourceFile{{
			Path:    "internal/tenant/application/service.go",
			Package: "application",
			Imports: []string{"example.com/demo/internal/device/ports"},
		}},
	}
	findings := EvaluateSource(snapshot, RuleOptions{
		GoModule:        "example.com/demo",
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant"},
	})
	if len(findings) != 0 {
		t.Fatalf("undeclared target domain was inferred as architecture fact: %#v", findings)
	}
}
