package add

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"yunka.io/app/cmd/projectflow"
)

func AddOperation(options OperationOptions) (Report, error) {
	return changeOperation(options, true)
}

func PlanOperation(options OperationOptions) (Report, error) {
	return changeOperation(options, false)
}

func changeOperation(options OperationOptions, apply bool) (Report, error) {
	domain, application, err := parseApplicationKey(options.ApplicationKey)
	if err != nil {
		return Report{}, requestFailure(err)
	}
	if err := validateOperationOptions(&options); err != nil {
		return Report{}, requestFailure(err)
	}
	inputs, err := projectflow.DescribeOwnershipInputs(projectflow.Options{Root: options.Root})
	if err != nil {
		return Report{}, sourceFailure("", fmt.Errorf("add operation: resolve project facts: %w", err))
	}
	sources, err := loadSources(inputs)
	if err != nil {
		return Report{}, err
	}
	source, service, err := selectApplicationSource(sources, domain, application, options.Source)
	if err != nil {
		return Report{}, err
	}
	for _, candidate := range sources {
		data, readErr := os.ReadFile(candidate.Absolute)
		if readErr != nil {
			return Report{}, sourceFailure(candidate.Relative, readErr)
		}
		if operationIDExists(string(data), options.OperationID) {
			return Report{}, conflictFailure(candidate.Relative, fmt.Errorf("add operation: operation ID %s already exists", options.OperationID))
		}
	}
	contents, err := os.ReadFile(source.Absolute)
	if err != nil {
		return Report{}, sourceFailure(source.Relative, err)
	}
	text := string(contents)
	rpcName := strings.TrimSpace(options.RPCName)
	if rpcName == "" {
		rpcName = exportedIdentifier(lastKeyPart(options.OperationID))
	}
	if !goIdentifierPattern.MatchString(rpcName) || !unicode.IsUpper([]rune(rpcName)[0]) {
		return Report{}, requestFailure(fmt.Errorf("add operation: rpc name %q must be an exported protobuf identifier", rpcName))
	}
	requestType := strings.TrimSpace(options.RequestType)
	if requestType == "" {
		requestType = rpcName + "Request"
	}
	responseType := strings.TrimSpace(options.ResponseType)
	if responseType == "" {
		responseType = rpcName + "Response"
	}
	for _, name := range []string{requestType, responseType} {
		if !goIdentifierPattern.MatchString(name) || !unicode.IsUpper([]rune(name)[0]) {
			return Report{}, requestFailure(fmt.Errorf("add operation: DTO message name %q must be an exported protobuf identifier", name))
		}
	}
	currentService, ok := findService(text, service.Name)
	if !ok {
		return Report{}, sourceFailure(source.Relative, fmt.Errorf("add operation: service %s disappeared from selected source", service.Name))
	}
	if rpcExists(currentService.Body, rpcName) {
		return Report{}, conflictFailure(source.Relative, fmt.Errorf("add operation: RPC %s already exists in service %s", rpcName, service.Name))
	}
	packageName := protoPackage(text)
	if packageName == "" {
		return Report{}, sourceFailure(source.Relative, errors.New("add operation: selected proto source has no package declaration"))
	}
	if protoGoPackage(text) == "" {
		return Report{}, sourceFailure(source.Relative, errors.New("add operation: typed Operation scaffold requires an explicit option go_package"))
	}
	requestExists, requestKind := dtoMessageKind(text, requestType)
	responseExists, responseKind := dtoMessageKind(text, responseType)
	if requestExists && requestKind != "input" {
		return Report{}, conflictFailure(source.Relative, fmt.Errorf("add operation: existing request message %s is not DTO_INPUT", requestType))
	}
	if responseExists && responseKind != "output" {
		return Report{}, conflictFailure(source.Relative, fmt.Errorf("add operation: existing response message %s is not DTO_OUTPUT", responseType))
	}
	if !requestExists {
		if paths, err := messageSourcesInPackage(sources, packageName, requestType, source.Relative); err != nil {
			return Report{}, err
		} else if len(paths) > 0 {
			return Report{}, conflictFailure("", fmt.Errorf("add operation: request message %s already exists in protobuf package %s outside the selected source (%s); AX5 will not infer cross-file imports or DTO reuse", requestType, packageName, strings.Join(paths, ", ")))
		}
	}
	if !responseExists {
		if paths, err := messageSourcesInPackage(sources, packageName, responseType, source.Relative); err != nil {
			return Report{}, err
		} else if len(paths) > 0 {
			return Report{}, conflictFailure("", fmt.Errorf("add operation: response message %s already exists in protobuf package %s outside the selected source (%s); AX5 will not infer cross-file imports or DTO reuse", responseType, packageName, strings.Join(paths, ", ")))
		}
	}

	method := renderRPCOperation(rpcName, requestType, responseType, options)
	updated, err := insertServiceMember(text, service.Name, method)
	if err != nil {
		return Report{}, sourceFailure(source.Relative, err)
	}
	if !requestExists {
		updated = appendProtoBlock(updated, renderDTOMessage(requestType, "DTO_INPUT", "request fields"))
	}
	if !responseExists {
		updated = appendProtoBlock(updated, renderDTOMessage(responseType, "DTO_OUTPUT", "response fields"))
	}
	owner, err := requireEditable(inputs.Project.Root, source.Relative)
	if err != nil {
		return Report{}, err
	}
	implementationRelative := filepath.ToSlash(filepath.Join(inputs.Project.GeneratedGoRoot, domain, "application", operationFileStem(options.OperationID)+".go"))
	implementationOwner, err := requireEditable(inputs.Project.Root, implementationRelative)
	if err != nil {
		return Report{}, err
	}
	implementationAbsolute := projectflow.ResolveDescriptorPath(inputs.Project, implementationRelative)
	if _, statErr := os.Stat(implementationAbsolute); statErr == nil {
		return Report{}, conflictFailure(implementationRelative, fmt.Errorf("add operation: developer implementation landing file already exists; refusing to overwrite"))
	} else if !os.IsNotExist(statErr) {
		return Report{}, sourceFailure(implementationRelative, statErr)
	}
	implementationContents := []byte(renderImplementationLanding(domain, application, options.OperationID, rpcName))

	report := Report{
		SchemaVersion: SchemaVersion,
		Kind:          "operation",
		Identity: map[string]string{
			"domain": domain, "application": application, "operationId": options.OperationID,
			"useCase": options.UseCase, "service": service.Name, "rpc": rpcName,
			"requestType": packageName + "." + requestType, "responseType": packageName + "." + responseType,
		},
		Mutations: []Mutation{
			{Path: source.Relative, Action: "modified", Owner: owner},
			{Path: implementationRelative, Action: "created", Owner: implementationOwner},
		},
		Effects: contractEffects(inputs.Project, true, true),
		NextActions: []NextAction{
			{Command: "yunka generate", Purpose: "derive protobuf Go, OperationPlan, policy/transport adapters, and assembly facts"},
			{Command: "yunka change plan --operation " + shellQuote(options.OperationID) + " --intent implementation --format json", Purpose: "resolve the newly canonical Operation implementation boundary after generation"},
			{Command: "yunka check --format agent-json", Purpose: "validate explicit Operation semantics and generated drift"},
			{Command: "go test ./...", Purpose: "verify the developer implementation and affected packages"},
			{Command: "yunka dev", Purpose: "verify runtime readiness and behavior"},
		},
		Notes: []string{
			"Request/response messages are empty structural DTOs; add business fields explicitly.",
			"Access, tenant, transaction, idempotency, composition, permissions, authentication, dependencies, and HTTP facts come only from the caller flags.",
			"The Go landing file contains only package declaration and TODO guidance; no receiver, function body, persistence, Saga, Outbox, event publication, external effect, or business implementation was generated.",
		},
	}
	if !apply {
		report.Kind = "operation-plan"
		report.NextActions = []NextAction{}
		report.Notes = append([]string{"Plan only: no project files were written; rerun the same explicit add operation request without --plan to apply after review."}, report.Notes...)
		normalizeReport(&report)
		return report, nil
	}

	if err := writeNewFile(implementationAbsolute, implementationContents); err != nil {
		if os.IsExist(err) {
			return Report{}, conflictFailure(implementationRelative, err)
		}
		return Report{}, sourceFailure(implementationRelative, err)
	}
	if err := writeAtomic(source.Absolute, []byte(updated)); err != nil {
		rollbackErr := os.Remove(implementationAbsolute)
		if rollbackErr != nil && !os.IsNotExist(rollbackErr) {
			return Report{}, sourceFailure(source.Relative, fmt.Errorf("%w; rollback of %s also failed: %v", err, implementationRelative, rollbackErr))
		}
		return Report{}, sourceFailure(source.Relative, err)
	}

	normalizeReport(&report)
	return report, nil
}
