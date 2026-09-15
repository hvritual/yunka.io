package change

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yunka.io/app/cmd/advisorcore"
	"yunka.io/app/cmd/auditcore"
)

const (
	QualityDebtSchemaVersion   = 1
	QualityWaiverSchemaVersion = 1
	DefaultQualityWaiverPath   = ".git/yunka/engineering-quality-waivers.json"
)

// QualityDebtProof is a read-only projection over the canonical deterministic
// Audit debt and optional advisory semantic delta. It is not a third finding
// engine and never rewrites either source of truth.
type QualityDebtProof struct {
	SchemaVersion     int                                  `json:"schemaVersion"`
	Deterministic     auditcore.DebtDelta                  `json:"deterministic"`
	Advisory          *advisorcore.SemanticFindingDelta    `json:"advisory,omitempty"`
	BlockingNew       []auditcore.Finding                  `json:"blockingNew"`
	WaivedBlocking    []QualityWaiverApplication           `json:"waivedBlocking"`
	UnwaivedBlocking  []auditcore.Finding                  `json:"unwaivedBlocking"`
	WaiverSetSHA256   string                               `json:"waiverSetSha256,omitempty"`
	ProofSHA256       string                               `json:"proofSha256"`
}

type QualityWaiverScope struct {
	BaseSHA       string `json:"baseSha"`
	HeadSHA       string `json:"headSha"`
	FindingID     string `json:"findingId"`
	FindingSHA256 string `json:"findingSha256"`
	Rule          string `json:"rule"`
	Path          string `json:"path"`
}

type QualityWaiver struct {
	ID              string             `json:"id"`
	Owner           string             `json:"owner"`
	Reason          string             `json:"reason"`
	Scope           QualityWaiverScope `json:"scope"`
	ExpiresAt       string             `json:"expiresAt"`
	ReviewCondition string             `json:"reviewCondition"`
}

type QualityWaiverSet struct {
	SchemaVersion int             `json:"schemaVersion"`
	Waivers       []QualityWaiver `json:"waivers"`
	SetSHA256     string          `json:"setSha256"`
}

type QualityWaiverApplication struct {
	WaiverID        string `json:"waiverId"`
	FindingID       string `json:"findingId"`
	Owner           string `json:"owner"`
	Reason          string `json:"reason"`
	ExpiresAt       string `json:"expiresAt"`
	ReviewCondition string `json:"reviewCondition"`
}

type qualityDebtDigestPayload struct {
	SchemaVersion    int                               `json:"schemaVersion"`
	Deterministic    auditcore.DebtDelta               `json:"deterministic"`
	Advisory         *advisorcore.SemanticFindingDelta `json:"advisory,omitempty"`
	BlockingNew      []auditcore.Finding               `json:"blockingNew"`
	WaivedBlocking   []QualityWaiverApplication        `json:"waivedBlocking"`
	UnwaivedBlocking []auditcore.Finding               `json:"unwaivedBlocking"`
	WaiverSetSHA256  string                            `json:"waiverSetSha256,omitempty"`
}

type qualityWaiverSetPayload struct {
	SchemaVersion int             `json:"schemaVersion"`
	Waivers       []QualityWaiver `json:"waivers"`
}

func BuildQualityDebtProof(debt auditcore.DebtDelta, advisory *advisorcore.SemanticFindingDelta, waivers *QualityWaiverSet, baseSHA, headSHA string, now time.Time) (QualityDebtProof, error) {
	baseSHA = strings.TrimSpace(baseSHA)
	headSHA = strings.TrimSpace(headSHA)
	if baseSHA == "" || headSHA == "" {
		return QualityDebtProof{}, fmt.Errorf("quality debt proof: base and head SHA are required")
	}
	if debt.BaseSHA != baseSHA {
		return QualityDebtProof{}, fmt.Errorf("quality debt proof: deterministic baseline %s does not match change base %s", debt.BaseSHA, baseSHA)
	}
	blocking := blockingNewFindings(debt.New)
	proof := QualityDebtProof{
		SchemaVersion:    QualityDebtSchemaVersion,
		Deterministic:    debt,
		BlockingNew:      blocking,
		WaivedBlocking:   []QualityWaiverApplication{},
		UnwaivedBlocking: append([]auditcore.Finding(nil), blocking...),
	}
	if advisory != nil {
		encoded, err := advisorcore.MarshalSemanticFindingDelta(*advisory)
		if err != nil {
			return QualityDebtProof{}, fmt.Errorf("quality debt proof: advisory delta: %w", err)
		}
		decoded, err := advisorcore.DecodeSemanticFindingDelta(encoded)
		if err != nil {
			return QualityDebtProof{}, fmt.Errorf("quality debt proof: advisory delta: %w", err)
		}
		proof.Advisory = &decoded
	}
	if waivers != nil {
		normalized, err := canonicalQualityWaiverSet(*waivers, baseSHA, headSHA, blocking, now, true)
		if err != nil {
			return QualityDebtProof{}, err
		}
		proof.WaiverSetSHA256 = normalized.SetSHA256
		byFinding := make(map[string]QualityWaiver, len(normalized.Waivers))
		for _, waiver := range normalized.Waivers {
			byFinding[waiver.Scope.FindingID] = waiver
		}
		proof.UnwaivedBlocking = []auditcore.Finding{}
		for _, finding := range blocking {
			waiver, ok := byFinding[finding.ID]
			if !ok {
				proof.UnwaivedBlocking = append(proof.UnwaivedBlocking, finding)
				continue
			}
			proof.WaivedBlocking = append(proof.WaivedBlocking, QualityWaiverApplication{
				WaiverID: waiver.ID, FindingID: finding.ID, Owner: waiver.Owner, Reason: waiver.Reason,
				ExpiresAt: waiver.ExpiresAt, ReviewCondition: waiver.ReviewCondition,
			})
		}
	}
	normalizeQualityDebtProof(&proof)
	digest, err := qualityDebtProofDigest(proof)
	if err != nil {
		return QualityDebtProof{}, err
	}
	proof.ProofSHA256 = digest
	return proof, nil
}

func LoadQualityWaiverSet(root, input string, baseSHA, headSHA string, blocking []auditcore.Finding, now time.Time) (*QualityWaiverSet, error) {
	input = strings.TrimSpace(input)
	explicit := input != ""
	if input == "" {
		input = DefaultQualityWaiverPath
	}
	path, _, err := resolveGitPrivateStatePath(root, input, DefaultQualityWaiverPath)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) && !explicit {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("quality waiver: read %s: %w", input, err)
	}
	var set QualityWaiverSet
	if err := decodeStrictJSON(contents, &set); err != nil {
		return nil, fmt.Errorf("quality waiver: decode %s: %w", input, err)
	}
	normalized, err := canonicalQualityWaiverSet(set, baseSHA, headSHA, blocking, now, true)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

func WriteQualityWaiverSet(root, output string, set QualityWaiverSet) (string, error) {
	if strings.TrimSpace(output) == "" {
		output = DefaultQualityWaiverPath
	}
	path, display, err := resolveGitPrivateStatePath(root, output, DefaultQualityWaiverPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	contents, err := json.MarshalIndent(set, "", "  ")
	if err != nil {
		return "", err
	}
	contents = append(contents, '\n')
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		return "", err
	}
	return display, nil
}

func NewQualityWaiverSet(baseSHA, headSHA, owner, reason, expiresAt, reviewCondition string, findings []auditcore.Finding, findingIDs []string, now time.Time) (QualityWaiverSet, error) {
	owner = strings.TrimSpace(owner)
	reason = strings.TrimSpace(reason)
	expiresAt = strings.TrimSpace(expiresAt)
	reviewCondition = strings.TrimSpace(reviewCondition)
	if owner == "" || reason == "" || expiresAt == "" || reviewCondition == "" {
		return QualityWaiverSet{}, fmt.Errorf("quality waiver: owner, reason, expiresAt, and reviewCondition are required")
	}
	blocking := blockingNewFindings(findings)
	index := make(map[string]auditcore.Finding, len(blocking))
	for _, finding := range blocking {
		index[finding.ID] = finding
	}
	ids := uniqueSortedStrings(findingIDs)
	if len(ids) == 0 {
		return QualityWaiverSet{}, fmt.Errorf("quality waiver: at least one blocking new finding id is required")
	}
	set := QualityWaiverSet{SchemaVersion: QualityWaiverSchemaVersion, Waivers: []QualityWaiver{}}
	for _, id := range ids {
		finding, ok := index[id]
		if !ok {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: finding %s is not a current blocking new finding", id)
		}
		waiver := QualityWaiver{
			Owner: owner,
			Reason: reason,
			Scope: QualityWaiverScope{
				BaseSHA: baseSHA, HeadSHA: headSHA, FindingID: finding.ID,
				FindingSHA256: qualityFindingDigest(finding), Rule: finding.Rule, Path: qualityFindingPath(finding),
			},
			ExpiresAt: expiresAt, ReviewCondition: reviewCondition,
		}
		waiver.ID = qualityWaiverID(waiver)
		set.Waivers = append(set.Waivers, waiver)
	}
	return canonicalQualityWaiverSet(set, baseSHA, headSHA, blocking, now, false)
}

func canonicalQualityWaiverSet(set QualityWaiverSet, baseSHA, headSHA string, blocking []auditcore.Finding, now time.Time, verifyDigest bool) (QualityWaiverSet, error) {
	if set.SchemaVersion != QualityWaiverSchemaVersion {
		return QualityWaiverSet{}, fmt.Errorf("quality waiver: unsupported schemaVersion %d", set.SchemaVersion)
	}
	set.SetSHA256 = strings.TrimSpace(set.SetSHA256)
	index := make(map[string]auditcore.Finding, len(blocking))
	for _, finding := range blocking {
		index[finding.ID] = finding
	}
	seenIDs := map[string]struct{}{}
	seenFindings := map[string]struct{}{}
	for i := range set.Waivers {
		waiver := &set.Waivers[i]
		waiver.ID = strings.TrimSpace(waiver.ID)
		waiver.Owner = strings.TrimSpace(waiver.Owner)
		waiver.Reason = strings.TrimSpace(waiver.Reason)
		waiver.ExpiresAt = strings.TrimSpace(waiver.ExpiresAt)
		waiver.ReviewCondition = strings.TrimSpace(waiver.ReviewCondition)
		waiver.Scope.BaseSHA = strings.TrimSpace(waiver.Scope.BaseSHA)
		waiver.Scope.HeadSHA = strings.TrimSpace(waiver.Scope.HeadSHA)
		waiver.Scope.FindingID = strings.TrimSpace(waiver.Scope.FindingID)
		waiver.Scope.FindingSHA256 = strings.TrimSpace(waiver.Scope.FindingSHA256)
		waiver.Scope.Rule = strings.TrimSpace(waiver.Scope.Rule)
		waiver.Scope.Path = cleanProjectPath(waiver.Scope.Path)
		if waiver.Owner == "" || waiver.Reason == "" || waiver.ReviewCondition == "" {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver[%d] owner, reason, and reviewCondition are required", i)
		}
		if waiver.Scope.BaseSHA != baseSHA || waiver.Scope.HeadSHA != headSHA {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver %s is stale for exact base/head %s/%s", waiver.ID, baseSHA, headSHA)
		}
		expiry, err := time.Parse(time.RFC3339, waiver.ExpiresAt)
		if err != nil {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver %s expiresAt must be RFC3339: %w", waiver.ID, err)
		}
		if !now.Before(expiry) {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver %s expired at %s", waiver.ID, waiver.ExpiresAt)
		}
		finding, ok := index[waiver.Scope.FindingID]
		if !ok {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver %s scope finding %s is not a current blocking new finding", waiver.ID, waiver.Scope.FindingID)
		}
		if waiver.Scope.Rule != finding.Rule || waiver.Scope.Path != qualityFindingPath(finding) || waiver.Scope.FindingSHA256 != qualityFindingDigest(finding) {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver %s scope no longer matches exact finding %s", waiver.ID, finding.ID)
		}
		expectedID := qualityWaiverID(*waiver)
		if waiver.ID != expectedID {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: waiver id %q does not match exact scope %q", waiver.ID, expectedID)
		}
		if _, duplicate := seenIDs[waiver.ID]; duplicate {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: duplicate waiver id %q", waiver.ID)
		}
		seenIDs[waiver.ID] = struct{}{}
		if _, duplicate := seenFindings[waiver.Scope.FindingID]; duplicate {
			return QualityWaiverSet{}, fmt.Errorf("quality waiver: multiple waivers target finding %q", waiver.Scope.FindingID)
		}
		seenFindings[waiver.Scope.FindingID] = struct{}{}
	}
	sort.Slice(set.Waivers, func(i, j int) bool { return set.Waivers[i].ID < set.Waivers[j].ID })
	if set.Waivers == nil {
		set.Waivers = []QualityWaiver{}
	}
	payload, err := json.Marshal(qualityWaiverSetPayload{SchemaVersion: set.SchemaVersion, Waivers: set.Waivers})
	if err != nil {
		return QualityWaiverSet{}, err
	}
	digest := digestRaw(payload)
	if verifyDigest && set.SetSHA256 != digest {
		return QualityWaiverSet{}, fmt.Errorf("quality waiver: setSha256 mismatch")
	}
	set.SetSHA256 = digest
	return set, nil
}

func qualityDebtProofDigest(proof QualityDebtProof) (string, error) {
	payload := qualityDebtDigestPayload{
		SchemaVersion: proof.SchemaVersion, Deterministic: proof.Deterministic, Advisory: proof.Advisory,
		BlockingNew: proof.BlockingNew, WaivedBlocking: proof.WaivedBlocking,
		UnwaivedBlocking: proof.UnwaivedBlocking, WaiverSetSHA256: proof.WaiverSetSHA256,
	}
	contents, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return digestRaw(contents), nil
}

func normalizeQualityDebtProof(proof *QualityDebtProof) {
	if proof == nil {
		return
	}
	sort.Slice(proof.BlockingNew, func(i, j int) bool { return proof.BlockingNew[i].ID < proof.BlockingNew[j].ID })
	sort.Slice(proof.UnwaivedBlocking, func(i, j int) bool { return proof.UnwaivedBlocking[i].ID < proof.UnwaivedBlocking[j].ID })
	sort.Slice(proof.WaivedBlocking, func(i, j int) bool { return proof.WaivedBlocking[i].FindingID < proof.WaivedBlocking[j].FindingID })
	if proof.BlockingNew == nil { proof.BlockingNew = []auditcore.Finding{} }
	if proof.UnwaivedBlocking == nil { proof.UnwaivedBlocking = []auditcore.Finding{} }
	if proof.WaivedBlocking == nil { proof.WaivedBlocking = []QualityWaiverApplication{} }
}

func blockingNewFindings(values []auditcore.Finding) []auditcore.Finding {
	result := make([]auditcore.Finding, 0, len(values))
	for _, finding := range values {
		if finding.Class == auditcore.FindingProvenViolation && finding.Blocking {
			result = append(result, finding)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func qualityFindingPath(finding auditcore.Finding) string {
	if path := cleanProjectPath(finding.Path); path != "" {
		return path
	}
	return architectureDebtFindingPath(finding)
}

func qualityFindingDigest(finding auditcore.Finding) string {
	contents, _ := json.Marshal(finding)
	return digestRaw(contents)
}

func qualityWaiverID(waiver QualityWaiver) string {
	payload := struct {
		Owner string `json:"owner"`
		Reason string `json:"reason"`
		Scope QualityWaiverScope `json:"scope"`
		ExpiresAt string `json:"expiresAt"`
		ReviewCondition string `json:"reviewCondition"`
	}{waiver.Owner, waiver.Reason, waiver.Scope, waiver.ExpiresAt, waiver.ReviewCondition}
	contents, _ := json.Marshal(payload)
	return "waiver-" + digestRaw(contents)[:20]
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" { continue }
		if _, ok := seen[value]; ok { continue }
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func digestRaw(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
