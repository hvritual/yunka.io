package applicationgraph

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hvritual/yunka.io/pkg/operationplan"
)

// AddOperationPlans projects the compiled C9 execution contract into the graph.
// It deliberately consumes only the immutable derived plan and does not infer
// business relationships from package names or source scanning.
func AddOperationPlans(builder *Builder, set operationplan.Set) error {
	if builder == nil {
		return fmt.Errorf("applicationgraph: nil builder")
	}
	set = operationplan.Normalize(set)
	if err := operationplan.Validate(set); err != nil {
		return fmt.Errorf("applicationgraph: invalid operation plans: %w", err)
	}
	digest, err := operationplan.Digest(set)
	if err != nil {
		return err
	}
	evidence := []Evidence{Declared("operation.plan", "compiled protobuf execution plan")}

	for _, plan := range set.Operations {
		domainID := ID(NodeDomain, plan.Domain)
		if err := builder.AddNode(Node{ID: domainID, Kind: NodeDomain, Name: plan.Domain, Evidence: evidence}); err != nil {
			return err
		}
		applicationName := plan.Domain + "/" + plan.Application
		applicationID := ID(NodeApplication, applicationName)
		if err := builder.AddNode(Node{ID: applicationID, Kind: NodeApplication, Name: applicationName,
			Attributes: map[string]string{"domain": plan.Domain, "application": plan.Application}, Evidence: evidence}); err != nil {
			return err
		}
		operationAttrs := map[string]string{
			"operationId":         plan.OperationID,
			"useCase":             plan.UseCase,
			"domain":              plan.Domain,
			"application":         plan.Application,
			"public":              strconv.FormatBool(plan.Security.Public),
			"tenantRequired":      strconv.FormatBool(plan.Security.TenantRequired),
			"permissionMode":      plan.Security.PermissionMode,
			"transaction":         plan.Execution.Transaction,
			"idempotency":         plan.Execution.Idempotency,
			"operationPlanSchema": strconv.Itoa(set.SchemaVersion),
			"operationPlanDigest": digest,
		}
		if plan.Composition.Boundary != "" {
			operationAttrs["composition"] = plan.Composition.Boundary
		}
		if plan.Bindings.RPC != "" {
			operationAttrs["rpcPath"] = plan.Bindings.RPC
		}
		if err := builder.AddNode(Node{ID: ID(NodeOperation, plan.OperationID), Kind: NodeOperation, Name: plan.OperationID, Attributes: operationAttrs, Evidence: evidence}); err != nil {
			return err
		}
	}

	for _, plan := range set.Operations {
		domainID := ID(NodeDomain, plan.Domain)
		applicationName := plan.Domain + "/" + plan.Application
		applicationID := ID(NodeApplication, applicationName)
		operationID := ID(NodeOperation, plan.OperationID)
		if err := builder.AddEdge(Edge{From: domainID, To: applicationID, Kind: EdgeContains, Evidence: evidence}); err != nil {
			return err
		}
		if err := builder.AddEdge(Edge{From: applicationID, To: operationID, Kind: EdgeContains, Evidence: evidence}); err != nil {
			return err
		}
		for _, dependency := range plan.ApplicationRequires {
			if err := builder.AddEdge(Edge{From: applicationID, To: ID(NodeApplication, dependency), Kind: EdgeDependsOn, Evidence: evidence}); err != nil {
				return err
			}
		}
		for _, dependency := range plan.Composition.RequiresOperations {
			if err := builder.AddEdge(Edge{From: operationID, To: ID(NodeOperation, dependency), Kind: EdgeDependsOn, Evidence: evidence}); err != nil {
				return err
			}
		}
		for _, permission := range plan.Security.Permissions {
			permissionID := ID(NodePermission, permission)
			if err := builder.AddNode(Node{ID: permissionID, Kind: NodePermission, Name: permission, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: operationID, To: permissionID, Kind: EdgeRequires, Evidence: evidence}); err != nil {
				return err
			}
		}
		if plan.RequestType != "" {
			requestID := ID(NodeMessage, plan.RequestType)
			if err := builder.AddNode(Node{ID: requestID, Kind: NodeMessage, Name: plan.RequestType, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: operationID, To: requestID, Kind: EdgeAccepts, Evidence: evidence}); err != nil {
				return err
			}
		}
		if plan.ResponseType != "" {
			responseID := ID(NodeMessage, plan.ResponseType)
			if err := builder.AddNode(Node{ID: responseID, Kind: NodeMessage, Name: plan.ResponseType, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: operationID, To: responseID, Kind: EdgeReturns, Evidence: evidence}); err != nil {
				return err
			}
		}
		if service, ok := serviceFromRPCBinding(plan.Bindings.RPC); ok {
			serviceID := ID(NodeService, service)
			if err := builder.AddNode(Node{ID: serviceID, Kind: NodeService, Name: service, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: serviceID, To: operationID, Kind: EdgeExposes, Evidence: evidence}); err != nil {
				return err
			}
		}
		for _, binding := range plan.Bindings.HTTP {
			name := strings.ToUpper(strings.TrimSpace(binding.Method)) + " " + strings.TrimSpace(binding.Path)
			routeID := ID(NodeHTTPRoute, name)
			attrs := map[string]string{"method": strings.ToUpper(strings.TrimSpace(binding.Method)), "path": strings.TrimSpace(binding.Path)}
			if binding.Body != "" {
				attrs["body"] = binding.Body
			}
			if binding.ResponseBody != "" {
				attrs["responseBody"] = binding.ResponseBody
			}
			if err := builder.AddNode(Node{ID: routeID, Kind: NodeHTTPRoute, Name: name, Attributes: attrs, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: routeID, To: operationID, Kind: EdgeRoutesTo, Evidence: evidence}); err != nil {
				return err
			}
		}
	}
	return nil
}

func serviceFromRPCBinding(binding string) (string, bool) {
	binding = strings.TrimSpace(binding)
	if !strings.HasPrefix(binding, "/") {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(binding, "/"), "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", false
	}
	return strings.TrimSpace(parts[0]), true
}
