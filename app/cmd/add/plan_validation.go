package add

import (
	"fmt"
	"strings"

	"yunka.io/app/cmd/boundarycore"
)

func RevalidateOperationPlan(root string, candidate Report) (Report, error) {
	if candidate.SchemaVersion != OperationReportVersion {
		return Report{}, fmt.Errorf("%w: add operation plan: unsupported schemaVersion %d", boundarycore.ErrStaleBoundaryProof, candidate.SchemaVersion)
	}
	if strings.TrimSpace(candidate.Kind) != "operation-plan" {
		return Report{}, fmt.Errorf("add operation plan: kind must be operation-plan")
	}
	if candidate.ExplicitSemantics == nil {
		return Report{}, fmt.Errorf("add operation plan: explicitSemantics are required")
	}
	if candidate.BoundaryDecision == nil || candidate.BaseSHA == "" || candidate.InputsDigest == "" {
		return Report{}, fmt.Errorf("%w: boundary-bound operation plan is required", boundarycore.ErrStaleBoundaryProof)
	}
	if !boundaryAllows(candidate) {
		return Report{}, fmt.Errorf("%w: %s", ErrBoundaryBlocked, candidate.BoundaryDecision.Outcome)
	}
	identity := candidate.Identity
	for _, key := range []string{"domain", "application", "operationId", "useCase", "rpc", "requestType", "responseType"} {
		if strings.TrimSpace(identity[key]) == "" {
			return Report{}, fmt.Errorf("add operation plan: identity.%s is required", key)
		}
	}
	if strings.TrimSpace(identity["useCase"]) != strings.TrimSpace(candidate.ExplicitSemantics.UseCase) {
		return Report{}, fmt.Errorf("add operation plan: identity.useCase does not match explicitSemantics.useCase")
	}
	var source string
	for _, mutation := range candidate.Mutations {
		if mutation.Action != "modified" || !strings.HasSuffix(strings.ToLower(strings.TrimSpace(mutation.Path)), ".proto") {
			continue
		}
		if source != "" {
			return Report{}, fmt.Errorf("add operation plan: exactly one modified protobuf source is required")
		}
		source = mutation.Path
	}
	if strings.TrimSpace(source) == "" {
		return Report{}, fmt.Errorf("add operation plan: modified protobuf source is required")
	}
	semantics := candidate.ExplicitSemantics
	options := OperationOptions{
		Root:               root,
		Boundary:           semantics.Boundary,
		ProtoPaths:         append([]string(nil), candidate.ProtoPaths...),
		ApplicationKey:     strings.TrimSpace(identity["domain"]) + "/" + strings.TrimSpace(identity["application"]),
		OperationID:        strings.TrimSpace(identity["operationId"]),
		Source:             source,
		UseCase:            semantics.UseCase,
		RPCName:            strings.TrimSpace(identity["rpc"]),
		RequestType:        unqualifiedMessage(identity["requestType"]),
		ResponseType:       unqualifiedMessage(identity["responseType"]),
		Access:             semantics.Access,
		Permissions:        append([]string{}, semantics.Permissions...),
		PermissionMode:     semantics.PermissionMode,
		Tenant:             semantics.Tenant,
		Authentication:     append([]string{}, semantics.Authentication...),
		Transaction:        semantics.Transaction,
		Idempotency:        semantics.Idempotency,
		Composition:        semantics.Composition,
		RequiresOperations: append([]string{}, semantics.RequiresOperations...),
	}
	if semantics.HTTP != nil {
		options.HTTPMethod = semantics.HTTP.Method
		options.HTTPPath = semantics.HTTP.Path
		options.HTTPBody = semantics.HTTP.Body
	}
	rebuilt, err := PlanOperation(options)
	if err != nil {
		return Report{}, fmt.Errorf("%w: add operation plan: replan against current project: %v", boundarycore.ErrStaleBoundaryProof, err)
	}
	candidateJSON, err := Render(candidate, FormatAgentJSON)
	if err != nil {
		return Report{}, err
	}
	rebuiltJSON, err := Render(rebuilt, FormatAgentJSON)
	if err != nil {
		return Report{}, err
	}
	if candidateJSON != rebuiltJSON {
		return Report{}, fmt.Errorf("%w: supplied plan does not match canonical replan for the current project", boundarycore.ErrStaleBoundaryProof)
	}
	if !boundaryAllows(rebuilt) {
		return Report{}, fmt.Errorf("%w: %s", ErrBoundaryBlocked, rebuilt.BoundaryDecision.Outcome)
	}
	return rebuilt, nil
}

func unqualifiedMessage(value string) string {
	value = strings.TrimSpace(value)
	if index := strings.LastIndex(value, "."); index >= 0 && index+1 < len(value) {
		return value[index+1:]
	}
	return value
}
