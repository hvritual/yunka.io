package auditcore

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	RuleHistoricalSourceIdentity = "AUDIT-NAME-001"
	RuleGenericContainerCohesion = "AUDIT-NAME-002"
)

var historicalTaskPrefix = regexp.MustCompile(`(?i)^(?:(?:ag|ax|cg|ce|yu)[_-]?\d+(?:[._-]\d+)*|ec[_-]?ri[_-]?\d+(?:[._-]\d+)*|(?:b|c)\d+(?:[._-]\d+)*)`)
var numberedHistoryPrefix = regexp.MustCompile(`(?i)^(?:round|wave|phase|stage|task|tmp|final|old)[_-]?\d+`)

var genericContainerNames = map[string]struct{}{
	"model": {}, "types": {}, "common": {}, "utils": {}, "helper": {}, "misc": {}, "manager": {}, "processor": {}, "data": {},
}

func evaluateNaming(snapshot SourceSnapshot) []Finding {
	var findings []Finding
	packageFiles := map[string][]GoSourceFile{}
	for _, file := range snapshot.Files {
		if file.Generated {
			continue
		}
		packageKey := filepath.ToSlash(filepath.Dir(file.Path)) + "\x00" + file.Package
		packageFiles[packageKey] = append(packageFiles[packageKey], file)

		if file.Exception == "" {
			if reason := historicalIdentityReason("file", sourceFileIdentity(file.Path)); reason != "" {
				findings = append(findings, historicalIdentityFinding(file.Path, "file", sourceFileIdentity(file.Path), reason))
			}
			if !file.Test {
				if finding, ok := genericContainerFinding(file); ok {
					findings = append(findings, finding)
				}
			}
		}
		for _, declaration := range file.Declarations {
			if declaration.Exception != "" {
				continue
			}
			identity := declaration.Name
			if declaration.Kind == "test" {
				identity = durableTestSubject(identity)
			}
			if reason := historicalIdentityReason(declaration.Kind, identity); reason != "" {
				symbol := declaration.Name
				if declaration.Receiver != "" {
					symbol = declaration.Receiver + "." + declaration.Name
				}
				findings = append(findings, historicalIdentityFinding(file.Path, declaration.Kind, symbol, reason))
			}
		}
	}

	for _, files := range packageFiles {
		sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
		if len(files) == 0 {
			continue
		}
		exempt := false
		for _, file := range files {
			if file.Exception != "" {
				exempt = true
				break
			}
		}
		if exempt {
			continue
		}
		if reason := historicalIdentityReason("package", files[0].Package); reason != "" {
			findings = append(findings, historicalIdentityFinding(files[0].Path, "package", files[0].Package, reason))
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

func historicalIdentityReason(kind, identity string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	if match := historicalTaskPrefix.FindString(identity); match != "" {
		return "identity starts with delivery-history token " + match
	}
	if match := numberedHistoryPrefix.FindString(identity); match != "" {
		return "identity starts with numbered delivery-history label " + match
	}
	if kind == "file" || kind == "package" {
		switch leadingIdentityWord(identity) {
		case "round", "wave", "phase", "stage", "task", "tmp", "final", "old":
			return "identity is led by a delivery-history label rather than durable domain or technical semantics"
		}
	}
	return ""
}

func historicalIdentityFinding(path, kind, symbol, reason string) Finding {
	path = cleanSlash(path)
	symbol = strings.TrimSpace(symbol)
	remediation := "rename the durable source identity to describe domain or technical responsibility; if the term is a genuine business concept or reviewed compatibility boundary, add a scoped yunka:audit-name-exception directive with a reason"
	return Finding{
		ID:          RuleHistoricalSourceIdentity + ":" + path + ":" + kind + ":" + symbol,
		Rule:        RuleHistoricalSourceIdentity,
		Class:       FindingProvenViolation,
		Subject:     path + "#" + symbol,
		Summary:     "durable source identity leaks delivery history",
		Invariant:   "production package, file, exported symbol and durable test identities must describe enduring domain or technical semantics rather than delivery history",
		Path:        path,
		Symbol:      symbol,
		Reason:      reason,
		Remediation: remediation,
		Evidence: []Evidence{
			{Kind: EvidenceCanonical, Source: "docs/ENGINEERING_QUALITY_RULES.md", Detail: "production identities describe semantics, not task history"},
			{Kind: EvidenceSource, Source: "go.identity", Path: path, Detail: "kind=" + kind + " identity=" + symbol + " reason=" + reason},
		},
	}
}

func genericContainerFinding(file GoSourceFile) (Finding, bool) {
	stem := sourceFileIdentity(file.Path)
	if _, ok := genericContainerNames[strings.ToLower(stem)]; !ok {
		return Finding{}, false
	}
	var typeNames []string
	rootCounts := map[string]int{}
	for _, declaration := range file.Declarations {
		if declaration.Kind != "type" {
			continue
		}
		typeNames = append(typeNames, declaration.Name)
		if root := leadingIdentityWord(declaration.Name); root != "" {
			rootCounts[root]++
		}
	}
	if len(typeNames) < 4 || len(rootCounts) < 3 {
		return Finding{}, false
	}
	maxRoot := 0
	var roots []string
	for root, count := range rootCounts {
		roots = append(roots, root)
		if count > maxRoot {
			maxRoot = count
		}
	}
	if maxRoot*2 >= len(typeNames) {
		return Finding{}, false
	}
	sort.Strings(typeNames)
	sort.Strings(roots)
	reason := "generic container owns " + itoa(len(typeNames)) + " exported types across " + itoa(len(roots)) + " distinct leading concept stems: " + strings.Join(roots, ", ")
	remediation := "review cohesion and split unrelated primary concepts into semantically named files; do not mechanically enforce one type per file"
	return Finding{
		ID:          RuleGenericContainerCohesion + ":" + cleanSlash(file.Path),
		Rule:        RuleGenericContainerCohesion,
		Class:       FindingEvidenceObservation,
		Subject:     cleanSlash(file.Path),
		Summary:     "generic container file is a cohesion-review candidate",
		Path:        cleanSlash(file.Path),
		Symbol:      strings.Join(typeNames, ","),
		Reason:      reason,
		Remediation: remediation,
		Evidence: []Evidence{
			{Kind: EvidenceCanonical, Source: "docs/ENGINEERING_QUALITY_RULES.md", Detail: "generic container names require explicit justification when they aggregate unrelated primary concepts"},
			{Kind: EvidenceSource, Source: "go.declarations", Path: cleanSlash(file.Path), Detail: reason},
		},
	}, true
}

func sourceFileIdentity(path string) string {
	name := strings.TrimSuffix(filepath.Base(cleanSlash(path)), filepath.Ext(path))
	name = strings.TrimSuffix(name, "_test")
	return strings.TrimSpace(name)
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	var digits [20]byte
	index := len(digits)
	for value > 0 {
		index--
		digits[index] = byte('0' + value%10)
		value /= 10
	}
	return string(digits[index:])
}
