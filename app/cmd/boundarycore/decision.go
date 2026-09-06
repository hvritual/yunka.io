package boundarycore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
)

// AdditionPolicyVersion identifies a deliberately narrow, deterministic policy.
// Changing critical dimensions or sufficiency rules requires a new version.
const AdditionPolicyVersion = "canonical-peer-addition/v1"

const (
	ReuseExistingApplication   = "reuse_existing_application"
	CreateNewApplication       = "create_new_application"
	ArchitectureReviewRequired = "architecture_review_required"
)

var ErrStaleBoundaryProof = errors.New("STALE_BOUNDARY_PROOF")

// AdditionRequest is caller-owned task identity, not a second service taxonomy.
// The caller must resolve BaseSHA to an immutable commit and compile Before and
// After from the intended inputs. This pure core does not read Git or source.
type AdditionRequest struct {
	BaseSHA     string `json:"baseSha"`
	Application string `json:"application"`
	OperationID string `json:"operationId"`
}

type EvidenceRef struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Side    string `json:"side"`
	Subject string `json:"subject"`
	Digest  string `json:"digest"`
}

type BoundaryDimension struct {
	Name            string   `json:"name"`
	Verdict         string   `json:"verdict"` // same, compatible, different, unknown
	Critical        bool     `json:"critical"`
	Reason          string   `json:"reason"`
	Evidence        []string `json:"evidence"`
	CounterEvidence []string `json:"counterEvidence"`
}

// ServiceBoundaryDecision is a reproducible policy result, NOT a signed proof,
// a mutation capability or a declaration of business-ontology correctness.
// Revalidation must recompute the complete decision; checking its digest alone
// cannot establish authority. Other authoring/ChangeSet gates are still required.
type ServiceBoundaryDecision struct {
	SchemaVersion        int                 `json:"schemaVersion"`
	Authority            string              `json:"authority"`
	PolicyVersion        string              `json:"policyVersion"`
	BaseSHA              string              `json:"baseSha"`
	OperationID          string              `json:"operationId"`
	TargetApplication    string              `json:"targetApplication"`
	TargetService        string              `json:"targetService"`
	OperationPlanDigest  string              `json:"operationPlanDigest"`
	BeforeFingerprint    string              `json:"beforeFingerprint"`
	AfterFingerprint     string              `json:"afterFingerprint"`
	BeforeManifestDigest string              `json:"beforeManifestDigest"`
	AfterManifestDigest  string              `json:"afterManifestDigest"`
	Outcome              string              `json:"outcome"`
	Dimensions           []BoundaryDimension `json:"dimensions"`
	Evidence             []EvidenceRef       `json:"evidence"`
	CounterEvidence      []EvidenceRef       `json:"counterEvidence"`
	PeerWitnesses        []string            `json:"peerWitnesses"`
	NotEstablished       []string            `json:"notEstablished"`
	DecisionDigest       string              `json:"decisionDigest"`
}

// EvaluateAddition compares a single new Operation with an existing logical
// Application/Service boundary. It derives both projections afresh from validated
// canonical manifests, never from caller-supplied fingerprints or union summaries.
// Existing-Operation edits, moves and batches are intentionally separate policies.
func EvaluateAddition(request AdditionRequest, before, after contract.Manifest) (ServiceBoundaryDecision, error) {
	if !fullCommitSHA(request.BaseSHA) || request.Application != strings.TrimSpace(request.Application) || request.OperationID == "" || request.OperationID != strings.TrimSpace(request.OperationID) {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition: exact commit SHA, application and operation identity are required")
	}
	left, err := Inspect(before, request.Application)
	if err != nil {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition before: %w", err)
	}
	right, err := Inspect(after, request.Application)
	if err != nil {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition after: %w", err)
	}
	// Detach/normalize once for full-model digests and the single-addition guard.
	before, after = detachedManifest(before), detachedManifest(after)
	if hasOperation(before, request.OperationID) {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition: operation %s already exists in the base", request.OperationID)
	}
	var candidate *OperationEvidence
	for i := range right.Fingerprint.Operations {
		if right.Fingerprint.Operations[i].Plan.OperationID == request.OperationID {
			candidate = &right.Fingerprint.Operations[i]
		}
	}
	if candidate == nil {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition: operation %s is absent from target application", request.OperationID)
	}
	d := ServiceBoundaryDecision{
		SchemaVersion: 1, Authority: "read_only", PolicyVersion: AdditionPolicyVersion,
		BaseSHA: request.BaseSHA, OperationID: request.OperationID, TargetApplication: request.Application, TargetService: right.Fingerprint.Service,
		OperationPlanDigest: modelDigest("boundary-candidate-plan/v1", candidate.Plan),
		BeforeFingerprint:   left.FingerprintDigest, AfterFingerprint: right.FingerprintDigest,
		BeforeManifestDigest: modelDigest("boundary-manifest/v1", before), AfterManifestDigest: modelDigest("boundary-manifest/v1", after),
		Outcome: ArchitectureReviewRequired, Dimensions: []BoundaryDimension{}, Evidence: []EvidenceRef{}, CounterEvidence: []EvidenceRef{}, PeerWitnesses: []string{},
		NotEstablished: []string{"business_ontology", "client_usage", "external_input_content", "mutation_authority", "runtime_availability", "runtime_provider_state", "signed_provenance", "source_byte_identity"},
	}
	d.Evidence = append(d.Evidence,
		EvidenceRef{"before:manifest", "canonical_manifest", "before", request.Application, d.BeforeManifestDigest},
		EvidenceRef{"after:manifest", "canonical_manifest", "after", request.Application, d.AfterManifestDigest},
		EvidenceRef{"after:candidate", "operation", "after", request.OperationID, modelDigest("boundary-operation/v1", candidate)},
	)
	for _, peer := range left.Fingerprint.Operations {
		d.Evidence = append(d.Evidence, EvidenceRef{peerRef(peer), "operation", "before", peer.Plan.OperationID, modelDigest("boundary-operation/v1", peer)})
	}
	scope := newDimension("change_scope", true, "single addition preserves existing canonical declarations and only introduces reachable source/type evidence")
	if preservesBaseline(before, after, *candidate) {
		scope.Verdict, scope.Evidence = "same", []string{"before:manifest", "after:manifest"}
	} else {
		scope.Verdict, scope.CounterEvidence = "different", []string{"before:manifest", "after:manifest"}
	}
	d.Dimensions = append(d.Dimensions, scope)
	contextDim := intentDimension("context", *candidate, left.Fingerprint.Operations)
	aggregateDim := intentDimension("aggregate", *candidate, left.Fingerprint.Operations)
	d.Dimensions = append(d.Dimensions, contextDim, aggregateDim)

	// All dimensions in the automatic-reuse path must have ONE common peer.
	// Independent per-dimension matches can otherwise manufacture a precedent
	// that never existed (e.g. one peer's permissions + another peer's transaction).
	comparisons := []struct {
		name   string
		reason string
		equal  func(OperationEvidence, OperationEvidence) bool
	}{
		{"authorization", "exact canonical security vocabulary; permission prefixes are not resource-domain proof", func(a, b OperationEvidence) bool { return jsonEqual(a.Plan.Security, b.Plan.Security) }},
		{"consistency", "exact execution and composition boundary; policy differences require review, not an inferred contradiction", func(a, b OperationEvidence) bool {
			return jsonEqual(a.Plan.Execution, b.Plan.Execution) && a.Plan.Composition.Boundary == b.Plan.Composition.Boundary
		}},
		{"dependency", "unchanged Application requirements/capabilities and exact declared child-Operation dependency profile", func(a, b OperationEvidence) bool {
			return jsonEqual(left.Fingerprint.ApplicationRequires, right.Fingerprint.ApplicationRequires) && jsonEqual(left.Fingerprint.Capabilities, right.Fingerprint.Capabilities) && jsonEqual(a.Plan.Composition, b.Plan.Composition)
		}},
		{"client_contract", "shared canonical request/response identities and transport shape, not inferred client usage or route-name cohesion", sameClientContract},
	}
	witnesses := map[string]bool{}
	for _, peer := range left.Fingerprint.Operations {
		witnesses[peer.Plan.OperationID] = true
	}
	for _, compare := range comparisons {
		dim := newDimension(compare.name, true, compare.reason)
		for _, peer := range left.Fingerprint.Operations {
			if compare.equal(*candidate, peer) {
				dim.Evidence = append(dim.Evidence, peerRef(peer))
			} else {
				dim.CounterEvidence = append(dim.CounterEvidence, peerRef(peer))
				delete(witnesses, peer.Plan.OperationID)
			}
		}
		if len(dim.Evidence) > 0 {
			dim.Verdict = "same"
		} else if len(dim.CounterEvidence) > 0 {
			dim.Verdict = "different"
		}
		dim.Evidence = append(dim.Evidence, "after:candidate")
		d.Dimensions = append(d.Dimensions, dim)
	}
	coupling := newDimension("cross_operation_coupling", true, "one unchanged peer must jointly witness every canonical compatibility dimension; co-location alone is not evidence")
	for _, peer := range left.Fingerprint.Operations {
		if witnesses[peer.Plan.OperationID] {
			d.PeerWitnesses = append(d.PeerWitnesses, peer.Plan.OperationID)
			coupling.Evidence = append(coupling.Evidence, peerRef(peer))
		} else {
			coupling.CounterEvidence = append(coupling.CounterEvidence, peerRef(peer))
		}
	}
	if len(d.PeerWitnesses) > 0 {
		coupling.Verdict = "compatible"
	}
	d.Dimensions = append(d.Dimensions, coupling,
		newDimension("lifecycle", false, "no observed release/change-cadence evidence is accepted by this policy"),
		newDimension("availability", false, "no observed failure/scaling evidence is accepted by this policy"))

	// Contradictions are considered before missing evidence. A context different
	// from every known base declaration cannot be rescued by an unknown peer.
	// A mixed existing Service stays review-required even when a peer matches.
	knownContexts := left.IntentCoverage.Contexts
	if scope.Verdict == "same" && candidate.Boundary != nil && len(knownContexts) > 0 && !containsString(knownContexts, candidate.Boundary.Context) {
		d.Outcome = CreateNewApplication
	} else {
		all := true
		for _, dim := range d.Dimensions {
			if dim.Critical && dim.Verdict != "same" && dim.Verdict != "compatible" {
				all = false
			}
		}
		if all {
			d.Outcome = ReuseExistingApplication
		}
	}
	counter := map[string]bool{}
	for _, dim := range d.Dimensions {
		for _, id := range dim.CounterEvidence {
			counter[id] = true
		}
	}
	sort.Slice(d.Evidence, func(i, j int) bool { return d.Evidence[i].ID < d.Evidence[j].ID })
	for _, ref := range d.Evidence {
		if counter[ref.ID] {
			d.CounterEvidence = append(d.CounterEvidence, ref)
		}
	}
	d.DecisionDigest = modelDigest("service-boundary-decision/v1", d)
	return d, nil
}

// RevalidateAddition never trusts a claimed outcome, fingerprint, counter-evidence
// list or self-consistent hash. It recomputes everything against caller-resolved
// current facts and task identity, including the active policy version. A matching
// REVIEW/BLOCK decision is valid evidence but remains a non-reuse outcome.
func RevalidateAddition(request AdditionRequest, before, after contract.Manifest, proof ServiceBoundaryDecision) error {
	current, err := EvaluateAddition(request, before, after)
	if err != nil {
		return fmt.Errorf("%w: cannot recompute decision: %v", ErrStaleBoundaryProof, err)
	}
	if !jsonEqual(current, proof) {
		return fmt.Errorf("%w: decision differs from current canonical policy evaluation", ErrStaleBoundaryProof)
	}
	return nil
}

func newDimension(name string, critical bool, reason string) BoundaryDimension {
	return BoundaryDimension{Name: name, Verdict: "unknown", Critical: critical, Reason: reason, Evidence: []string{}, CounterEvidence: []string{}}
}
func peerRef(peer OperationEvidence) string { return "before:operation:" + peer.Plan.OperationID }
func intentDimension(name string, candidate OperationEvidence, peers []OperationEvidence) BoundaryDimension {
	d := newDimension(name, true, "all existing Operations require explicit compatible intent; missing or mixed intent cannot authorize automatic reuse")
	if candidate.Boundary == nil {
		d.CounterEvidence = append(d.CounterEvidence, "after:candidate")
		return d
	}
	unknown, different := len(peers) == 0, false
	for _, peer := range peers {
		if peer.Boundary == nil {
			unknown = true
			d.CounterEvidence = append(d.CounterEvidence, peerRef(peer))
			continue
		}
		same := candidate.Boundary.Context == peer.Boundary.Context
		if name == "aggregate" {
			same = candidate.Boundary.Aggregate == peer.Boundary.Aggregate && candidate.Boundary.AggregateNotApplicableReason == peer.Boundary.AggregateNotApplicableReason
		}
		if same {
			d.Evidence = append(d.Evidence, peerRef(peer))
		} else {
			different = true
			d.CounterEvidence = append(d.CounterEvidence, peerRef(peer))
		}
	}
	switch {
	case different:
		d.Verdict = "different"
	case !unknown:
		d.Verdict = "same"
	}
	d.Evidence = append(d.Evidence, "after:candidate")
	return d
}
func sameClientContract(a, b OperationEvidence) bool {
	// Distinct RPC method names/HTTP paths are expected for a new Operation.
	// Compare the declared shape only; never infer a business route family.
	x, y := a.Plan.Bindings, b.Plan.Bindings
	if a.Plan.RequestType != b.Plan.RequestType || a.Plan.ResponseType != b.Plan.ResponseType || a.ClientStreaming != b.ClientStreaming || a.ServerStreaming != b.ServerStreaming || (x.RPC == "") != (y.RPC == "") {
		return false
	}
	shape := func(op OperationEvidence) []string {
		values := []string{}
		for _, h := range op.Plan.Bindings.HTTP {
			values = append(values, modelDigest("http-shape/v1", []string{h.Method, h.Body, h.ResponseBody}))
		}
		sort.Strings(values)
		return values
	}
	return jsonEqual(shape(a), shape(b))
}
func hasOperation(m contract.Manifest, id string) bool {
	for _, s := range m.Services {
		for _, method := range s.Methods {
			if method.Operation != nil && method.Operation.ID == id {
				return true
			}
		}
		if s.Application != nil {
			for _, op := range s.Application.Operations {
				if op.ID == id {
					return true
				}
			}
		}
	}
	return false
}
func detachedManifest(m contract.Manifest) contract.Manifest {
	// These concrete canonical types contain only JSON-supported values; Inspect
	// has already validated/marshaled them. No user-provided Marshaler is called.
	data, _ := json.Marshal(m)
	var copy contract.Manifest
	_ = json.Unmarshal(data, &copy)
	copy.Normalize()
	return copy
}
func modelDigest(domain string, v any) string {
	data, _ := json.Marshal(v)
	h := sha256.Sum256(append([]byte(domain+"\n"), data...))
	return hex.EncodeToString(h[:])
}
func jsonEqual(a, b any) bool {
	x, ex := json.Marshal(a)
	y, ey := json.Marshal(b)
	return ex == nil && ey == nil && string(x) == string(y)
}
func fullCommitSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	if strings.Trim(s, "0") == "" {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
func containsString(values []string, v string) bool {
	for _, item := range values {
		if item == v {
			return true
		}
	}
	return false
}
