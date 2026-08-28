package contract

// externalContractTypes is the transport-visible protobuf type closure. The
// canonical Manifest may contain DTOs used only by application-level internal
// Operations; those types remain available to Application codegen and the
// OperationPlan compiler but must not leak into OpenAPI or generated transport
// clients unless a real protobuf service method reaches them.
type externalContractTypes struct {
	messages map[string]struct{}
	enums    map[string]struct{}
}

func collectExternalContractTypes(manifest Manifest) externalContractTypes {
	messages := messageIndex(manifest)
	result := externalContractTypes{
		messages: make(map[string]struct{}),
		enums:    make(map[string]struct{}),
	}
	var visitMessage func(string)
	visitMessage = func(name string) {
		name = normalizeTypeName(name)
		if name == "" {
			return
		}
		if _, seen := result.messages[name]; seen {
			return
		}
		message, ok := messages[name]
		if !ok {
			return
		}
		result.messages[name] = struct{}{}
		for _, field := range message.Fields {
			switch field.Kind {
			case "message":
				visitMessage(field.Type)
			case "enum":
				result.enums[normalizeTypeName(field.Type)] = struct{}{}
			}
			if !field.Map {
				continue
			}
			switch field.MapValueKind {
			case "message":
				visitMessage(field.MapValueType)
			case "enum":
				result.enums[normalizeTypeName(field.MapValueType)] = struct{}{}
			}
		}
	}
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			visitMessage(method.Request)
			visitMessage(method.Response)
		}
	}
	return result
}
