package auditcore

import (
	"path"
	"sort"
	"strings"
)

const (
	RuleCrossDomainRepositoryBypass = "AUDIT-APP-001"
	RulePlatformProviderBypass      = "AUDIT-INFRA-001"
	RuleAuthorizationBypass         = "AUDIT-AUTH-001"
)

var frameworkPlatformImports = map[string]struct{}{
	"github.com/hvritual/yunka.io/framework/platform": {},
	"yunka.io/framework/platform":                    {},
}

var gatewayAuthorizationImports = map[string]struct{}{
	"github.com/hvritual/yunka.io/gateway/authz": {},
	"yunka.io/gateway/authz":                    {},
}

type RuleOptions struct {
	GoModule        string
	GeneratedGoRoot string
	DeclaredDomains []string
	Limits          QualityLimits
}

func EvaluateSource(snapshot SourceSnapshot, options RuleOptions) []Finding {
	domains := stringSet(options.DeclaredDomains)
	goModule := strings.Trim(strings.TrimSpace(options.GoModule), "/")
	generatedRoot := cleanSlash(options.GeneratedGoRoot)
	findings := evaluateNaming(snapshot)
	findings = append(findings, evaluateDocumentation(snapshot, options)...)
	findings = append(findings, evaluateDeclaredLimits(snapshot, options.Limits)...)
	for _, file := range snapshot.Files {
		if file.Test || file.Generated {
			continue
		}
		sourceDomain, ok := applicationSourceDomain(file.Path, generatedRoot, domains)
		if !ok {
			continue
		}
		for _, importPath := range file.Imports {
			importPath = strings.TrimSpace(importPath)
			if targetDomain, boundary, ok := crossDomainRepositoryImport(importPath, goModule, generatedRoot, domains); ok && targetDomain != sourceDomain {
				reason := "application in domain " + sourceDomain + " imports " + targetDomain + " repository/persistence boundary directly"
				findings = append(findings, Finding{
					ID:          findingID(RuleCrossDomainRepositoryBypass, file.Path, importPath),
					Rule:        RuleCrossDomainRepositoryBypass,
					Class:       FindingProvenViolation,
					Subject:     sourceDomain,
					Summary:     "application imports another declared domain repository/persistence boundary directly",
					Invariant:   "local cross-Application composition must use generated typed child capabilities; direct cross-domain repository access is forbidden",
					Path:        cleanSlash(file.Path),
					Symbol:      importPath,
					Reason:      reason,
					Remediation: "replace the direct repository/persistence import with the declared generated typed child capability for the target Application boundary",
					Evidence: []Evidence{
						{Kind: EvidenceCanonical, Source: "contract.manifest", Detail: "source domain=" + sourceDomain + " target domain=" + targetDomain},
						{Kind: EvidenceSource, Source: "go.import", Path: file.Path, Detail: importPath + " boundary=" + boundary},
					},
				})
			}
			if _, ok := frameworkPlatformImports[importPath]; ok {
				reason := "application imports framework platform provider " + importPath + " directly"
				findings = append(findings, Finding{
					ID:          findingID(RulePlatformProviderBypass, file.Path, importPath),
					Rule:        RulePlatformProviderBypass,
					Class:       FindingProvenViolation,
					Subject:     sourceDomain,
					Summary:     "application imports the framework platform provider directly",
					Invariant:   "process infrastructure is App-owned and business Applications receive declared typed capabilities rather than provider factories",
					Path:        cleanSlash(file.Path),
					Symbol:      importPath,
					Reason:      reason,
					Remediation: "inject the declared typed capability at the Application boundary instead of importing the process-level provider factory",
					Evidence: []Evidence{
						{Kind: EvidenceCanonical, Source: "contract.manifest", Detail: "declared application domain=" + sourceDomain},
						{Kind: EvidenceSource, Source: "go.import", Path: file.Path, Detail: importPath},
					},
				})
			}
			if _, ok := gatewayAuthorizationImports[importPath]; ok {
				reason := "application imports canonical authorization implementation " + importPath + " directly"
				findings = append(findings, Finding{
					ID:          findingID(RuleAuthorizationBypass, file.Path, importPath),
					Rule:        RuleAuthorizationBypass,
					Class:       FindingProvenViolation,
					Subject:     sourceDomain,
					Summary:     "application imports the canonical authorization implementation directly",
					Invariant:   "authorization is evaluated at the root execution security boundary and business Applications must not repeat role/permission evaluation",
					Path:        cleanSlash(file.Path),
					Symbol:      importPath,
					Reason:      reason,
					Remediation: "remove the direct authorization implementation import and consume the root execution security decision through the declared boundary",
					Evidence: []Evidence{
						{Kind: EvidenceCanonical, Source: "contract.manifest", Detail: "declared application domain=" + sourceDomain},
						{Kind: EvidenceSource, Source: "go.import", Path: file.Path, Detail: importPath},
					},
				})
			}
		}
	}
	for index := range findings {
		findings[index].Evidence = normalizeEvidence(findings[index].Evidence)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].ID < findings[j].ID })
	if findings == nil {
		return []Finding{}
	}
	return findings
}

func applicationSourceDomain(filePath, generatedRoot string, declared map[string]struct{}) (string, bool) {
	filePath = cleanSlash(filePath)
	generatedRoot = cleanSlash(generatedRoot)
	prefix := generatedRoot + "/"
	if generatedRoot == "." {
		prefix = ""
	}
	if !strings.HasPrefix(filePath, prefix) {
		return "", false
	}
	relative := strings.TrimPrefix(filePath, prefix)
	parts := strings.Split(relative, "/")
	if len(parts) < 3 || parts[1] != "application" {
		return "", false
	}
	domain := strings.TrimSpace(parts[0])
	_, ok := declared[domain]
	return domain, ok
}

func crossDomainRepositoryImport(importPath, goModule, generatedRoot string, declared map[string]struct{}) (string, string, bool) {
	if goModule == "" {
		return "", "", false
	}
	generatedRoot = cleanSlash(generatedRoot)
	prefix := goModule + "/"
	if generatedRoot != "." {
		prefix += strings.Trim(generatedRoot, "/") + "/"
	}
	if !strings.HasPrefix(importPath, prefix) {
		return "", "", false
	}
	relative := strings.TrimPrefix(importPath, prefix)
	parts := strings.Split(relative, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	domain := strings.TrimSpace(parts[0])
	if _, ok := declared[domain]; !ok {
		return "", "", false
	}
	boundary := path.Clean(strings.Join(parts[1:], "/"))
	if boundary == "ports" || strings.HasPrefix(boundary, "ports/") || boundary == "infrastructure/persistence" || strings.HasPrefix(boundary, "infrastructure/persistence/") {
		return domain, boundary, true
	}
	return "", "", false
}

func findingID(rule, filePath, importPath string) string {
	return strings.TrimSpace(rule) + ":" + cleanSlash(filePath) + ":" + strings.TrimSpace(importPath)
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result[value] = struct{}{}
		}
	}
	return result
}
