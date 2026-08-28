package contract

import (
	"fmt"
	"sort"
	"strings"
)

type ChangeSeverity string

const (
	ChangeBreaking ChangeSeverity = "breaking"
	ChangeWarning  ChangeSeverity = "warning"
	ChangeInfo     ChangeSeverity = "info"
)

type Change struct {
	Severity ChangeSeverity `json:"severity"`
	Kind     string         `json:"kind"`
	Path     string         `json:"path"`
	Message  string         `json:"message"`
}

type Diff struct {
	Changes []Change `json:"changes"`
}

func (diff Diff) HasBreaking() bool {
	for _, change := range diff.Changes {
		if change.Severity == ChangeBreaking {
			return true
		}
	}
	return false
}

func Compare(baseline, current Manifest) Diff {
	baseline.Normalize()
	current.Normalize()
	var changes []Change

	changes = append(changes, compareFileDeclarations(baseline, current)...)

	baseMessages := messageIndex(baseline)
	currentMessages := messageIndex(current)
	for name, oldMessage := range baseMessages {
		newMessage, ok := currentMessages[name]
		if !ok {
			changes = append(changes, breaking("message_removed", "message."+name, "message removed"))
			continue
		}
		changes = append(changes, compareMessage(oldMessage, newMessage)...)
	}
	for name := range currentMessages {
		if _, ok := baseMessages[name]; !ok {
			changes = append(changes, info("message_added", "message."+name, "message added"))
		}
	}

	baseEnums := enumIndex(baseline)
	currentEnums := enumIndex(current)
	for name, oldEnum := range baseEnums {
		newEnum, ok := currentEnums[name]
		if !ok {
			changes = append(changes, breaking("enum_removed", "enum."+name, "enum removed"))
			continue
		}
		changes = append(changes, compareEnum(oldEnum, newEnum)...)
	}
	for name := range currentEnums {
		if _, ok := baseEnums[name]; !ok {
			changes = append(changes, info("enum_added", "enum."+name, "enum added"))
		}
	}

	baseServices := serviceIndex(baseline)
	currentServices := serviceIndex(current)
	for name, oldService := range baseServices {
		newService, ok := currentServices[name]
		if !ok {
			changes = append(changes, breaking("service_removed", "service."+name, "service removed"))
			continue
		}
		changes = append(changes, compareService(oldService, newService)...)
	}
	for name := range currentServices {
		if _, ok := baseServices[name]; !ok {
			changes = append(changes, info("service_added", "service."+name, "service added"))
		}
	}

	sort.Slice(changes, func(i, j int) bool {
		left, right := changes[i], changes[j]
		if left.Severity != right.Severity {
			return changeSeverityRank(left.Severity) < changeSeverityRank(right.Severity)
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.Message < right.Message
	})
	return Diff{Changes: changes}
}

func compareFileDeclarations(baseline, current Manifest) []Change {
	base := make(map[string]File, len(baseline.Files))
	now := make(map[string]File, len(current.Files))
	for _, file := range baseline.Files {
		base[file.Name] = file
	}
	for _, file := range current.Files {
		now[file.Name] = file
	}
	var changes []Change
	for name, oldFile := range base {
		newFile, ok := now[name]
		if !ok {
			if oldFile.Domain != nil {
				changes = append(changes, breaking("domain_file_removed", "file."+name, "file owning typed domain declaration removed"))
			}
			continue
		}
		path := "file." + name + ".domain"
		switch {
		case oldFile.Domain == nil && newFile.Domain != nil:
			changes = append(changes, info("domain_declared", path, "typed domain declaration added"))
		case oldFile.Domain != nil && newFile.Domain == nil:
			changes = append(changes, breaking("domain_removed", path, "typed domain declaration removed"))
		case oldFile.Domain != nil && newFile.Domain != nil:
			if oldFile.Domain.Name != newFile.Domain.Name {
				changes = append(changes, breaking("domain_identity_changed", path, fmt.Sprintf("domain identity changed from %s to %s", oldFile.Domain.Name, newFile.Domain.Name)))
			}
			if oldFile.Domain.Version != newFile.Domain.Version {
				changes = append(changes, warning("domain_version_changed", path, fmt.Sprintf("domain version changed from %s to %s", oldFile.Domain.Version, newFile.Domain.Version)))
			}
		}
	}
	return changes
}

func compareMessage(oldMessage, newMessage Message) []Change {
	var changes []Change
	pathRoot := "message." + oldMessage.FullName
	switch {
	case oldMessage.DTO == nil && newMessage.DTO != nil:
		changes = append(changes, info("dto_declared", pathRoot+".dto", "DTO classification added: "+newMessage.DTO.Kind))
	case oldMessage.DTO != nil && newMessage.DTO == nil:
		changes = append(changes, breaking("dto_removed", pathRoot+".dto", "DTO classification removed"))
	case oldMessage.DTO != nil && newMessage.DTO != nil && oldMessage.DTO.Kind != newMessage.DTO.Kind:
		changes = append(changes, breaking("dto_kind_changed", pathRoot+".dto", fmt.Sprintf("DTO kind changed from %s to %s", oldMessage.DTO.Kind, newMessage.DTO.Kind)))
	}

	oldByNumber := make(map[int32]Field)
	newByNumber := make(map[int32]Field)
	newByName := make(map[string]Field)
	for _, field := range oldMessage.Fields {
		oldByNumber[field.Number] = field
	}
	for _, field := range newMessage.Fields {
		newByNumber[field.Number] = field
		newByName[field.Name] = field
	}
	for _, oldField := range oldMessage.Fields {
		path := pathRoot + ".field." + oldField.Name
		newField, sameNumber := newByNumber[oldField.Number]
		if !sameNumber {
			if byName, sameName := newByName[oldField.Name]; sameName {
				changes = append(changes, breaking("field_number_changed", path, fmt.Sprintf("field number changed from %d to %d", oldField.Number, byName.Number)))
			} else {
				changes = append(changes, breaking("field_removed", path, fmt.Sprintf("field %s (%d) removed", oldField.Name, oldField.Number)))
			}
			continue
		}
		if oldField.Name != newField.Name {
			changes = append(changes, breaking("field_name_changed", path, fmt.Sprintf("field %d renamed from %s to %s", oldField.Number, oldField.Name, newField.Name)))
		}
		if oldField.JSONName != newField.JSONName {
			changes = append(changes, breaking("field_json_name_changed", path, fmt.Sprintf("JSON name changed from %s to %s", oldField.JSONName, newField.JSONName)))
		}
		if oldField.Kind != newField.Kind || oldField.Type != newField.Type || oldField.Map != newField.Map || oldField.MapKeyType != newField.MapKeyType || oldField.MapValueKind != newField.MapValueKind || oldField.MapValueType != newField.MapValueType {
			changes = append(changes, breaking("field_type_changed", path, fmt.Sprintf("field type changed from %s to %s", describeFieldType(oldField), describeFieldType(newField))))
		}
		if oldField.Repeated != newField.Repeated {
			changes = append(changes, breaking("field_cardinality_changed", path, fmt.Sprintf("repeated changed from %v to %v", oldField.Repeated, newField.Repeated)))
		}
		if oldField.Required != newField.Required {
			changes = append(changes, breaking("field_required_changed", path, fmt.Sprintf("required changed from %v to %v", oldField.Required, newField.Required)))
		}
		if oldField.Optional != newField.Optional {
			changes = append(changes, breaking("field_presence_changed", path, fmt.Sprintf("optional presence changed from %v to %v", oldField.Optional, newField.Optional)))
		}
	}
	for _, newField := range newMessage.Fields {
		if _, ok := oldByNumber[newField.Number]; !ok {
			severity := ChangeInfo
			message := fmt.Sprintf("field %s (%d) added", newField.Name, newField.Number)
			if newField.Required {
				severity = ChangeBreaking
				message = fmt.Sprintf("required field %s (%d) added", newField.Name, newField.Number)
			}
			changes = append(changes, Change{Severity: severity, Kind: "field_added", Path: "message." + newMessage.FullName + ".field." + newField.Name, Message: message})
		}
	}
	return changes
}

func compareEnum(oldEnum, newEnum Enum) []Change {
	var changes []Change
	newByName := make(map[string]EnumValue)
	for _, value := range newEnum.Values {
		newByName[value.Name] = value
	}
	oldByName := make(map[string]EnumValue)
	for _, value := range oldEnum.Values {
		oldByName[value.Name] = value
		path := "enum." + oldEnum.FullName + ".value." + value.Name
		current, ok := newByName[value.Name]
		if !ok {
			changes = append(changes, breaking("enum_value_removed", path, fmt.Sprintf("enum value %s=%d removed", value.Name, value.Number)))
			continue
		}
		if current.Number != value.Number {
			changes = append(changes, breaking("enum_number_changed", path, fmt.Sprintf("enum value number changed from %d to %d", value.Number, current.Number)))
		}
	}
	for _, value := range newEnum.Values {
		if _, ok := oldByName[value.Name]; !ok {
			changes = append(changes, info("enum_value_added", "enum."+newEnum.FullName+".value."+value.Name, fmt.Sprintf("enum value %s=%d added", value.Name, value.Number)))
		}
	}
	return changes
}

func compareService(oldService, newService Service) []Change {
	var changes []Change
	servicePath := "service." + oldService.FullName
	if oldService.Domain != newService.Domain {
		if oldService.Domain == "" {
			changes = append(changes, info("service_domain_declared", servicePath, "typed domain ownership added: "+newService.Domain))
		} else if newService.Domain == "" {
			changes = append(changes, breaking("service_domain_removed", servicePath, "typed domain ownership removed"))
		} else {
			changes = append(changes, breaking("service_domain_changed", servicePath, fmt.Sprintf("domain changed from %s to %s", oldService.Domain, newService.Domain)))
		}
	}
	changes = append(changes, compareApplicationDeclaration(servicePath, oldService.Application, newService.Application)...)

	oldMethods := make(map[string]Method)
	newMethods := make(map[string]Method)
	for _, method := range oldService.Methods {
		oldMethods[method.Name] = method
	}
	for _, method := range newService.Methods {
		newMethods[method.Name] = method
	}
	for name, oldMethod := range oldMethods {
		path := servicePath + ".method." + name
		newMethod, ok := newMethods[name]
		if !ok {
			changes = append(changes, breaking("method_removed", path, "RPC method removed"))
			continue
		}
		if oldMethod.Request != newMethod.Request {
			changes = append(changes, breaking("method_request_changed", path, fmt.Sprintf("request changed from %s to %s", oldMethod.Request, newMethod.Request)))
		}
		if oldMethod.Response != newMethod.Response {
			changes = append(changes, breaking("method_response_changed", path, fmt.Sprintf("response changed from %s to %s", oldMethod.Response, newMethod.Response)))
		}
		if oldMethod.ClientStreaming != newMethod.ClientStreaming || oldMethod.ServerStreaming != newMethod.ServerStreaming {
			changes = append(changes, breaking("method_streaming_changed", path, "streaming mode changed"))
		}
		if directivesKey(oldMethod.Directives) != directivesKey(newMethod.Directives) {
			changes = append(changes, warning("method_directives_changed", path, "yunka method directives changed; review migration metadata"))
		}
		changes = append(changes, compareOperationDeclaration(path, oldMethod.Operation, newMethod.Operation)...)
		if oldMethod.Operation == nil && newMethod.Operation == nil && authorizationKey(oldMethod.Authorization) != authorizationKey(newMethod.Authorization) {
			if oldMethod.Authorization != nil && newMethod.Authorization != nil && oldMethod.Authorization.OperationID != newMethod.Authorization.OperationID {
				changes = append(changes, breaking("operation_id_changed", path, "stable authorization operation id changed"))
			} else {
				changes = append(changes, warning("authorization_policy_changed", path, "legacy authorization policy changed; review permission impact"))
			}
		}
		changes = append(changes, compareHTTPBindings(path, oldMethod.HTTP, newMethod.HTTP)...)
	}
	for name := range newMethods {
		if _, ok := oldMethods[name]; !ok {
			changes = append(changes, info("method_added", "service."+newService.FullName+".method."+name, "RPC method added"))
		}
	}
	return changes
}

func compareApplicationDeclaration(path string, oldValue, newValue *ApplicationDeclaration) []Change {
	switch {
	case oldValue == nil && newValue != nil:
		return []Change{info("application_declared", path+".application", "typed application declaration added: "+newValue.Name)}
	case oldValue != nil && newValue == nil:
		return []Change{breaking("application_removed", path+".application", "typed application declaration removed")}
	case oldValue == nil || newValue == nil:
		return nil
	}
	var changes []Change
	if oldValue.Name != newValue.Name {
		changes = append(changes, breaking("application_identity_changed", path+".application", fmt.Sprintf("application identity changed from %s to %s", oldValue.Name, newValue.Name)))
	}
	oldOperations := make(map[string]OperationDeclaration, len(oldValue.Operations))
	newOperations := make(map[string]OperationDeclaration, len(newValue.Operations))
	for _, operation := range oldValue.Operations {
		oldOperations[operation.ID] = operation
	}
	for _, operation := range newValue.Operations {
		newOperations[operation.ID] = operation
	}
	for id, oldOperation := range oldOperations {
		operationPath := path + ".application.operation." + id
		newOperation, ok := newOperations[id]
		if !ok {
			changes = append(changes, breaking("internal_operation_removed", operationPath, "application-level operation removed"))
			continue
		}
		changes = append(changes, compareOperationDeclaration(operationPath, &oldOperation, &newOperation)...)
	}
	for id := range newOperations {
		if _, ok := oldOperations[id]; !ok {
			changes = append(changes, info("internal_operation_added", path+".application.operation."+id, "application-level operation added"))
		}
	}
	return changes
}

func compareOperationDeclaration(path string, oldValue, newValue *OperationDeclaration) []Change {
	switch {
	case oldValue == nil && newValue != nil:
		return []Change{info("operation_declared", path+".operation", "typed operation declaration added: "+newValue.ID)}
	case oldValue != nil && newValue == nil:
		return []Change{breaking("operation_removed", path+".operation", "typed operation declaration removed")}
	case oldValue == nil || newValue == nil:
		return nil
	}
	var changes []Change
	operationPath := path + ".operation"
	if oldValue.ID != newValue.ID {
		changes = append(changes, breaking("operation_id_changed", operationPath, fmt.Sprintf("operation id changed from %s to %s", oldValue.ID, newValue.ID)))
	}
	if oldValue.UseCase != newValue.UseCase {
		changes = append(changes, breaking("use_case_changed", operationPath, fmt.Sprintf("use case changed from %s to %s", oldValue.UseCase, newValue.UseCase)))
	}
	if oldValue.PermissionMode != newValue.PermissionMode {
		changes = append(changes, breaking("permission_mode_changed", operationPath, fmt.Sprintf("permission mode changed from %s to %s", oldValue.PermissionMode, newValue.PermissionMode)))
	}
	if strings.Join(stableStrings(oldValue.Permissions), ",") != strings.Join(stableStrings(newValue.Permissions), ",") {
		changes = append(changes, breaking("permissions_changed", operationPath, "permission requirements changed"))
	}
	if strings.Join(stableStrings(oldValue.Authentication), ",") != strings.Join(stableStrings(newValue.Authentication), ",") {
		changes = append(changes, breaking("authentication_changed", operationPath, "authentication requirements changed"))
	}
	if oldValue.TenantRequired != newValue.TenantRequired {
		changes = append(changes, breaking("tenant_requirement_changed", operationPath, fmt.Sprintf("tenant requirement changed from %v to %v", oldValue.TenantRequired, newValue.TenantRequired)))
	}
	if oldValue.Public != newValue.Public {
		changes = append(changes, breaking("public_policy_changed", operationPath, fmt.Sprintf("public changed from %v to %v", oldValue.Public, newValue.Public)))
	}
	if oldValue.RequestType != newValue.RequestType {
		changes = append(changes, breaking("operation_request_changed", operationPath, fmt.Sprintf("request type changed from %s to %s", oldValue.RequestType, newValue.RequestType)))
	}
	if oldValue.ResponseType != newValue.ResponseType {
		changes = append(changes, breaking("operation_response_changed", operationPath, fmt.Sprintf("response type changed from %s to %s", oldValue.ResponseType, newValue.ResponseType)))
	}
	if oldValue.ApplicationMethod != newValue.ApplicationMethod {
		changes = append(changes, breaking("application_method_changed", operationPath, fmt.Sprintf("application method changed from %s to %s", oldValue.ApplicationMethod, newValue.ApplicationMethod)))
	}
	return changes
}

func compareHTTPBindings(path string, oldBindings, newBindings []HTTPBinding) []Change {
	var changes []Change
	newSet := make(map[string]struct{}, len(newBindings))
	oldSet := make(map[string]struct{}, len(oldBindings))
	for _, binding := range newBindings {
		newSet[bindingKey(binding)] = struct{}{}
	}
	for _, binding := range oldBindings {
		key := bindingKey(binding)
		oldSet[key] = struct{}{}
		if _, ok := newSet[key]; !ok {
			changes = append(changes, breaking("http_binding_removed", path, "HTTP binding removed or changed: "+key))
		}
	}
	for _, binding := range newBindings {
		key := bindingKey(binding)
		if _, ok := oldSet[key]; !ok {
			changes = append(changes, info("http_binding_added", path, "HTTP binding added: "+key))
		}
	}
	return changes
}

func bindingKey(binding HTTPBinding) string {
	return strings.Join([]string{strings.ToUpper(binding.Method), binding.Path, binding.Body, binding.ResponseBody}, "|")
}

func describeFieldType(field Field) string {
	if field.Map {
		return fmt.Sprintf("map<%s,%s:%s>", field.MapKeyType, field.MapValueKind, field.MapValueType)
	}
	return field.Kind + ":" + field.Type
}

func serviceIndex(manifest Manifest) map[string]Service {
	result := make(map[string]Service, len(manifest.Services))
	for _, item := range manifest.Services {
		result[item.FullName] = item
	}
	return result
}

func breaking(kind, path, message string) Change {
	return Change{Severity: ChangeBreaking, Kind: kind, Path: path, Message: message}
}

func info(kind, path, message string) Change {
	return Change{Severity: ChangeInfo, Kind: kind, Path: path, Message: message}
}

func warning(kind, path, message string) Change {
	return Change{Severity: ChangeWarning, Kind: kind, Path: path, Message: message}
}

func directivesKey(directives map[string]string) string {
	if len(directives) == 0 {
		return ""
	}
	keys := make([]string, 0, len(directives))
	for key := range directives {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(directives[key])
		builder.WriteByte('\n')
	}
	return builder.String()
}

func changeSeverityRank(severity ChangeSeverity) int {
	switch severity {
	case ChangeBreaking:
		return 0
	case ChangeWarning:
		return 1
	default:
		return 2
	}
}

func authorizationKey(policy *AuthorizationPolicy) string {
	if policy == nil {
		return ""
	}
	copy := *policy
	copy.Permissions = append([]string(nil), policy.Permissions...)
	copy.Authentication = append([]string(nil), policy.Authentication...)
	sort.Strings(copy.Permissions)
	sort.Strings(copy.Authentication)
	return strings.Join([]string{copy.OperationID, copy.PermissionMode, fmt.Sprint(copy.TenantRequired), strings.Join(copy.Permissions, ","), strings.Join(copy.Authentication, ",")}, "|")
}
