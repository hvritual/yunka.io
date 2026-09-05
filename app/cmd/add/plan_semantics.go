package add

import "strings"

func explicitOperationSemantics(options OperationOptions) *OperationSemantics {
	result := &OperationSemantics{
		UseCase:            options.UseCase,
		Access:             options.Access,
		Permissions:        append([]string{}, options.Permissions...),
		PermissionMode:     options.PermissionMode,
		Tenant:             options.Tenant,
		Authentication:     canonicalPlanAuthentication(options.Authentication),
		Transaction:        options.Transaction,
		Idempotency:        options.Idempotency,
		Composition:        options.Composition,
		RequiresOperations: append([]string{}, options.RequiresOperations...),
	}
	if options.HTTPMethod != "" {
		result.HTTP = &OperationHTTPSemantics{
			Method: options.HTTPMethod,
			Path:   options.HTTPPath,
			Body:   options.HTTPBody,
		}
	}
	return result
}

// canonicalPlanAuthentication projects the public add-operation aliases into
// the same stable vocabulary emitted by canonical contract generation. The
// internal authoring options may retain parser-friendly aliases because the
// protobuf renderer maps them to enum values; prospective semantic artifacts
// must not leak those aliases into ChangeSet comparison.
func canonicalPlanAuthentication(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		switch normalized {
		case "api_key", "api-key":
			normalized = "api-key"
		case "service", "service_token", "service-token":
			normalized = "service-token"
		}
		if normalized != "" {
			result = append(result, normalized)
		}
	}
	return stableStrings(result)
}
