package add

func explicitOperationSemantics(options OperationOptions) *OperationSemantics {
	result := &OperationSemantics{
		UseCase:            options.UseCase,
		Access:             options.Access,
		Permissions:        append([]string{}, options.Permissions...),
		PermissionMode:     options.PermissionMode,
		Tenant:             options.Tenant,
		Authentication:     append([]string{}, options.Authentication...),
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
