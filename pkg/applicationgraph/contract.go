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
	for _, message := range manifest.Messages {
		if err := builder.AddNode(Node{ID: ID(NodeMessage, message.FullName), Kind: NodeMessage, Name: message.FullName,
			Attributes: map[string]string{"fieldCount": strconv.Itoa(len(message.Fields))}, Evidence: evidence}); err != nil {
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
		if err := builder.AddNode(Node{ID: serviceID, Kind: NodeService, Name: service.FullName, Evidence: evidence}); err != nil {
			return err
		}
		for _, method := range service.Methods {
			operationID := ID(NodeOperation, method.FullName)
			attrs := map[string]string{
				"rpcPath":         "/" + strings.TrimPrefix(service.FullName, ".") + "/" + method.Name,
				"clientStreaming": strconv.FormatBool(method.ClientStreaming),
				"serverStreaming": strconv.FormatBool(method.ServerStreaming),
			}
			for key, value := range method.Directives {
				attrs["directive."+key] = value
			}
			if err := builder.AddNode(Node{ID: operationID, Kind: NodeOperation, Name: method.FullName, Attributes: attrs, Evidence: evidence}); err != nil {
				return err
			}
			if err := builder.AddEdge(Edge{From: serviceID, To: operationID, Kind: EdgeContains, Evidence: evidence}); err != nil {
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
	clone.Files = append([]contract.File(nil), manifest.Files...)
	clone.Messages = make([]contract.Message, len(manifest.Messages))
	for i, message := range manifest.Messages {
		clone.Messages[i] = message
		clone.Messages[i].Fields = append([]contract.Field(nil), message.Fields...)
	}
	clone.Enums = make([]contract.Enum, len(manifest.Enums))
	for i, enum := range manifest.Enums {
		clone.Enums[i] = enum
		clone.Enums[i].Values = append([]contract.EnumValue(nil), enum.Values...)
	}
	clone.Services = make([]contract.Service, len(manifest.Services))
	for i, service := range manifest.Services {
		clone.Services[i] = service
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
		}
	}
	return clone
}
