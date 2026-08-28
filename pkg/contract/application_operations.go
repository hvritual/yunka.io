package contract

import (
	"fmt"
	"sort"
	"strings"
)

type applicationOperationBinding struct {
	MethodName   string
	RequestType  string
	ResponseType string
	Operation    OperationDeclaration
	RPC          string
	HTTP         []HTTPBinding
	SourcePath   string
}

func serviceApplicationOperations(service Service) ([]applicationOperationBinding, error) {
	if service.Application == nil {
		return nil, nil
	}
	result := make([]applicationOperationBinding, 0, len(service.Methods)+len(service.Application.Operations))
	methodOwners := map[string]string{}
	add := func(binding applicationOperationBinding) error {
		binding.MethodName = strings.TrimSpace(binding.MethodName)
		binding.RequestType = normalizeTypeName(binding.RequestType)
		binding.ResponseType = normalizeTypeName(binding.ResponseType)
		normalizeOperationDeclaration(&binding.Operation)
		if binding.MethodName == "" {
			return fmt.Errorf("contract application operation: %s requires application_method", binding.SourcePath)
		}
		if binding.RequestType == "" || binding.ResponseType == "" {
			return fmt.Errorf("contract application operation: %s requires request_type and response_type", binding.SourcePath)
		}
		if owner, duplicate := methodOwners[binding.MethodName]; duplicate {
			return fmt.Errorf("contract application operation: application method %s is declared by both %s and %s", binding.MethodName, owner, binding.SourcePath)
		}
		methodOwners[binding.MethodName] = binding.SourcePath
		binding.Operation.ApplicationMethod = binding.MethodName
		binding.Operation.RequestType = binding.RequestType
		binding.Operation.ResponseType = binding.ResponseType
		binding.HTTP = append([]HTTPBinding(nil), binding.HTTP...)
		result = append(result, binding)
		return nil
	}
	for _, method := range service.Methods {
		if method.Operation == nil {
			continue
		}
		operation := cloneOperationDeclaration(*method.Operation)
		if value := strings.TrimSpace(operation.ApplicationMethod); value != "" && value != method.Name {
			return nil, fmt.Errorf("contract application operation: %s application_method %s does not match RPC method %s", method.FullName, value, method.Name)
		}
		if value := normalizeTypeName(operation.RequestType); value != "" && value != method.Request {
			return nil, fmt.Errorf("contract application operation: %s request_type %s does not match RPC request %s", method.FullName, value, method.Request)
		}
		if value := normalizeTypeName(operation.ResponseType); value != "" && value != method.Response {
			return nil, fmt.Errorf("contract application operation: %s response_type %s does not match RPC response %s", method.FullName, value, method.Response)
		}
		if err := add(applicationOperationBinding{
			MethodName: method.Name, RequestType: method.Request, ResponseType: method.Response,
			Operation: operation, RPC: "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name,
			HTTP: method.HTTP, SourcePath: "service." + service.FullName + ".method." + method.Name,
		}); err != nil {
			return nil, err
		}
	}
	for index, declared := range service.Application.Operations {
		operation := cloneOperationDeclaration(declared)
		if err := add(applicationOperationBinding{
			MethodName: operation.ApplicationMethod, RequestType: operation.RequestType, ResponseType: operation.ResponseType,
			Operation: operation, SourcePath: fmt.Sprintf("service.%s.application.operation[%d]", service.FullName, index),
		}); err != nil {
			return nil, err
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MethodName == result[j].MethodName {
			return result[i].Operation.ID < result[j].Operation.ID
		}
		return result[i].MethodName < result[j].MethodName
	})
	return result, nil
}
