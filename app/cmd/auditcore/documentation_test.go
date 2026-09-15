package auditcore

import "testing"

func TestEvaluateDocumentationReportsMissingPackageAndInterfaceDocs(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path:    "internal/tenant/ports/repository.go",
		Package: "ports",
		Declarations: []SourceDeclaration{{
			Kind:     "type",
			Name:     "TenantRepository",
			Contract: true,
		}},
	}}}
	findings := evaluateDocumentation(snapshot, RuleOptions{
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant"},
	})
	if len(findings) != 2 {
		t.Fatalf("findings=%d want=2: %#v", len(findings), findings)
	}
	seen := map[string]Finding{}
	for _, finding := range findings {
		seen[finding.Rule] = finding
		if finding.Class != FindingProvenViolation {
			t.Fatalf("documentation finding %s class=%s", finding.ID, finding.Class)
		}
		if finding.Path == "" || finding.Symbol == "" || finding.Reason == "" || finding.Remediation == "" {
			t.Fatalf("documentation finding lacks structured evidence: %#v", finding)
		}
	}
	if _, ok := seen[RuleMissingPackageDocumentation]; !ok {
		t.Fatalf("missing package-documentation finding: %#v", findings)
	}
	if _, ok := seen[RuleMissingContractDocumentation]; !ok {
		t.Fatalf("missing interface-contract finding: %#v", findings)
	}
}

func TestEvaluateDocumentationAcceptsDiscoverablePackageAndInterfaceDocs(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path:              "internal/tenant/ports/doc.go",
		Package:           "ports",
		PackageDocumented: true,
	}, {
		Path:    "internal/tenant/ports/repository.go",
		Package: "ports",
		Declarations: []SourceDeclaration{{
			Kind:       "type",
			Name:       "TenantRepository",
			Contract:   true,
			Documented: true,
		}},
	}}}
	findings := evaluateDocumentation(snapshot, RuleOptions{
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant"},
	})
	if len(findings) != 0 {
		t.Fatalf("documented package produced findings: %#v", findings)
	}
}

func TestEvaluateDocumentationDoesNotUseTestOrGeneratedDocsAsHandwrittenPackageProof(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path:    "internal/tenant/domain/tenant.go",
		Package: "domain",
	}, {
		Path:              "internal/tenant/domain/tenant_test.go",
		Package:           "domain",
		Test:              true,
		PackageDocumented: true,
	}, {
		Path:              "internal/tenant/domain/zz_yunka_entity_gen.go",
		Package:           "domain",
		Generated:         true,
		PackageDocumented: true,
	}}}
	findings := evaluateDocumentation(snapshot, RuleOptions{
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant"},
	})
	if len(findings) != 1 || findings[0].Rule != RuleMissingPackageDocumentation {
		t.Fatalf("test/generated docs unexpectedly satisfied developer-owned package docs: %#v", findings)
	}
}

func TestEvaluateDocumentationSkipsGeneratedOnlyAndUndeclaredPackages(t *testing.T) {
	snapshot := SourceSnapshot{Files: []GoSourceFile{{
		Path:      "internal/tenant/transport/zz_yunka_rpc_gen.go",
		Package:   "transport",
		Generated: true,
	}, {
		Path:    "internal/legacy/domain/model.go",
		Package: "domain",
	}}}
	findings := evaluateDocumentation(snapshot, RuleOptions{
		GeneratedGoRoot: "internal",
		DeclaredDomains: []string{"tenant"},
	})
	if len(findings) != 0 {
		t.Fatalf("generated-only or undeclared package was governed: %#v", findings)
	}
}

func TestDocumentationFindingsParticipateInExistingDebtDelta(t *testing.T) {
	missing := SourceSnapshot{Files: []GoSourceFile{{
		Path:    "internal/tenant/domain/tenant.go",
		Package: "domain",
	}}}
	documented := SourceSnapshot{Files: []GoSourceFile{{
		Path:              "internal/tenant/domain/tenant.go",
		Package:           "domain",
		PackageDocumented: true,
	}}}
	options := RuleOptions{GeneratedGoRoot: "internal", DeclaredDomains: []string{"tenant"}}
	delta := CompareProvenFindings(
		evaluateDocumentation(missing, options),
		evaluateDocumentation(documented, options),
	)
	if len(delta.Existing) != 0 || len(delta.New) != 0 || len(delta.Fixed) != 1 {
		t.Fatalf("documentation debt delta=%#v", delta)
	}
	if delta.Fixed[0].Rule != RuleMissingPackageDocumentation {
		t.Fatalf("fixed rule=%s", delta.Fixed[0].Rule)
	}
}
