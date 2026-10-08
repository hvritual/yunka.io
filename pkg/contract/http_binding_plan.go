package contract

import "fmt"

// httpBindingPlan is a disposable projection of PB field ownership, not another
// writable contract. Only typed Application adapters use this support boundary;
// untyped inventories remain descriptive and retain their legacy artifact ABI.
type httpBindingPlan struct {
	Body  string
	Path  []httpPathField
	Query []Field
}

type httpPathField struct {
	Name  string // Exact template variable; it need not equal the JSON spelling.
	Field Field
}

func compileHTTPBindingPlan(method Method, binding HTTPBinding, messages map[string]Message) (httpBindingPlan, error) {
	plan := httpBindingPlan{Body: binding.Body}
	unsupported := func(field, reason string) (httpBindingPlan, error) {
		operation := method.FullName
		if method.Operation != nil {
			operation = method.Operation.ID
		}
		return httpBindingPlan{}, fmt.Errorf("UNSUPPORTED_HTTP_BINDING: operation %q (%s), %s %q, field %q: %s", operation, method.FullName, binding.Method, binding.Path, field, reason)
	}
	if err := validateHTTPMethod(binding.Method); err != nil {
		return unsupported("", err.Error())
	}
	if err := validateC9HTTPBindingPath(binding.Path); err != nil {
		return unsupported("", err.Error())
	}
	if binding.Body != "" && binding.Body != "*" {
		return unsupported(binding.Body, "named body requires handwritten mapping; use a supported explicit binding")
	}
	if binding.ResponseBody != "" {
		return unsupported(binding.ResponseBody, "response_body requires handwritten mapping")
	}
	request, found := messages[method.Request]
	if !found && method.Request != "google.protobuf.Empty" {
		return unsupported("", "request fields are unavailable: "+method.Request)
	}
	aliases := make(map[string]string)
	for _, field := range request.Fields {
		for _, alias := range []string{field.Name, httpJSONName(field)} {
			if previous, ok := aliases[alias]; ok && previous != field.Name {
				return unsupported(field.Name, "ambiguous protobuf/JSON name also owned by "+previous)
			}
			aliases[alias] = field.Name
		}
	}
	pathFields, err := simplePathFields(binding.Path)
	if err != nil {
		return unsupported("", err.Error())
	}
	pathSet := make(map[string]bool)
	for _, name := range pathFields {
		field, ok := findMessageField(request, name)
		if !ok {
			return unsupported(name, "path field not found in "+method.Request)
		}
		if pathSet[field.Name] {
			return unsupported(name, "field is bound to the path more than once")
		}
		if field.Repeated || !httpScalarSupported(field) {
			return unsupported(name, "path requires a non-repeated scalar outside a oneof")
		}
		pathSet[field.Name] = true
		plan.Path = append(plan.Path, httpPathField{Name: name, Field: field})
	}
	for _, field := range request.Fields {
		if pathSet[field.Name] || binding.Body == "*" {
			continue
		}
		if !httpScalarSupported(field) {
			return unsupported(field.Name, "query supports scalar or repeated scalar fields only; enum/message/map/oneof requires an explicit supported binding")
		}
		plan.Query = append(plan.Query, field)
	}
	return plan, nil
}

func httpScalarSupported(field Field) bool {
	if field.Kind != "scalar" || field.Map || field.Oneof {
		return false
	}
	switch field.Type {
	case "string", "bytes", "bool", "int32", "int64", "sint32", "sint64", "sfixed32", "sfixed64", "uint32", "uint64", "fixed32", "fixed64", "float", "double":
		return true
	default:
		return false
	}
}

func httpJSONName(field Field) string {
	if field.JSONName != "" {
		return field.JSONName
	}
	return field.Name
}

// validateApplicationHTTPBindings also runs at projection entry points. A
// caller rendering OpenAPI or TypeScript without Lint must not publish a typed
// HTTP feature that the generated runtime will ignore or cannot implement.
func validateApplicationHTTPBindings(manifest Manifest) error {
	messages := messageIndex(manifest)
	for _, service := range manifest.Services {
		if service.Application == nil {
			continue
		}
		for _, method := range service.Methods {
			for _, binding := range method.HTTP {
				if _, err := compileHTTPBindingPlan(method, binding, messages); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
