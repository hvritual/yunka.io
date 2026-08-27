package contract

import (
	"fmt"
	"sort"
	"strings"
)

const (
	dslFileDomainOption      = 51001
	dslMessageDTOOption      = 51002
	dslServiceApplicationOpt = 51003
	dslMethodOperationOption = 51004
)

type dslServiceDeclaration struct {
	Domain      string
	Application *ApplicationDeclaration
}

type dslDeclarations struct {
	Files    map[string]*DomainDeclaration
	Messages map[string]*DTODeclaration
	Services map[string]dslServiceDeclaration
	Methods  map[string]*OperationDeclaration
}

func parseDSLDeclarations(data []byte) (dslDeclarations, error) {
	result := dslDeclarations{
		Files:    map[string]*DomainDeclaration{},
		Messages: map[string]*DTODeclaration{},
		Services: map[string]dslServiceDeclaration{},
		Methods:  map[string]*OperationDeclaration{},
	}
	err := scanWire(data, func(field wireField) error {
		if field.Number != 1 || field.Type != 2 {
			return nil
		}
		return parseDSLFile(field.Bytes, &result)
	})
	return result, err
}

func parseDSLFile(data []byte, out *dslDeclarations) error {
	var name, pkg string
	var options []byte
	var messages, services [][]byte
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			name = string(field.Bytes)
		case 2:
			pkg = string(field.Bytes)
		case 4:
			messages = append(messages, append([]byte(nil), field.Bytes...))
		case 6:
			services = append(services, append([]byte(nil), field.Bytes...))
		case 8:
			options = append([]byte(nil), field.Bytes...)
		}
		return nil
	}); err != nil {
		return err
	}
	domain, err := parseDomainDeclaration(optionPayload(options, dslFileDomainOption))
	if err != nil {
		return fmt.Errorf("contract: %s domain option: %w", name, err)
	}
	if domain != nil {
		out.Files[name] = domain
	}
	for _, raw := range messages {
		if err := parseDSLMessage(raw, pkg, "", out); err != nil {
			return err
		}
	}
	for _, raw := range services {
		if err := parseDSLService(raw, pkg, domain, out); err != nil {
			return err
		}
	}
	return nil
}

func parseDSLMessage(data []byte, pkg, parent string, out *dslDeclarations) error {
	var name string
	var options []byte
	var nested [][]byte
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			name = string(field.Bytes)
		case 3:
			nested = append(nested, append([]byte(nil), field.Bytes...))
		case 7:
			options = append([]byte(nil), field.Bytes...)
		}
		return nil
	}); err != nil {
		return err
	}
	full := fullName(pkg, parent, name)
	dto, err := parseDTODeclaration(optionPayload(options, dslMessageDTOOption))
	if err != nil {
		return fmt.Errorf("contract: %s dto option: %w", full, err)
	}
	if dto != nil {
		out.Messages[full] = dto
	}
	nextParent := name
	if parent != "" {
		nextParent = parent + "." + name
	}
	for _, raw := range nested {
		if err := parseDSLMessage(raw, pkg, nextParent, out); err != nil {
			return err
		}
	}
	return nil
}

func parseDSLService(data []byte, pkg string, domain *DomainDeclaration, out *dslDeclarations) error {
	var name string
	var options []byte
	var methods [][]byte
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			name = string(field.Bytes)
		case 2:
			methods = append(methods, append([]byte(nil), field.Bytes...))
		case 3:
			options = append([]byte(nil), field.Bytes...)
		}
		return nil
	}); err != nil {
		return err
	}
	full := fullName(pkg, "", name)
	application, err := parseApplicationDeclaration(optionPayload(options, dslServiceApplicationOpt))
	if err != nil {
		return fmt.Errorf("contract: %s application option: %w", full, err)
	}
	service := dslServiceDeclaration{Application: application}
	if domain != nil {
		service.Domain = domain.Name
	}
	if application != nil || service.Domain != "" {
		out.Services[full] = service
	}
	for _, raw := range methods {
		if err := parseDSLMethod(raw, full, out); err != nil {
			return err
		}
	}
	return nil
}

func parseDSLMethod(data []byte, serviceFullName string, out *dslDeclarations) error {
	var name string
	var options []byte
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			name = string(field.Bytes)
		case 4:
			options = append([]byte(nil), field.Bytes...)
		}
		return nil
	}); err != nil {
		return err
	}
	full := serviceFullName + "." + name
	operation, err := parseOperationDeclaration(optionPayload(options, dslMethodOperationOption))
	if err != nil {
		return fmt.Errorf("contract: %s operation option: %w", full, err)
	}
	if operation != nil {
		out.Methods[full] = operation
	}
	return nil
}

func optionPayload(options []byte, number int) []byte {
	if len(options) == 0 {
		return nil
	}
	var payload []byte
	_ = scanWire(options, func(field wireField) error {
		if field.Number == number && field.Type == 2 {
			payload = append([]byte(nil), field.Bytes...)
		}
		return nil
	})
	return payload
}

func parseDomainDeclaration(data []byte) (*DomainDeclaration, error) {
	if len(data) == 0 {
		return nil, nil
	}
	result := &DomainDeclaration{}
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			result.Name = string(field.Bytes)
		case 2:
			result.Version = string(field.Bytes)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func parseDTODeclaration(data []byte) (*DTODeclaration, error) {
	if len(data) == 0 {
		return nil, nil
	}
	result := &DTODeclaration{Kind: "unspecified"}
	if err := scanWire(data, func(field wireField) error {
		if field.Number == 1 && field.Type == 0 {
			result.Kind = dtoKindName(field.Varint)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}

func parseApplicationDeclaration(data []byte) (*ApplicationDeclaration, error) {
	if len(data) == 0 {
		return nil, nil
	}
	result := &ApplicationDeclaration{}
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			result.Name = string(field.Bytes)
		case 2:
			result.Requires = append(result.Requires, string(field.Bytes))
		}
		return nil
	}); err != nil {
		return nil, err
	}
	result.Requires = stableStrings(result.Requires)
	return result, nil
}

func parseOperationDeclaration(data []byte) (*OperationDeclaration, error) {
	if len(data) == 0 {
		return nil, nil
	}
	result := &OperationDeclaration{PermissionMode: "all"}
	if err := scanWire(data, func(field wireField) error {
		switch field.Number {
		case 1:
			result.ID = string(field.Bytes)
		case 2:
			result.UseCase = string(field.Bytes)
		case 3:
			result.Permissions = append(result.Permissions, string(field.Bytes))
		case 4:
			if field.Type == 0 && field.Varint == 1 {
				result.PermissionMode = "any"
			}
		case 5:
			if field.Type == 0 {
				result.TenantRequired = field.Varint != 0
			}
		case 6:
			switch field.Type {
			case 0:
				if name := authenticationName(field.Varint); name != "" {
					result.Authentication = append(result.Authentication, name)
				}
			case 2:
				values, err := packedInt32(field.Bytes)
				if err != nil {
					return err
				}
				for _, value := range values {
					if name := authenticationName(uint64(value)); name != "" {
						result.Authentication = append(result.Authentication, name)
					}
				}
			}
		case 7:
			if field.Type == 0 {
				result.Public = field.Varint != 0
			}
		case 8:
			result.RequiresOperations = append(result.RequiresOperations, string(field.Bytes))
		case 9:
			if field.Type == 0 {
				result.Composition = compositionBoundaryName(field.Varint)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	result.Permissions = stableStrings(result.Permissions)
	result.Authentication = stableStrings(result.Authentication)
	result.RequiresOperations = stableStrings(result.RequiresOperations)
	return result, nil
}

func compositionBoundaryName(value uint64) string {
	switch value {
	case 1:
		return "local"
	case 2:
		return "remote_saga"
	default:
		return ""
	}
}

func dtoKindName(value uint64) string {
	switch value {
	case 1:
		return "input"
	case 2:
		return "output"
	case 3:
		return "shared"
	case 4:
		return "event"
	case 5:
		return "value_object"
	default:
		return "unspecified"
	}
}

func authenticationName(value uint64) string {
	switch value {
	case 1:
		return "jwt"
	case 2:
		return "api-key"
	case 3:
		return "service-token"
	default:
		return ""
	}
}

func isDSLSupportFile(name string) bool {
	return strings.HasPrefix(strings.TrimSpace(name), "yunka/dsl/")
}

func applyDSLDeclarations(manifest *Manifest, data []byte) error {
	declarations, err := parseDSLDeclarations(data)
	if err != nil {
		return err
	}
	for i := range manifest.Files {
		if domain := declarations.Files[manifest.Files[i].Name]; domain != nil {
			copy := *domain
			manifest.Files[i].Domain = &copy
		}
	}
	for i := range manifest.Messages {
		if dto := declarations.Messages[manifest.Messages[i].FullName]; dto != nil {
			copy := *dto
			manifest.Messages[i].DTO = &copy
		}
	}
	for i := range manifest.Services {
		if declaration, ok := declarations.Services[manifest.Services[i].FullName]; ok {
			manifest.Services[i].Domain = declaration.Domain
			if declaration.Application != nil {
				copy := *declaration.Application
				copy.Requires = append([]string(nil), declaration.Application.Requires...)
				manifest.Services[i].Application = &copy
			}
		}
		for j := range manifest.Services[i].Methods {
			method := &manifest.Services[i].Methods[j]
			if operation := declarations.Methods[method.FullName]; operation != nil {
				copy := *operation
				copy.Permissions = append([]string(nil), operation.Permissions...)
				copy.Authentication = append([]string(nil), operation.Authentication...)
				copy.RequiresOperations = append([]string(nil), operation.RequiresOperations...)
				typedPolicy := authorizationFromOperation(&copy)
				if legacyPolicy := authorizationFromDirectives(method.Directives); legacyPolicy != nil && authorizationKey(legacyPolicy) != authorizationKey(typedPolicy) {
					return fmt.Errorf("contract: %s typed operation conflicts with legacy @yunka authorization directives", method.FullName)
				}
				method.Operation = &copy
				method.Authorization = typedPolicy
			}
		}
	}
	inferTypedDTOs(manifest)
	manifest.Normalize()
	return nil
}

func inferTypedDTOs(manifest *Manifest) {
	if manifest == nil {
		return
	}
	index := make(map[string]int, len(manifest.Messages))
	for i := range manifest.Messages {
		index[manifest.Messages[i].FullName] = i
	}
	type dtoUsage uint8
	const (
		dtoInput dtoUsage = 1 << iota
		dtoOutput
	)
	usage := make(map[string]dtoUsage)
	var visit func(string, dtoUsage, map[string]struct{})
	visit = func(name string, role dtoUsage, active map[string]struct{}) {
		if knownExternalType(name) {
			return
		}
		position, ok := index[name]
		if !ok {
			return
		}
		usage[name] |= role
		if _, cycle := active[name]; cycle {
			return
		}
		next := make(map[string]struct{}, len(active)+1)
		for key := range active {
			next[key] = struct{}{}
		}
		next[name] = struct{}{}
		for _, field := range manifest.Messages[position].Fields {
			if field.Kind == "message" {
				visit(field.Type, role, next)
			}
			if field.Map && field.MapValueKind == "message" {
				visit(field.MapValueType, role, next)
			}
		}
	}
	for _, service := range manifest.Services {
		if service.Application == nil || service.Domain == "" {
			continue
		}
		for _, method := range service.Methods {
			visit(method.Request, dtoInput, map[string]struct{}{})
			visit(method.Response, dtoOutput, map[string]struct{}{})
		}
	}
	for name, role := range usage {
		position := index[name]
		if manifest.Messages[position].DTO != nil {
			continue
		}
		kind := "shared"
		switch role {
		case dtoInput:
			kind = "input"
		case dtoOutput:
			kind = "output"
		}
		manifest.Messages[position].DTO = &DTODeclaration{Kind: kind}
	}
}

func authorizationFromOperation(operation *OperationDeclaration) *AuthorizationPolicy {
	if operation == nil || operation.Public {
		return nil
	}
	return &AuthorizationPolicy{
		OperationID:    operation.ID,
		Permissions:    append([]string(nil), operation.Permissions...),
		PermissionMode: operation.PermissionMode,
		TenantRequired: operation.TenantRequired,
		Authentication: append([]string(nil), operation.Authentication...),
	}
}

func operationKey(operation *OperationDeclaration) string {
	if operation == nil {
		return ""
	}
	permissions := stableStrings(operation.Permissions)
	authentication := stableStrings(operation.Authentication)
	requires := stableStrings(operation.RequiresOperations)
	return strings.Join([]string{
		operation.ID,
		operation.UseCase,
		operation.PermissionMode,
		fmt.Sprint(operation.TenantRequired),
		fmt.Sprint(operation.Public),
		strings.Join(permissions, ","),
		strings.Join(authentication, ","),
		strings.Join(requires, ","),
		operation.Composition,
	}, "|")
}

func sortedOperationIDs(manifest Manifest) []string {
	var ids []string
	for _, service := range manifest.Services {
		for _, method := range service.Methods {
			if method.Operation != nil && strings.TrimSpace(method.Operation.ID) != "" {
				ids = append(ids, strings.TrimSpace(method.Operation.ID))
			}
		}
	}
	sort.Strings(ids)
	return ids
}
