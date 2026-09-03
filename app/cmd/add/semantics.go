package add

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/ownership"
	"yunka.io/app/cmd/projectflow"
)

func validateOperationOptions(options *OperationOptions) error {
	if options == nil {
		return errors.New("add operation: options are required")
	}
	options.OperationID = strings.TrimSpace(options.OperationID)
	options.UseCase = strings.TrimSpace(options.UseCase)
	options.Access = strings.ToLower(strings.TrimSpace(options.Access))
	options.PermissionMode = strings.ToLower(strings.TrimSpace(options.PermissionMode))
	options.Tenant = strings.ToLower(strings.TrimSpace(options.Tenant))
	options.Transaction = normalizeChoice(options.Transaction)
	options.Idempotency = normalizeChoice(options.Idempotency)
	options.Composition = normalizeChoice(options.Composition)
	options.Permissions = normalizedKeys(options.Permissions)
	options.Authentication = normalizedChoices(options.Authentication)
	options.RequiresOperations = normalizedKeys(options.RequiresOperations)
	options.HTTPMethod = strings.ToUpper(strings.TrimSpace(options.HTTPMethod))
	options.HTTPPath = strings.TrimSpace(options.HTTPPath)
	options.HTTPBody = strings.TrimSpace(options.HTTPBody)

	if !validPolicyKey(options.OperationID) {
		return fmt.Errorf("add operation: operation ID %q must be an explicit stable lowercase key", options.OperationID)
	}
	if !validPolicyKey(options.UseCase) {
		return fmt.Errorf("add operation: --use-case is required and must be a stable lowercase key")
	}
	if options.Access != "public" && options.Access != "protected" {
		return fmt.Errorf("add operation: --access is required and must be public or protected")
	}
	if options.Tenant != "required" && options.Tenant != "optional" {
		return fmt.Errorf("add operation: --tenant is required and must be required or optional")
	}
	if !oneOf(options.Transaction, "none", "read_only", "local") {
		return fmt.Errorf("add operation: --transaction is required and must be none, read-only, or local")
	}
	if !oneOf(options.Idempotency, "none", "required") {
		return fmt.Errorf("add operation: --idempotency is required and must be none or required")
	}
	if !oneOf(options.Composition, "none", "local", "remote_saga") {
		return fmt.Errorf("add operation: --composition is required and must be none, local, or remote-saga")
	}
	if options.Idempotency == "required" && options.Transaction != "local" {
		return fmt.Errorf("add operation: durable idempotency requires --transaction local")
	}
	if options.Access == "public" {
		if options.Tenant != "optional" || len(options.Permissions) > 0 || len(options.Authentication) > 0 || options.PermissionMode != "" {
			return fmt.Errorf("add operation: public access requires tenant=optional and cannot declare permissions, permission-mode, or authentication")
		}
	} else {
		if len(options.Permissions) == 0 {
			return fmt.Errorf("add operation: protected access requires at least one --permission")
		}
		if options.PermissionMode != "all" && options.PermissionMode != "any" {
			return fmt.Errorf("add operation: protected access requires --permission-mode all or any")
		}
	}
	for _, value := range options.Permissions {
		if !validPolicyKey(value) {
			return fmt.Errorf("add operation: permission %q must be a stable lowercase key", value)
		}
	}
	for _, value := range options.RequiresOperations {
		if !validPolicyKey(value) {
			return fmt.Errorf("add operation: required operation %q must be a stable lowercase key", value)
		}
		if value == options.OperationID {
			return fmt.Errorf("add operation: operation %s cannot require itself", options.OperationID)
		}
	}
	for _, value := range options.Authentication {
		if !oneOf(value, "jwt", "api_key", "service") {
			return fmt.Errorf("add operation: unsupported authentication %q; use jwt, api-key, or service", value)
		}
	}
	if options.HTTPBody != "" && options.HTTPMethod == "" {
		return fmt.Errorf("add operation: --http-body requires --http-method and --http-path")
	}
	if (options.HTTPMethod == "") != (options.HTTPPath == "") {
		return fmt.Errorf("add operation: --http-method and --http-path must be supplied together")
	}
	if options.HTTPPath != "" && !strings.HasPrefix(options.HTTPPath, "/") {
		return fmt.Errorf("add operation: --http-path must start with /")
	}
	if options.HTTPMethod != "" && !oneOf(options.HTTPMethod, "GET", "POST", "PUT", "PATCH", "DELETE") {
		return fmt.Errorf("add operation: unsupported HTTP method %s", options.HTTPMethod)
	}
	return nil
}

func renderRPCOperation(rpcName, requestType, responseType string, options OperationOptions) string {
	var b strings.Builder
	if options.HTTPMethod != "" {
		fmt.Fprintf(&b, "  // @yunka.http %s %s", options.HTTPMethod, options.HTTPPath)
		if options.HTTPBody != "" {
			fmt.Fprintf(&b, " body=%s", options.HTTPBody)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "  rpc %s(%s) returns (%s) {\n", rpcName, requestType, responseType)
	b.WriteString("    option (yunka.dsl.v1.operation) = {\n")
	fmt.Fprintf(&b, "      id: %q\n", options.OperationID)
	fmt.Fprintf(&b, "      use_case: %q\n", options.UseCase)
	fmt.Fprintf(&b, "      public: %t\n", options.Access == "public")
	fmt.Fprintf(&b, "      tenant_required: %t\n", options.Tenant == "required")
	if options.Access == "protected" {
		for _, permission := range options.Permissions {
			fmt.Fprintf(&b, "      permissions: %q\n", permission)
		}
		fmt.Fprintf(&b, "      permission_mode: %s\n", permissionModeEnum(options.PermissionMode))
		for _, authentication := range options.Authentication {
			fmt.Fprintf(&b, "      authentication: %s\n", authenticationEnum(authentication))
		}
	}
	for _, dependency := range options.RequiresOperations {
		fmt.Fprintf(&b, "      requires_operations: %q\n", dependency)
	}
	if options.Composition != "none" {
		fmt.Fprintf(&b, "      composition: %s\n", compositionEnum(options.Composition))
	}
	fmt.Fprintf(&b, "      execution: { transaction: %s idempotency: %s }\n", transactionEnum(options.Transaction), idempotencyEnum(options.Idempotency))
	b.WriteString("    };\n")
	b.WriteString("  }\n")
	return b.String()
}

func renderDTOMessage(name, kind, todo string) string {
	return fmt.Sprintf("\n// AX5 structural scaffold. Developer-owned DTO; add business fields explicitly.\nmessage %s {\n  option (yunka.dsl.v1.dto) = { kind: %s };\n  // TODO(agent): add %s.\n}\n", name, kind, todo)
}

func contractEffects(project projectflow.ProjectDescriptor, operation, application bool) []Effect {
	root := strings.TrimSuffix(project.ContractGenerated, "/")
	result := []Effect{{Stage: "contract", Path: filepath.ToSlash(filepath.Join(root, contract.ManifestFilename)), Reason: "canonical manifest is derived from protobuf source"}}
	if operation {
		result = append(result,
			Effect{Stage: "operation-plan", Path: filepath.ToSlash(filepath.Join(root, contract.OperationPlansFilename)), Reason: "OperationPlan is derived from explicit typed Operation declarations"},
			Effect{Stage: "openapi", Path: filepath.ToSlash(filepath.Join(root, contract.OpenAPIFilename)), Conditional: true, Reason: "OpenAPI may change when an explicit HTTP binding is added"},
			Effect{Stage: "typescript", Path: filepath.ToSlash(filepath.Join(root, contract.TypeScriptFilename)), Conditional: true, Reason: "typed client projection may change when Operation/DTO facts change"},
		)
	}
	result = append(result, Effect{Stage: "protobuf-go", Scope: project.ProtobufGoManifest, Conditional: true, Reason: "the protobuf Go ownership manifest tracks derived Go outputs that may change after a contract source mutation"})
	if operation || application {
		result = append(result,
			Effect{Stage: "application-codegen", Scope: project.GeneratedGoRoot, Conditional: true, Reason: "typed application ports/adapters may change"},
			Effect{Stage: "assembly", Path: filepath.ToSlash(filepath.Join(root, contract.AssemblyPlanFilename)), Conditional: true, Reason: "application/operation structure may affect assembly"},
		)
	}
	return result
}

func contractNextActions(operationID string) []NextAction {
	result := []NextAction{{Command: "yunka generate", Purpose: "derive canonical generated artifacts"}, {Command: "yunka check --format agent-json", Purpose: "validate structure and generated drift"}}
	if strings.TrimSpace(operationID) != "" {
		result = append(result, NextAction{Command: "yunka change plan --operation " + shellQuote(operationID) + " --format json", Purpose: "inspect impact after the new Operation becomes canonical"})
	}
	return result
}

func parseApplicationKey(value string) (string, string, error) {
	value = strings.TrimSpace(value)
	parts := strings.Split(value, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("application capability %q must use exact <domain>/<application> stable keys", value)
	}
	domain := strings.TrimSpace(parts[0])
	application := strings.TrimSpace(parts[1])
	if !validPolicyKey(domain) || !validPolicyKey(application) {
		return "", "", fmt.Errorf("application capability %q must use exact <domain>/<application> stable keys", value)
	}
	return domain, application, nil
}

func requireEditable(root, path string) (string, error) {
	report, err := ownership.Build(root, []string{path})
	if err != nil {
		return "", ownershipFailure(path, err)
	}
	if len(report.Decisions) != 1 || !report.Decisions[0].SafeAutoEdit {
		reason := "ownership decision missing"
		if len(report.Decisions) == 1 {
			reason = report.Decisions[0].Reason
		}
		return "", ownershipFailure(path, fmt.Errorf("AX2 does not prove this structural target safe for automatic editing: %s", reason))
	}
	return report.Decisions[0].Owner, nil
}
