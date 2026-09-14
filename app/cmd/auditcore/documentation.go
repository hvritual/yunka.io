package auditcore

import (
	"path"
	"sort"
	"strings"
)

const (
	RuleMissingPackageDocumentation  = "AUDIT-DOC-001"
	RuleMissingContractDocumentation = "AUDIT-DOC-002"
)

type governedPackage struct {
	Domain string
	Dir    string
	Name   string
	Files  []GoSourceFile
}

func evaluateDocumentation(snapshot SourceSnapshot, options RuleOptions) []Finding {
	domains := stringSet(options.DeclaredDomains)
	if len(domains) == 0 {
		return []Finding{}
	}
	generatedRoot := cleanSlash(options.GeneratedGoRoot)
	packages := map[string]*governedPackage{}
	for _, file := range snapshot.Files {
		if file.Generated || file.Test {
			continue
		}
		domain, ok := governedPackageDomain(file.Path, generatedRoot, domains)
		if !ok {
			continue
		}
		dir := path.Dir(cleanSlash(file.Path))
		key := dir + "\x00" + strings.TrimSpace(file.Package)
		entry := packages[key]
		if entry == nil {
			entry = &governedPackage{Domain: domain, Dir: dir, Name: strings.TrimSpace(file.Package)}
			packages[key] = entry
		}
		entry.Files = append(entry.Files, file)
	}

	keys := make([]string, 0, len(packages))
	for key := range packages {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var findings []Finding
	for _, key := range keys {
		entry := packages[key]
		sort.Slice(entry.Files, func(i, j int) bool { return entry.Files[i].Path < entry.Files[j].Path })
		if !packageHasDeveloperDocumentation(entry.Files) {
			findings = append(findings, missingPackageDocumentationFinding(*entry))
		}
		for _, file := range entry.Files {
			for _, declaration := range file.Declarations {
				if declaration.Contract && !declaration.Documented {
					findings = append(findings, missingContractDocumentationFinding(entry.Domain, file, declaration))
				}
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

func governedPackageDomain(filePath, generatedRoot string, declared map[string]struct{}) (string, bool) {
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
	if len(parts) < 2 {
		return "", false
	}
	domain := strings.TrimSpace(parts[0])
	_, ok := declared[domain]
	return domain, ok
}

func packageHasDeveloperDocumentation(files []GoSourceFile) bool {
	for _, file := range files {
		if !file.Generated && !file.Test && file.PackageDocumented {
			return true
		}
	}
	return false
}

func missingPackageDocumentationFinding(pkg governedPackage) Finding {
	pathValue := cleanSlash(pkg.Dir)
	representative := pathValue
	if len(pkg.Files) > 0 {
		representative = cleanSlash(pkg.Files[0].Path)
	}
	reason := "governed package has developer-owned production source but no developer-owned package documentation"
	remediation := "add package documentation, preferably in developer-owned doc.go, that makes responsibility, primary concepts, important invariants, and intentional exclusions discoverable; do not edit generated source to satisfy this rule"
	return Finding{
		ID:          RuleMissingPackageDocumentation + ":" + pathValue + ":package:" + pkg.Name,
		Rule:        RuleMissingPackageDocumentation,
		Class:       FindingProvenViolation,
		Subject:     pathValue + "#package:" + pkg.Name,
		Summary:     "governed package lacks developer-owned package documentation",
		Invariant:   "each governed package with developer-owned production source must expose discoverable package responsibility documentation; generated and test source do not satisfy the handwritten documentation requirement",
		Path:        pathValue,
		Symbol:      pkg.Name,
		Reason:      reason,
		Remediation: remediation,
		Evidence: []Evidence{
			{Kind: EvidenceCanonical, Source: "docs/ENGINEERING_QUALITY_RULES.md", Detail: "core packages document purpose, boundary and invariants"},
			{Kind: EvidenceCanonical, Source: "contract.manifest", Detail: "declared domain=" + pkg.Domain},
			{Kind: EvidenceSource, Source: "go.package", Path: representative, Detail: reason},
		},
	}
}

func missingContractDocumentationFinding(domain string, file GoSourceFile, declaration SourceDeclaration) Finding {
	pathValue := cleanSlash(file.Path)
	symbol := strings.TrimSpace(declaration.Name)
	reason := "exported interface is an objective Go contract surface and has no declaration documentation"
	remediation := "add a declaration comment that explains the interface responsibility and any non-obvious boundary, invariant, failure, concurrency, transaction, idempotency, or security semantics; comment quality remains a semantic review concern"
	return Finding{
		ID:          RuleMissingContractDocumentation + ":" + pathValue + ":interface:" + symbol,
		Rule:        RuleMissingContractDocumentation,
		Class:       FindingProvenViolation,
		Subject:     pathValue + "#" + symbol,
		Summary:     "exported interface contract lacks documentation",
		Invariant:   "exported interface contracts in governed developer-owned source must have discoverable declaration documentation; deterministic presence does not prove semantic comment quality",
		Path:        pathValue,
		Symbol:      symbol,
		Reason:      reason,
		Remediation: remediation,
		Evidence: []Evidence{
			{Kind: EvidenceCanonical, Source: "docs/ENGINEERING_QUALITY_RULES.md", Detail: "comments explain constraints, not syntax"},
			{Kind: EvidenceCanonical, Source: "contract.manifest", Detail: "declared domain=" + strings.TrimSpace(domain)},
			{Kind: EvidenceSource, Source: "go.declaration", Path: pathValue, Detail: "kind=interface symbol=" + symbol + " documented=false"},
		},
	}
}
