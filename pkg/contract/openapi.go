package contract

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type OpenAPIOptions struct {
	Title   string
	Version string
}

var pathVariablePattern = regexp.MustCompile(`\{([^}=]+)(?:=[^}]*)?\}`)

func GenerateOpenAPI(manifest Manifest, options OpenAPIOptions) ([]byte, error) {
	if options.Title == "" {
		options.Title = "yunka API"
	}
	if options.Version == "" {
		options.Version = "0.0.0"
	}
	messages := messageIndex(manifest)
	enums := enumIndex(manifest)
	external := collectExternalContractTypes(manifest)
	components := make(map[string]any)
	for _, item := range manifest.Enums {
		if _, visible := external.enums[item.FullName]; !visible {
			continue
		}
		values := make([]string, 0, len(item.Values))
		for _, value := range item.Values {
			values = append(values, value.Name)
		}
		components[schemaName(item.FullName)] = map[string]any{
			"type":                    "string",
			"enum":                    values,
			"x-yunka-proto-full-name": item.FullName,
		}
	}
	for _, item := range manifest.Messages {
		if _, visible := external.messages[item.FullName]; !visible {
			continue
		}
		properties := make(map[string]any)
		for _, field := range item.Fields {
			properties[field.JSONName] = fieldSchema(field, enums)
		}
		components[schemaName(item.FullName)] = map[string]any{
			"type":                    "object",
			"properties":              properties,
			"x-yunka-proto-full-name": item.FullName,
		}
	}

	paths := make(map[string]any)
	var unbound []map[string]any
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if len(method.HTTP) == 0 {
				unbound = append(unbound, map[string]any{
					"operationId": method.FullName,
					"rpcPath":     rpcPath(service.FullName, method.Name),
					"request":     method.Request,
					"response":    method.Response,
				})
				continue
			}
			for bindingIndex, binding := range method.HTTP {
				operationID := method.FullName
				if bindingIndex > 0 {
					operationID = fmt.Sprintf("%s.binding%d", method.FullName, bindingIndex+1)
				}
				responseSchema := typeSchema(method.Response, "message", enums)
				if binding.ResponseBody != "" {
					if response, ok := messages[method.Response]; ok {
						if field, found := findField(response, binding.ResponseBody); found {
							responseSchema = fieldSchema(field, enums)
						}
					}
				}
				operation := map[string]any{
					"operationId": operationID,
					"tags":        []string{service.FullName},
					"x-yunka-rpc": rpcPath(service.FullName, method.Name),
					"responses": map[string]any{
						"200": map[string]any{
							"description": "Successful response",
							"content": map[string]any{
								"application/json": map[string]any{"schema": responseSchema},
							},
						},
					},
				}
				if len(method.Directives) > 0 {
					operation["x-yunka-directives"] = method.Directives
				}
				request, hasRequest := messages[method.Request]
				if hasRequest {
					applyHTTPParameters(operation, binding, request, enums)
				}
				pathItem, _ := paths[binding.Path].(map[string]any)
				if pathItem == nil {
					pathItem = make(map[string]any)
					paths[binding.Path] = pathItem
				}
				pathItem[strings.ToLower(binding.Method)] = operation
			}
		}
	}
	sort.Slice(unbound, func(i, j int) bool {
		return unbound[i]["operationId"].(string) < unbound[j]["operationId"].(string)
	})

	document := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   options.Title,
			"version": options.Version,
		},
		"paths": paths,
		"components": map[string]any{
			"schemas": components,
		},
		"x-yunka-contract-version": manifest.SchemaVersion,
		"x-yunka-rpc-methods":      unbound,
	}
	return marshalJSON(document)
}

func applyHTTPParameters(operation map[string]any, binding HTTPBinding, request Message, enums map[string]Enum) {
	pathFields := make(map[string]struct{})
	var parameters []any
	for _, match := range pathVariablePattern.FindAllStringSubmatch(binding.Path, -1) {
		if len(match) < 2 {
			continue
		}
		fieldName := strings.Split(match[1], ".")[0]
		pathFields[fieldName] = struct{}{}
		if field, ok := findField(request, fieldName); ok {
			parameters = append(parameters, map[string]any{
				"name":     field.JSONName,
				"in":       "path",
				"required": true,
				"schema":   fieldSchema(field, enums),
			})
		}
	}
	if binding.Body != "" {
		var schema any
		if binding.Body == "*" {
			schema = map[string]any{"$ref": "#/components/schemas/" + schemaName(request.FullName)}
		} else if field, ok := findField(request, binding.Body); ok {
			schema = fieldSchema(field, enums)
		}
		if schema != nil {
			operation["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{"schema": schema},
				},
			}
		}
	}
	for _, field := range request.Fields {
		if _, ok := pathFields[field.Name]; ok {
			continue
		}
		if binding.Body == "*" || binding.Body == field.Name || binding.Body == field.JSONName {
			continue
		}
		if field.Kind == "message" || field.Kind == "map" {
			continue
		}
		parameters = append(parameters, map[string]any{
			"name":     field.JSONName,
			"in":       "query",
			"required": field.Required,
			"schema":   fieldSchema(field, enums),
		})
	}
	if len(parameters) > 0 {
		operation["parameters"] = parameters
	}
}

func fieldSchema(field Field, enums map[string]Enum) any {
	var schema any
	if field.Map {
		valueField := Field{Kind: field.MapValueKind, Type: field.MapValueType}
		schema = map[string]any{
			"type":                 "object",
			"additionalProperties": fieldSchema(valueField, enums),
		}
	} else {
		schema = typeSchema(field.Type, field.Kind, enums)
	}
	if field.Repeated {
		return map[string]any{"type": "array", "items": schema}
	}
	return schema
}

func typeSchema(typeName, kind string, enums map[string]Enum) any {
	if kind == "message" {
		if schema, ok := wellKnownOpenAPISchema(typeName); ok {
			return schema
		}
		return map[string]any{"$ref": "#/components/schemas/" + schemaName(typeName)}
	}
	if kind == "enum" {
		return map[string]any{"$ref": "#/components/schemas/" + schemaName(typeName)}
	}
	switch typeName {
	case "double":
		return map[string]any{"type": "number", "format": "double"}
	case "float":
		return map[string]any{"type": "number", "format": "float"}
	case "int32", "sint32", "sfixed32":
		return map[string]any{"type": "integer", "format": "int32"}
	case "uint32", "fixed32":
		return map[string]any{"type": "integer", "minimum": 0}
	case "int64", "sint64", "sfixed64", "uint64", "fixed64":
		return map[string]any{"type": "string", "format": typeName, "x-yunka-protobuf-json-integer": true}
	case "bool":
		return map[string]any{"type": "boolean"}
	case "bytes":
		return map[string]any{"type": "string", "contentEncoding": "base64"}
	case "string":
		return map[string]any{"type": "string"}
	default:
		_ = enums
		return map[string]any{}
	}
}

func wellKnownOpenAPISchema(name string) (any, bool) {
	switch name {
	case "google.protobuf.Timestamp":
		return map[string]any{"type": "string", "format": "date-time"}, true
	case "google.protobuf.Duration":
		return map[string]any{"type": "string", "format": "duration"}, true
	case "google.protobuf.Empty":
		return map[string]any{"type": "object"}, true
	case "google.protobuf.Struct", "google.protobuf.Value", "google.protobuf.ListValue", "google.protobuf.Any":
		return map[string]any{}, true
	default:
		return nil, false
	}
}

func schemaName(fullName string) string {
	replacer := strings.NewReplacer(".", "_", "/", "_", "-", "_")
	return replacer.Replace(strings.TrimPrefix(fullName, "."))
}

func rpcPath(service, method string) string {
	return "/" + strings.TrimPrefix(service, ".") + "/" + method
}

func messageIndex(manifest Manifest) map[string]Message {
	result := make(map[string]Message, len(manifest.Messages))
	for _, item := range manifest.Messages {
		result[item.FullName] = item
	}
	return result
}

func enumIndex(manifest Manifest) map[string]Enum {
	result := make(map[string]Enum, len(manifest.Enums))
	for _, item := range manifest.Enums {
		result[item.FullName] = item
	}
	return result
}

func findField(message Message, name string) (Field, bool) {
	for _, field := range message.Fields {
		if field.Name == name || field.JSONName == name {
			return field, true
		}
	}
	return Field{}, false
}

func marshalJSON(value any) ([]byte, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
