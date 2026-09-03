package add

func explicitOperationSemantics(options OperationOptions) *OperationSemantics {
	result := &OperationSemantics{
		UseCase:            options.UseCase,
		Access:             options.Access,
		Permissions:        append([]string(nil), options.Permissions...),
		PermissionMode:     options.PermissionMode,
		Tenant:             options.Tenant,
		Authentication:     append([]string(nil), options.Authentication...),
		Transaction:        options.Transaction,
		Idempotency:        options.Idempotency,
		Composition:        options.Composition,
		RequiresOperations: append([]string(nil), options.RequiresOperations...),
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
