package applicationgraph

import (
	"fmt"
	"strconv"
	"strings"

	"yunka.io/pkg/contract"
)

func AddContract(builder *Builder, manifest contract.Manifest) error {
	if builder == nil {
		return fmt.Errorf("applicationgraph: nil builder")
	}
	manifest = cloneManifest(manifest)
	manifest.Normalize()
	evidence := []Evidence{Declared("contract.manifest", "protobuf contract")}

	for _, file := range manifest.Files {
		if file.Domain == nil || strings.TrimSpace(file.Domain.Name) == "" {
			continue
		}
		attrs := map[string]string{}
		if file.Domain.Version != "" {
			attrs["version"] = file.Domain.Version
		}
		if err := builder.AddNode(Node{ID: ID(NodeDomain, file.Domain.Name), Kind: NodeDomain, Name: file.Domain.Name, Attributes: attrs, Evidence: evidence}); err != nil {
			return err
		}
	}

	for _, message := range manifest.Messages {
		attrs := map[string]string{"fieldCount": strconv.Itoa(len(message.Fields))}
		if message.DTO != nil {
			attrs["dtoKind"] = message.DTO.Kind
		}
		if err := builder.AddNode(Node{ID: ID(NodeMessage, message.FullName), Kind: NodeMessage, Name: message.FullName, Attributes: attrs, Evidence: evidence}); err != nil {
			return err
		}
	}
	for _, enum := range manifest.Enums {
		if err := builder.AddNode(Node{ID: ID(NodeEnum, enum.FullName), Kind: NodeEnum, Name: enum.FullName,
			Attributes: map[string]string{"valueCount": strconv.Itoa(len(enum.Values))}, Evidence: evidence}); err != nil {
			return err
		}
	}
	for _, service := range manifest.Services {
		serviceID := ID(NodeService, service.FullName)
		serviceAttrs := map[string]string{}
		if service.Domain != "" {
			serviceAttrs["domain"] = service.Domain
		}
		if service.Application != nil {
			serviceAttrs["application"] = service.Application.Name
		}
		if err := builder.AddNode(Node{ID: serviceID, Kind: NodeService, Name: service.FullName, Attributes: serviceAttrs, Evidence: evidence}); err != nil {
			return err
		}

		var applicationID string
		if service.Application != nil && service.Domain != "" {
			domainID := ID(NodeDomain, service.Domain)
			if !builder.HasNode(domainID) {
				if err := builder.AddNode(Node{ID: domainID, Kind: NodeDomain, Name: service.Domain, Evidence: evidence}); err != nil {
					return err
				}
			}
			applicationName := service.Domain + "/" + service.Application.Name
			applicationID = ID(NodeApplication, applicationName)
			if err := builder.AddNode(Node{ID: applicationID, Kind: NodeApplication, Name: applicationName,
				Attributes: map[string]string{"domain": service.Domain, "application": service.Application.Name, "service": service.FullName}, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: domainID, To: applicationID, Kind: EdgeContains, Evidence: evidence}); err != nil {
				return err
			}
		}

		for _, method := range service.Methods {
			typedOperation := method.Operation != nil && strings.TrimSpace(method.Operation.ID) != ""
			operationName := method.FullName
			if typedOperation {
				operationName = method.Operation.ID
			}
			operationID := ID(NodeOperation, operationName)
			attrs := map[string]string{
				"rpcPath":         "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name,
				"rpcMethod":       method.FullName,
				"clientStreaming": strconv.FormatBool(method.ClientStreaming),
				"serverStreaming": strconv.FormatBool(method.ServerStreaming),
			}
			if typedOperation {
				attrs["operationId"] = method.Operation.ID
				attrs["useCase"] = method.Operation.UseCase
				attrs["public"] = strconv.FormatBool(method.Operation.Public)
				attrs["tenantRequired"] = strconv.FormatBool(method.Operation.TenantRequired)
				attrs["permissionMode"] = method.Operation.PermissionMode
				if service.Domain != "" {
					attrs["domain"] = service.Domain
				}
				if service.Application != nil {
					attrs["application"] = service.Application.Name
				}
			}
			for key, value := range method.Directives {
				attrs["directive."+key] = value
			}
			if err := builder.AddNode(Node{ID: operationID, Kind: NodeOperation, Name: operationName, Attributes: attrs, Evidence: evidence}); err != nil {
				return err
			}
			if typedOperation {
				if err := builder.AddEdge(Edge{From: serviceID, To: operationID, Kind: EdgeExposes, Evidence: evidence}); err != nil {
					return err
				}
				if applicationID != "" {
					if err := builder.AddEdge(Edge{From: applicationID, To: operationID, Kind: EdgeContains, Evidence: evidence}); err != nil {
						return err
					}
				}
			} else if err := builder.AddEdge(Edge{From: serviceID, To: operationID, Kind: EdgeContains, Evidence: evidence}); err != nil {
				return err
			}
			if method.Request != "" {
				requestID := ID(NodeMessage, method.Request)
				if !builder.HasNode(requestID) {
					if err := builder.AddNode(Node{ID: requestID, Kind: NodeMessage, Name: method.Request, Evidence: evidence}); err != nil {
						return err
					}
				}
				if err := builder.AddEdge(Edge{From: operationID, To: requestID, Kind: EdgeAccepts, Evidence: evidence}); err != nil {
					return err
				}
			}
			if method.Response != "" {
				responseID := ID(NodeMessage, method.Response)
				if !builder.HasNode(responseID) {
					if err := builder.AddNode(Node{ID: responseID, Kind: NodeMessage, Name: method.Response, Evidence: evidence}); err != nil {
						return err
					}
				}
				if err := builder.AddEdge(Edge{From: operationID, To: responseID, Kind: EdgeReturns, Evidence: evidence}); err != nil {
					return err
				}
			}
			if typedOperation {
				for _, permission := range method.Operation.Permissions {
					permission = strings.TrimSpace(permission)
					if permission == "" {
						continue
					}
					permissionID := ID(NodePermission, permission)
					if err := builder.AddNode(Node{ID: permissionID, Kind: NodePermission, Name: permission, Evidence: evidence}); err != nil {
						return err
					}
					if err := builder.AddEdge(Edge{From: operationID, To: permissionID, Kind: EdgeRequires, Evidence: evidence}); err != nil {
						return err
					}
				}
			}
			for _, binding := range method.HTTP {
				name := strings.ToUpper(strings.TrimSpace(binding.Method)) + " " + strings.TrimSpace(binding.Path)
				routeID := ID(NodeHTTPRoute, name)
				routeAttrs := map[string]string{"method": strings.ToUpper(binding.Method), "path": binding.Path}
				if binding.Body != "" {
					routeAttrs["body"] = binding.Body
				}
				if binding.ResponseBody != "" {
					routeAttrs["responseBody"] = binding.ResponseBody
				}
				if err := builder.AddNode(Node{ID: routeID, Kind: NodeHTTPRoute, Name: name, Attributes: routeAttrs, Evidence: evidence}); err != nil {
					return err
				}
				if err := builder.AddEdge(Edge{From: routeID, To: operationID, Kind: EdgeRoutesTo, Evidence: evidence}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cloneManifest(manifest contract.Manifest) contract.Manifest {
	clone := manifest
	clone.Files = make([]contract.File, len(manifest.Files))
	for i, file := range manifest.Files {
		clone.Files[i] = file
		if file.Domain != nil {
			value := *file.Domain
			clone.Files[i].Domain = &value
		}
	}
	clone.Messages = make([]contract.Message, len(manifest.Messages))
	for i, message := range manifest.Messages {
		clone.Messages[i] = message
		clone.Messages[i].Fields = append([]contract.Field(nil), message.Fields...)
		if message.DTO != nil {
			value := *message.DTO
			clone.Messages[i].DTO = &value
		}
	}
	clone.Enums = make([]contract.Enum, len(manifest.Enums))
	for i, enum := range manifest.Enums {
		clone.Enums[i] = enum
		clone.Enums[i].Values = append([]contract.EnumValue(nil), enum.Values...)
	}
	clone.Services = make([]contract.Service, len(manifest.Services))
	for i, service := range manifest.Services {
		clone.Services[i] = service
		if service.Application != nil {
			value := *service.Application
			clone.Services[i].Application = &value
		}
		clone.Services[i].Methods = make([]contract.Method, len(service.Methods))
		for j, method := range service.Methods {
			clone.Services[i].Methods[j] = method
			clone.Services[i].Methods[j].HTTP = append([]contract.HTTPBinding(nil), method.HTTP...)
			if method.Directives != nil {
				clone.Services[i].Methods[j].Directives = make(map[string]string, len(method.Directives))
				for key, value := range method.Directives {
					clone.Services[i].Methods[j].Directives[key] = value
				}
			}
			if method.Operation != nil {
				value := *method.Operation
				value.Permissions = append([]string(nil), method.Operation.Permissions...)
				value.Authentication = append([]string(nil), method.Operation.Authentication...)
				clone.Services[i].Methods[j].Operation = &value
			}
			if method.Authorization != nil {
				value := *method.Authorization
				value.Permissions = append([]string(nil), method.Authorization.Permissions...)
				value.Authentication = append([]string(nil), method.Authorization.Authentication...)
				clone.Services[i].Methods[j].Authorization = &value
			}
		}
	}
	return clone
}
