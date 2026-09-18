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

const AdditionPolicyVersion = "semantic-cohesion-addition/v2"

const (
	ReuseExistingService       = "reuse_existing_service"
	CreateNewService           = "create_new_service"
	ArchitectureReviewRequired = "architecture_review_required"
)

var ErrStaleBoundaryProof = errors.New("STALE_BOUNDARY_PROOF")

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
	Verdict         string   `json:"verdict"`
	Critical        bool     `json:"critical"`
	Reason          string   `json:"reason"`
	Evidence        []string `json:"evidence"`
	CounterEvidence []string `json:"counterEvidence"`
}

type ServiceBoundaryDecision struct {
	SchemaVersion        int                 `json:"schemaVersion"`
	Authority            string              `json:"authority"`
	PolicyVersion        string              `json:"policyVersion"`
	BaseSHA              string              `json:"baseSha,omitempty"`
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

func EvaluateAddition(request AdditionRequest, before, after contract.Manifest) (ServiceBoundaryDecision, error) {
	if (request.BaseSHA != "" && !fullCommitSHA(request.BaseSHA)) || strings.TrimSpace(request.Application) == "" || strings.TrimSpace(request.OperationID) == "" || request.Application != strings.TrimSpace(request.Application) || request.OperationID != strings.TrimSpace(request.OperationID) {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition: optional base SHA must be exact; application and operation identity are required")
	}
	left, err := Inspect(before, request.Application)
	if err != nil {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition before: %w", err)
	}
	right, err := Inspect(after, request.Application)
	if err != nil {
		return ServiceBoundaryDecision{}, fmt.Errorf("boundary addition after: %w", err)
	}
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
		SchemaVersion: 2, Authority: "read_only", PolicyVersion: AdditionPolicyVersion,
		BaseSHA: request.BaseSHA, OperationID: request.OperationID, TargetApplication: request.Application, TargetService: right.Fingerprint.Service,
		OperationPlanDigest: modelDigest("boundary-candidate-plan/v2", candidate.Plan),
		BeforeFingerprint:   left.FingerprintDigest, AfterFingerprint: right.FingerprintDigest,
		BeforeManifestDigest: modelDigest("boundary-manifest/v2", before), AfterManifestDigest: modelDigest("boundary-manifest/v2", after),
		Outcome: ArchitectureReviewRequired, Dimensions: []BoundaryDimension{}, Evidence: []EvidenceRef{}, CounterEvidence: []EvidenceRef{}, PeerWitnesses: []string{},
		NotEstablished: []string{"business_ontology", "observed_client_usage", "release_lifecycle", "runtime_availability", "runtime_provider_state", "signed_provenance"},
	}
	d.Evidence = append(d.Evidence,
		EvidenceRef{ID: "before:manifest", Kind: "canonical_manifest", Side: "before", Subject: request.Application, Digest: d.BeforeManifestDigest},
		EvidenceRef{ID: "after:manifest", Kind: "canonical_manifest", Side: "after", Subject: request.Application, Digest: d.AfterManifestDigest},
		EvidenceRef{ID: "after:candidate", Kind: "operation", Side: "after", Subject: request.OperationID, Digest: modelDigest("boundary-operation/v2", candidate)},
	)
	for _, peer := range left.Fingerprint.Operations {
		d.Evidence = append(d.Evidence, EvidenceRef{ID: peerRef(peer), Kind: "operation", Side: "before", Subject: peer.Plan.OperationID, Digest: modelDigest("boundary-operation/v2", peer)})
	}

	scope := newDimension("change_scope", true, "single Operation addition must preserve every existing canonical declaration; only candidate-reachable source/type additions are permitted")
	if preservesBaseline(before, after, *candidate) {
		scope.Verdict = "same"
		scope.Evidence = []string{"before:manifest", "after:manifest"}
	} else {
		scope.Verdict = "different"
		scope.CounterEvidence = []string{"before:manifest", "after:manifest"}
	}
	d.Dimensions = append(d.Dimensions, scope)

	contextDim := intentDimensionV2("context", *candidate, left.Fingerprint.Operations)
	aggregateDim := intentDimensionV2("aggregate", *candidate, left.Fingerprint.Operations)
	d.Dimensions = append(d.Dimensions, contextDim, aggregateDim)

	comparisons := []struct {
		name, reason string
		equal        func(OperationEvidence, OperationEvidence) bool
	}{
		{"authorization", "derived permission/security vocabulary must remain in the same canonical family; exact permission IDs and authentication mechanisms may vary by Operation", sameAuthorizationFamily},
		{"consistency", "transaction and composition boundary must match an existing peer", func(a, b OperationEvidence) bool {
			return a.Plan.Execution.Transaction == b.Plan.Execution.Transaction && a.Plan.Composition.Boundary == b.Plan.Composition.Boundary
		}},
		{"dependency", "Application requirements/capabilities and declared child-Operation dependencies must match an existing peer", func(a, b OperationEvidence) bool {
			return jsonEqual(left.Fingerprint.ApplicationRequires, right.Fingerprint.ApplicationRequires) && jsonEqual(left.Fingerprint.Capabilities, right.Fingerprint.Capabilities) && jsonEqual(a.Plan.Composition.RequiresOperations, b.Plan.Composition.RequiresOperations)
		}},
		{"client_contract", "request/response package, RPC/HTTP transport family and streaming shape must remain compatible; exact DTO names and route names are Operation-specific", sameClientContractFamily},
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
				dim.Verdict = "compatible"
			} else {
				dim.CounterEvidence = append(dim.CounterEvidence, peerRef(peer))
				delete(witnesses, peer.Plan.OperationID)
			}
		}
		dim.Evidence = append(dim.Evidence, "after:candidate")
		if len(left.Fingerprint.Operations) == 0 {
			dim.Verdict = "same"
		}
		d.Dimensions = append(d.Dimensions, dim)
	}

	coupling := newDimension("cross_operation_coupling", true, "one existing peer must jointly witness authorization, consistency, dependency and client-contract compatibility; unrelated per-dimension peers cannot manufacture cohesion")
	if len(left.Fingerprint.Operations) == 0 {
		coupling.Verdict = "same"
		coupling.Evidence = []string{"after:candidate"}
	} else {
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
	}
	d.Dimensions = append(d.Dimensions, coupling,
		newDimension("lifecycle", false, "no observed release/change-cadence evidence is accepted by this deterministic policy"),
		newDimension("availability", false, "no observed failure/scaling evidence is accepted by this deterministic policy"),
	)

	peers := left.Fingerprint.Operations
	if candidate.Boundary == nil || scope.Verdict != "same" {
		d.Outcome = ArchitectureReviewRequired
	} else if len(peers) == 0 {
		// First Operation may establish a boundary only when explicit intent is present.
		d.Outcome = ReuseExistingService
	} else {
		knownContexts, knownAggregates, unknownIntent := map[string]bool{}, map[string]bool{}, false
		for _, peer := range peers {
			if peer.Boundary == nil {
				unknownIntent = true
				continue
			}
			knownContexts[peer.Boundary.Context] = true
			knownAggregates[aggregateKey(peer.Boundary)] = true
		}
		candidateAggregate := aggregateKey(candidate.Boundary)
		switch {
		case unknownIntent:
			d.Outcome = ArchitectureReviewRequired
		case !knownContexts[candidate.Boundary.Context] && len(knownContexts) > 0:
			d.Outcome = CreateNewService
		case !knownAggregates[candidateAggregate] && len(knownAggregates) > 0:
			d.Outcome = CreateNewService
		case len(knownContexts) != 1 || len(knownAggregates) != 1:
			d.Outcome = ArchitectureReviewRequired
		default:
			all := true
			for _, dim := range d.Dimensions {
				if dim.Critical && dim.Verdict != "same" && dim.Verdict != "compatible" {
					all = false
				}
			}
			if all {
				d.Outcome = ReuseExistingService
			}
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
	sort.Strings(d.PeerWitnesses)
	d.DecisionDigest = modelDigest("service-boundary-decision/v2", d)
	return d, nil
}

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

func intentDimensionV2(name string, candidate OperationEvidence, peers []OperationEvidence) BoundaryDimension {
	d := newDimension(name, true, "explicit boundary intent is required; all existing peer Operations must have one coherent compatible declaration")
	if candidate.Boundary == nil {
		d.CounterEvidence = append(d.CounterEvidence, "after:candidate")
		return d
	}
	if len(peers) == 0 {
		d.Verdict = "same"
		d.Evidence = append(d.Evidence, "after:candidate")
		return d
	}
	unknown, different := false, false
	for _, peer := range peers {
		if peer.Boundary == nil {
			unknown = true
			d.CounterEvidence = append(d.CounterEvidence, peerRef(peer))
			continue
		}
		same := candidate.Boundary.Context == peer.Boundary.Context
		if name == "aggregate" {
			same = aggregateKey(candidate.Boundary) == aggregateKey(peer.Boundary)
		}
		if same {
			d.Evidence = append(d.Evidence, peerRef(peer))
		} else {
			different = true
			d.CounterEvidence = append(d.CounterEvidence, peerRef(peer))
		}
	}
	d.Evidence = append(d.Evidence, "after:candidate")
	if different {
		d.Verdict = "different"
	} else if !unknown {
		d.Verdict = "same"
	}
	return d
}

func aggregateKey(value *contract.BoundaryIntent) string {
	if value == nil {
		return "<unknown>"
	}
	if strings.TrimSpace(value.Aggregate) != "" {
		return "aggregate:" + strings.TrimSpace(value.Aggregate)
	}
	return "not-applicable:" + strings.TrimSpace(value.AggregateNotApplicableReason)
}

func sameAuthorizationFamily(a, b OperationEvidence) bool {
	return jsonEqual(permissionFamilies(a), permissionFamilies(b)) && a.Plan.Security.TenantRequired == b.Plan.Security.TenantRequired
}
func permissionFamilies(op OperationEvidence) []string {
	families := []string{}
	if op.Plan.Security.Public {
		families = append(families, "public")
	}
	for _, permission := range op.Plan.Security.Permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if index := strings.LastIndex(permission, "."); index > 0 {
			permission = permission[:index]
		}
		families = append(families, permission)
	}
	sort.Strings(families)
	result := families[:0]
	for _, value := range families {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return append([]string(nil), result...)
}

func sameClientContractFamily(a, b OperationEvidence) bool {
	if typePackage(a.Plan.RequestType) != typePackage(b.Plan.RequestType) || typePackage(a.Plan.ResponseType) != typePackage(b.Plan.ResponseType) {
		return false
	}
	if a.ClientStreaming != b.ClientStreaming || a.ServerStreaming != b.ServerStreaming {
		return false
	}
	if (a.Plan.Bindings.RPC == "") != (b.Plan.Bindings.RPC == "") {
		return false
	}
	return jsonEqual(httpMethodFamily(a), httpMethodFamily(b))
}
func typePackage(value string) string {
	if i := strings.LastIndex(strings.TrimSpace(value), "."); i > 0 {
		return value[:i]
	}
	return strings.TrimSpace(value)
}
func httpMethodFamily(op OperationEvidence) []string {
	values := []string{}
	for _, h := range op.Plan.Bindings.HTTP {
		values = append(values, strings.ToUpper(strings.TrimSpace(h.Method)))
	}
	sort.Strings(values)
	return values
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
