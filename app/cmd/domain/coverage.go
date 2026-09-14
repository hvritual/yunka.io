package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DomainCoverageSchemaVersion = 1
	DomainCoverageRelativePath  = ".yunka/domain-coverage.json"

	CoverageManaged CoverageState = "MANAGED"
	CoverageExempt  CoverageState = "EXEMPT"
	CoverageUnknown CoverageState = "UNKNOWN"
)

// CoverageState describes whether a domain-like source surface is managed by
// the Domain compiler, explicitly exempt from it, or missing an ownership
// decision. UNKNOWN is never an accepted check state.
type CoverageState string

// CoverageExemption records the reason a domain-like source surface is
// intentionally outside Domain compiler ownership. It does not duplicate
// managed-domain identity: MANAGED remains derived from domain.json.
type CoverageExemption struct {
	Domain  string `json:"domain"`
	Reason  string `json:"reason"`
	Owner   string `json:"owner,omitempty"`
	Expires string `json:"expires,omitempty"`
}

// CoverageContract is the project-owned exception register for Domain
// governance. Managed domains are intentionally absent because domain.json is
// already their canonical ownership fact.
type CoverageContract struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Exemptions    []CoverageExemption `json:"exemptions,omitempty"`
}

// CoverageEntry is a derived read-only view of one domain-like source surface.
type CoverageEntry struct {
	Domain  string
	Root    string
	State   CoverageState
	Signals []string
	Reason  string
	Owner   string
	Expires string
}

var domainTopologySignals = []string{
	"application",
	"domain",
	"infrastructure",
	"policy",
	"ports",
	"transport",
}

// InspectCoverage derives Domain ownership from canonical source evidence.
// A domain.json means MANAGED. A domain-like tree without a manifest may be
// EXEMPT only when the project declares an exact exemption. Otherwise it is
// UNKNOWN and ValidateCoverage will fail closed.
func InspectCoverage(root string) ([]CoverageEntry, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		root = "internal"
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(absolute)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	moduleDirectory, _, err := findOwningGoModule(absolute)
	if err != nil {
		return nil, err
	}
	contract, err := loadCoverageContract(moduleDirectory)
	if err != nil {
		return nil, err
	}
	exemptions := make(map[string]CoverageExemption, len(contract.Exemptions))
	for _, exemption := range contract.Exemptions {
		exemptions[exemption.Domain] = exemption
	}

	result := make([]CoverageEntry, 0, len(entries))
	seenExemptions := make(map[string]struct{}, len(exemptions))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		domainRoot := filepath.Join(absolute, entry.Name())
		manifestPath := filepath.Join(domainRoot, ManifestName)
		_, manifestErr := os.Stat(manifestPath)
		managed := manifestErr == nil
		if manifestErr != nil && !os.IsNotExist(manifestErr) {
			return nil, manifestErr
		}
		signals, err := domainLikeSignals(domainRoot)
		if err != nil {
			return nil, err
		}
		if managed {
			if _, conflict := exemptions[entry.Name()]; conflict {
				return nil, fmt.Errorf("DOMAIN_COVERAGE_CONFLICT: domain %q has %s and an exemption; remove the exemption", entry.Name(), ManifestName)
			}
			result = append(result, CoverageEntry{Domain: entry.Name(), Root: domainRoot, State: CoverageManaged, Signals: signals})
			continue
		}
		if len(signals) == 0 {
			continue
		}
		if exemption, ok := exemptions[entry.Name()]; ok {
			seenExemptions[entry.Name()] = struct{}{}
			result = append(result, CoverageEntry{
				Domain: entry.Name(), Root: domainRoot, State: CoverageExempt, Signals: signals,
				Reason: exemption.Reason, Owner: exemption.Owner, Expires: exemption.Expires,
			})
			continue
		}
		result = append(result, CoverageEntry{Domain: entry.Name(), Root: domainRoot, State: CoverageUnknown, Signals: signals})
	}

	for domain := range exemptions {
		if _, seen := seenExemptions[domain]; !seen {
			return nil, fmt.Errorf("DOMAIN_EXEMPTION_TARGET_INVALID: exemption for %q does not match an existing unmanaged domain-like surface", domain)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Domain < result[j].Domain })
	return result, nil
}

// ValidateCoverage rejects every UNKNOWN domain-like surface and returns the
// complete derived inventory when the project has explicit ownership closure.
func ValidateCoverage(root string) ([]CoverageEntry, error) {
	entries, err := InspectCoverage(root)
	if err != nil {
		return nil, err
	}
	var failures []error
	for _, entry := range entries {
		if entry.State != CoverageUnknown {
			continue
		}
		failures = append(failures, fmt.Errorf(
			"UNMANAGED_DOMAIN_TOPOLOGY: domain %q has domain-like source structure (%s) but no %s and no exemption in %s",
			entry.Domain, strings.Join(entry.Signals, ","), ManifestName, DomainCoverageRelativePath,
		))
	}
	return entries, errors.Join(failures...)
}

func domainLikeSignals(root string) ([]string, error) {
	signals := make([]string, 0, len(domainTopologySignals))
	for _, relative := range domainTopologySignals {
		info, err := os.Stat(filepath.Join(root, relative))
		switch {
		case err == nil && info.IsDir():
			signals = append(signals, relative)
		case err == nil:
			continue
		case os.IsNotExist(err):
			continue
		default:
			return nil, err
		}
	}
	return signals, nil
}

func loadCoverageContract(moduleDirectory string) (CoverageContract, error) {
	path := filepath.Join(moduleDirectory, filepath.FromSlash(DomainCoverageRelativePath))
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return CoverageContract{SchemaVersion: DomainCoverageSchemaVersion, Exemptions: []CoverageExemption{}}, nil
	}
	if err != nil {
		return CoverageContract{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	var contract CoverageContract
	if err := decoder.Decode(&contract); err != nil {
		return CoverageContract{}, fmt.Errorf("domain: decode %s: %w", DomainCoverageRelativePath, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return CoverageContract{}, fmt.Errorf("domain: decode %s: %w", DomainCoverageRelativePath, err)
	}
	if err := validateCoverageContract(contract); err != nil {
		return CoverageContract{}, err
	}
	return contract, nil
}

func validateCoverageContract(contract CoverageContract) error {
	if contract.SchemaVersion != DomainCoverageSchemaVersion {
		return fmt.Errorf("domain: %s schemaVersion=%d is unsupported", DomainCoverageRelativePath, contract.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(contract.Exemptions))
	for index, exemption := range contract.Exemptions {
		domain := strings.TrimSpace(exemption.Domain)
		reason := strings.TrimSpace(exemption.Reason)
		owner := strings.TrimSpace(exemption.Owner)
		expires := strings.TrimSpace(exemption.Expires)
		if exemption.Domain != domain || exemption.Reason != reason || exemption.Owner != owner || exemption.Expires != expires {
			return fmt.Errorf("domain: %s exemption for %q contains non-canonical whitespace", DomainCoverageRelativePath, domain)
		}
		if domain == "" || domain == "." || domain == ".." || domain != filepath.Base(domain) || strings.ContainsAny(domain, "/\\") {
			return fmt.Errorf("domain: %s exemptions[%d].domain must be one direct domain directory name", DomainCoverageRelativePath, index)
		}
		if reason == "" {
			return fmt.Errorf("domain: %s exemption for %q requires a reason", DomainCoverageRelativePath, domain)
		}
		if _, duplicate := seen[domain]; duplicate {
			return fmt.Errorf("domain: %s contains duplicate exemption for %q", DomainCoverageRelativePath, domain)
		}
		seen[domain] = struct{}{}
		if expires != "" {
			if _, err := time.Parse("2006-01-02", expires); err != nil {
				return fmt.Errorf("domain: %s exemption for %q has invalid expires date %q; use YYYY-MM-DD", DomainCoverageRelativePath, domain, exemption.Expires)
			}
		}
	}
	return nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("unexpected trailing JSON value")
}
