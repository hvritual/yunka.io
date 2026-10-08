package contract

// applyPlannedHTTPParameters renders only the ownership accepted by the typed
// adapter compiler. Query aliases describe one logical parameter, not two
// independent inputs, and path names match the literal route placeholders.
func applyPlannedHTTPParameters(operation map[string]any, plan httpBindingPlan, request Message, enums map[string]Enum) {
	var parameters []any
	pathFields := make(map[string]bool)
	for _, path := range plan.Path {
		pathFields[path.Field.Name] = true
		parameters = append(parameters, map[string]any{
			"name": path.Name, "in": "path", "required": true,
			"schema": fieldSchema(path.Field, enums),
		})
	}
	if plan.Body == "*" {
		properties := make(map[string]any)
		for _, field := range request.Fields {
			if !pathFields[field.Name] {
				properties[httpJSONName(field)] = fieldSchema(field, enums)
			}
		}
		operation["requestBody"] = map[string]any{
			"required": true,
			"content": map[string]any{"application/json": map[string]any{
				"schema": map[string]any{"type": "object", "properties": properties},
			}},
		}
	}
	for _, field := range plan.Query {
		parameter := map[string]any{
			"name": httpJSONName(field), "in": "query", "required": field.Required,
			"schema":                fieldSchema(field, enums),
			"x-yunka-protobuf-name": field.Name,
		}
		if field.Repeated {
			parameter["style"], parameter["explode"] = "form", true
		}
		parameters = append(parameters, parameter)
	}
	if len(parameters) > 0 {
		operation["parameters"] = parameters
	}
}
