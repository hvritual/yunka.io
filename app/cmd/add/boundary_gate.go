package add

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/boundarycore"
	"yunka.io/app/cmd/projectflow"
)

type OperationBoundarySemantics struct {
	Context                      string `json:"context"`
	Aggregate                    string `json:"aggregate,omitempty"`
	AggregateNotApplicableReason string `json:"aggregateNotApplicableReason,omitempty"`
}

type OperationBoundaryDecision = boundarycore.ServiceBoundaryDecision

func operationBoundaryIntent(options OperationOptions) *contract.BoundaryIntent {
	if options.BoundaryContext == "" && options.BoundaryAggregate == "" && options.BoundaryNoAggregateReason == "" {
		return nil
	}
	return &contract.BoundaryIntent{Context: options.BoundaryContext, Aggregate: options.BoundaryAggregate, AggregateNotApplicableReason: options.BoundaryNoAggregateReason}
}

func evaluateOperationBoundary(root, sourcePath, domain, application, packageName, rpcName, requestType, responseType string, options OperationOptions) (*OperationBoundaryDecision, error) {
	baseSHA, err := resolveBoundaryBaseSHA(root)
	if err != nil {
		return nil, fmt.Errorf("add operation: bind boundary decision to Git HEAD: %w", err)
	}
	snapshot, err := projectflow.DescribeContractSourceSnapshot(context.Background(), projectflow.Options{Root: root, ProtoPaths: append([]string(nil), options.ProtoPaths...)})
	if err != nil {
		return nil, fmt.Errorf("add operation: compile current canonical boundary facts: %w", err)
	}
	before := snapshot.Manifest
	data, err := json.Marshal(before)
	if err != nil {
		return nil, err
	}
	var after contract.Manifest
	if err := json.Unmarshal(data, &after); err != nil {
		return nil, err
	}
	after.Normalize()

	applicationKey := domain + "/" + application
	var target *contract.Service
	for i := range after.Services {
		service := &after.Services[i]
		if service.Application == nil || service.Domain+"/"+service.Application.Name != applicationKey {
			continue
		}
		if target != nil {
			return nil, fmt.Errorf("add operation: application %s has multiple canonical Service projections", applicationKey)
		}
		target = service
	}
	if target == nil {
		return nil, fmt.Errorf("add operation: canonical application %s was not found", applicationKey)
	}
	if projectPath, ok := snapshot.Paths[target.SourceFile]; ok && cleanRelative(projectPath) != cleanRelative(sourcePath) {
		return nil, fmt.Errorf("add operation: selected source %s disagrees with canonical Service source %s", sourcePath, projectPath)
	}

	permissionMode := options.PermissionMode
	if permissionMode == "" {
		permissionMode = "all"
	}
	composition := options.Composition
	if composition == "none" {
		composition = ""
	}
	operation := contract.OperationDeclaration{
		ID: options.OperationID, UseCase: options.UseCase, Permissions: append([]string(nil), options.Permissions...), PermissionMode: permissionMode,
		TenantRequired: options.Tenant == "required", Authentication: append([]string(nil), options.Authentication...), Public: options.Access == "public",
		RequiresOperations: append([]string(nil), options.RequiresOperations...), Composition: composition,
		Execution: &contract.ExecutionPolicy{Transaction: options.Transaction, Idempotency: options.Idempotency}, Boundary: operationBoundaryIntent(options),
	}
	requestFull := packageName + "." + requestType
	responseFull := packageName + "." + responseType
	method := contract.Method{Name: rpcName, FullName: target.FullName + "." + rpcName, SourceFile: target.SourceFile, Request: requestFull, Response: responseFull, Operation: &operation}
	if options.Access == "protected" {
		method.Authorization = &contract.AuthorizationPolicy{OperationID: options.OperationID, Permissions: append([]string(nil), options.Permissions...), PermissionMode: permissionMode, TenantRequired: options.Tenant == "required", Authentication: append([]string(nil), options.Authentication...)}
	}
	if options.HTTPMethod != "" {
		method.HTTP = []contract.HTTPBinding{{Method: options.HTTPMethod, Path: options.HTTPPath, Body: options.HTTPBody}}
	}
	target.Methods = append(target.Methods, method)

	hasMessage := func(name string) bool {
		for _, item := range after.Messages {
			if item.FullName == name {
				return true
			}
		}
		return false
	}
	if !hasMessage(requestFull) {
		after.Messages = append(after.Messages, contract.Message{Name: requestType, FullName: requestFull, SourceFile: target.SourceFile, DTO: &contract.DTODeclaration{Kind: "input"}, Fields: []contract.Field{}})
	}
	if !hasMessage(responseFull) {
		after.Messages = append(after.Messages, contract.Message{Name: responseType, FullName: responseFull, SourceFile: target.SourceFile, DTO: &contract.DTODeclaration{Kind: "output"}, Fields: []contract.Field{}})
	}
	after.Normalize()

	currentSHA, err := resolveBoundaryBaseSHA(root)
	if err != nil {
		return nil, fmt.Errorf("add operation: re-read Git HEAD before boundary decision: %w", err)
	}
	if currentSHA != baseSHA {
		return nil, fmt.Errorf("add operation: Git HEAD changed while boundary evidence was being evaluated; base=%s current=%s", baseSHA, currentSHA)
	}
	decision, err := boundarycore.EvaluateAddition(boundarycore.AdditionRequest{BaseSHA: baseSHA, Application: applicationKey, OperationID: options.OperationID}, before, after)
	if err != nil {
		return nil, err
	}
	return &decision, nil
}

func resolveBoundaryBaseSHA(root string) (string, error) {
	command := exec.Command("git", "-C", root, "rev-parse", "--verify", "HEAD^{commit}")
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("resolve exact Git HEAD: %s", detail)
	}
	sha := strings.TrimSpace(string(output))
	if !exactBoundaryCommitSHA(sha) {
		return "", fmt.Errorf("Git HEAD is not an exact lowercase commit SHA: %q", sha)
	}
	return sha, nil
}

func exactBoundaryCommitSHA(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	if strings.Trim(value, "0") == "" {
		return false
	}
	for _, c := range value {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func boundaryAllowsMutation(value *OperationBoundaryDecision) bool {
	return value != nil && value.Outcome == boundarycore.ReuseExistingService
}

func boundaryBlockedError(value *OperationBoundaryDecision) error {
	if value == nil {
		return fmt.Errorf("add operation: boundary decision is missing")
	}
	return fmt.Errorf("add operation: Operation Growth boundary outcome=%s; review the plan boundaryDecision before changing the Service", value.Outcome)
}

func boundarySemantics(value *contract.BoundaryIntent) *OperationBoundarySemantics {
	if value == nil {
		return nil
	}
	return &OperationBoundarySemantics{Context: strings.TrimSpace(value.Context), Aggregate: strings.TrimSpace(value.Aggregate), AggregateNotApplicableReason: strings.TrimSpace(value.AggregateNotApplicableReason)}
}
