package contract

import (
	"fmt"
	"go/format"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"yunka.io/pkg/operationplan"
)

// RenderC9ApplicationCode emits the canonical C9 application boundary.
// C8 Application Ports, capability ports, and static policy metadata remain
// source-compatible, but the legacy C8 REST/RPC transport files are filtered
// out so generated runtime traffic has one canonical Executor path.
func RenderC9ApplicationCode(manifest Manifest, options ApplicationCodeOptions) ([]GeneratedApplicationFile, error) {
	base, err := RenderApplicationCode(manifest, options)
	if err != nil {
		return nil, err
	}
	base = c9NonTransportCompatibilityFiles(base)
	manifest.Normalize()
	plans, err := CompileOperationPlans(manifest)
	if err != nil {
		return nil, err
	}
	planByID := make(map[string]operationplan.Plan, len(plans.Operations))
	for _, plan := range plans.Operations {
		planByID[plan.OperationID] = plan
	}
	packages, err := buildProtoGoPackages(manifest)
	if err != nil {
		return nil, err
	}
	messages := messageIndex(manifest)
	typedByDomain := make(map[string][]Service)
	serviceByApplication := make(map[string]Service)
	for _, service := range manifest.Services {
		if service.Application != nil {
			typedByDomain[service.Domain] = append(typedByDomain[service.Domain], service)
			serviceByApplication[service.Domain+"/"+service.Application.Name] = service
		}
	}
	rootImport := strings.TrimRight(strings.TrimSpace(options.RootImport), "/")
	if rootImport == "" && len(planByID) > 0 {
		return nil, fmt.Errorf("contract C9 application codegen: root import is required")
	}
	files := append([]GeneratedApplicationFile(nil), base...)
	domains := make([]string, 0, len(typedByDomain))
	for domain := range typedByDomain {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		services := append([]Service(nil), typedByDomain[domain]...)
		sort.Slice(services, func(i, j int) bool { return services[i].FullName < services[j].FullName })
		multi := len(services) > 1
		for _, service := range services {
			naming := namingForService(service, multi)
			generated := []struct {
				path   string
				render func() (string, error)
			}{
				{filepath.ToSlash(filepath.Join(domain, "policy", "zz_yunka_"+naming.FileStem+"_operation_plan_gen.go")), func() (string, error) {
					return renderC9OperationPlans(service, naming, planByID)
				}},
			}
			if len(service.Methods) > 0 {
				generated = append(generated,
					struct {
						path   string
						render func() (string, error)
					}{filepath.ToSlash(filepath.Join(domain, "transport", "rpc", "zz_yunka_"+naming.FileStem+"_operation_executor_gen.go")), func() (string, error) {
						return renderC9RPCAdapter(service, packages, rootImport, naming)
					}},
					struct {
						path   string
						render func() (string, error)
					}{filepath.ToSlash(filepath.Join(domain, "transport", "rest", "zz_yunka_"+naming.FileStem+"_operation_executor_gen.go")), func() (string, error) {
						return renderC9RESTAdapter(service, packages, messages, rootImport, naming)
					}},
				)
			}
			if len(service.Application.Requires) > 0 {
				generated = append(generated, struct {
					path   string
					render func() (string, error)
				}{
					filepath.ToSlash(filepath.Join(domain, "application", "zz_yunka_"+naming.FileStem+"_capability_ports_gen.go")),
					func() (string, error) {
						return renderC9CapabilityPorts(service, naming, serviceByApplication, typedByDomain, packages, rootImport)
					},
				})
			}
			for _, artifact := range generated {
				source, err := artifact.render()
				if err != nil {
					return nil, err
				}
				formatted, err := format.Source([]byte(source))
				if err != nil {
					return nil, fmt.Errorf("contract C9 application codegen: format %s: %w\n%s", artifact.path, err, source)
				}
				files = append(files, GeneratedApplicationFile{Path: artifact.path, Content: formatted})
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

func c9NonTransportCompatibilityFiles(files []GeneratedApplicationFile) []GeneratedApplicationFile {
	result := make([]GeneratedApplicationFile, 0, len(files))
	for _, file := range files {
		path := filepath.ToSlash(file.Path)
		if strings.Contains(path, "/transport/rest/") || strings.Contains(path, "/transport/rpc/") || strings.HasSuffix(path, "_capability_ports_gen.go") {
			continue
		}
		result = append(result, file)
	}
	return result
}

func c9PlanFunctionName(naming serviceCodegenNaming, methodName string) string {
	if naming.Multi {
		return "OperationPlan" + naming.OperationPrefix + methodName
	}
	return "OperationPlan" + methodName
}

func c9PlanFunction(naming serviceCodegenNaming, method Method) string {
	return c9PlanFunctionName(naming, method.Name)
}

func c9RegisterName(naming serviceCodegenNaming) string {
	if naming.Multi {
		return "Register" + naming.Symbol + "OperationExecutor"
	}
	return "RegisterOperationExecutor"
}

func c9ServerName(naming serviceCodegenNaming) string {
	if naming.Multi {
		return naming.Symbol + "OperationServer"
	}
	return "OperationServer"
}

func c9RESTHandlerName(naming serviceCodegenNaming) string {
	if naming.Multi {
		return naming.Symbol + "OperationHandler"
	}
	return "OperationHandler"
}

func c9RESTErrorName(naming serviceCodegenNaming) string {
	if naming.Multi {
		return "write" + naming.Symbol + "OperationError"
	}
	return "writeOperationError"
}

func c9CapabilityEdgeSymbol(source Service, dependency string) string {
	sourceSymbol := ""
	if source.Application != nil {
		sourceSymbol = exportedApplicationSymbol(source.Application.Name)
	}
	if sourceSymbol == "" {
		sourceSymbol = exportedApplicationSymbol(source.Name)
	}
	targetSymbol := exportedApplicationSymbol(strings.ReplaceAll(dependency, "/", "_"))
	return sourceSymbol + "To" + targetSymbol
}

func c9RequiredCapabilityOperations(source Service, target Service) ([]applicationOperationBinding, error) {
	targetOperations, err := serviceApplicationOperations(target)
	if err != nil {
		return nil, err
	}
	targetByID := make(map[string]applicationOperationBinding, len(targetOperations))
	for _, binding := range targetOperations {
		targetByID[binding.Operation.ID] = binding
	}

	sourceOperations, err := serviceApplicationOperations(source)
	if err != nil {
		return nil, err
	}
	required := map[string]struct{}{}
	for _, binding := range sourceOperations {
		for _, operationID := range binding.Operation.RequiresOperations {
			if _, ok := targetByID[operationID]; ok {
				required[operationID] = struct{}{}
			}
		}
	}

	result := make([]applicationOperationBinding, 0, len(required))
	for _, binding := range targetOperations {
		if _, ok := required[binding.Operation.ID]; ok {
			result = append(result, binding)
		}
	}
	return result, nil
}

func renderC9CapabilityPorts(service Service, naming serviceCodegenNaming, services map[string]Service, typedByDomain map[string][]Service, packages []protoGoPackage, rootImport string) (string, error) {
	imports := newImportSet()
	imports.add("errors", "errors")
	imports.add("yunka.io/framework/operation", "operation")
	var declarations strings.Builder
	var providerMethods strings.Builder
	seen := map[string]string{}
	for _, dependency := range stableStrings(service.Application.Requires) {
		target, ok := services[dependency]
		if !ok || target.Application == nil {
			return "", fmt.Errorf("contract C9 application codegen: unknown capability dependency %s for %s", dependency, service.FullName)
		}
		targetNaming := namingForService(target, len(typedByDomain[target.Domain]) > 1)
		dependencySymbol := exportedApplicationSymbol(strings.ReplaceAll(dependency, "/", "_"))
		if owner, duplicate := seen[dependencySymbol]; duplicate && owner != dependency {
			return "", fmt.Errorf("contract C9 application codegen: capability dependencies %s and %s collapse to symbol %s", owner, dependency, dependencySymbol)
		}
		seen[dependencySymbol] = dependency
		edgeSymbol := c9CapabilityEdgeSymbol(service, dependency)
		if edgeSymbol == "" {
			return "", fmt.Errorf("contract C9 application codegen: capability edge %s -> %s has no generated symbol", service.Domain+"/"+service.Application.Name, dependency)
		}
		interfaceName := edgeSymbol + "ChildCapability"
		implementationName := "c9" + edgeSymbol + "ChildCapability"
		constructorName := "New" + edgeSymbol + "ChildCapability"

		targetApplicationType := targetNaming.ApplicationInterface
		if target.Domain != service.Domain {
			alias := imports.add(rootImport+"/"+target.Domain+"/application", safeFileName(target.Domain)+"application")
			targetApplicationType = alias + "." + targetNaming.ApplicationInterface
		}

		operations, err := c9RequiredCapabilityOperations(service, target)
		if err != nil {
			return "", err
		}
		var policyAlias string
		if len(operations) > 0 {
			imports.add("context", "context")
			policyAlias = imports.add(rootImport+"/"+target.Domain+"/policy", safeFileName(target.Domain)+"policy")
		}
		var interfaceMethods strings.Builder
		var wrapperMethods strings.Builder
		for _, operationBinding := range operations {
			request, err := resolveGoType(operationBinding.RequestType, packages, imports)
			if err != nil {
				return "", err
			}
			response, err := resolveGoType(operationBinding.ResponseType, packages, imports)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&interfaceMethods, "\t%s(context.Context, *%s.%s) (*%s.%s, error)\n", operationBinding.MethodName, request.Alias, request.Type, response.Alias, response.Type)
			fmt.Fprintf(&wrapperMethods, "func (capability *%s) %s(ctx context.Context, request *%s.%s) (*%s.%s, error) {\n", implementationName, operationBinding.MethodName, request.Alias, request.Type, response.Alias, response.Type)
			fmt.Fprintf(&wrapperMethods, "\treturn operation.ExecuteChildTyped(ctx, capability.executor, %s.%s(), request, capability.application.%s)\n}\n\n", policyAlias, c9PlanFunctionName(targetNaming, operationBinding.MethodName), operationBinding.MethodName)
		}
		fmt.Fprintf(&declarations, "type %s interface {\n%s}\n\n", interfaceName, interfaceMethods.String())
		fmt.Fprintf(&declarations, "type %s struct { application %s; executor operation.Executor }\n\n", implementationName, targetApplicationType)
		fmt.Fprintf(&declarations, "func %s(application %s, executor operation.Executor) (%s, error) {\n", constructorName, targetApplicationType, interfaceName)
		fmt.Fprintf(&declarations, "\tif application == nil { return nil, errors.New(%q) }\n", "contract C9 child capability: target application is required")
		fmt.Fprintf(&declarations, "\tif executor == nil { return nil, errors.New(%q) }\n", "contract C9 child capability: operation executor is required")
		fmt.Fprintf(&declarations, "\treturn &%s{application: application, executor: executor}, nil\n}\n\n", implementationName)
		declarations.WriteString(wrapperMethods.String())
		fmt.Fprintf(&providerMethods, "\t%s() %s\n", dependencySymbol, interfaceName)
	}
	var b strings.Builder
	b.WriteString(GeneratedApplicationMarker + "\n\npackage application\n\nimport (\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	b.WriteString(declarations.String())
	fmt.Fprintf(&b, "// %sCapabilities exposes edge-owned C9 child-Operation wrappers for declared operation dependencies.\n", naming.Symbol)
	fmt.Fprintf(&b, "type %sCapabilities interface {\n%s}\n", naming.Symbol, providerMethods.String())
	return b.String(), nil
}

func renderC9OperationPlans(service Service, naming serviceCodegenNaming, plans map[string]operationplan.Plan) (string, error) {
	var b strings.Builder
	b.WriteString(GeneratedApplicationMarker + "\n\npackage policy\n\nimport \"yunka.io/pkg/operationplan\"\n\n")
	operations, err := serviceApplicationOperations(service)
	if err != nil {
		return "", err
	}
	for _, operationBinding := range operations {
		plan, ok := plans[operationBinding.Operation.ID]
		if !ok {
			return "", fmt.Errorf("contract C9 application codegen: no compiled plan for %s", operationBinding.Operation.ID)
		}
		fmt.Fprintf(&b, "func %s() operationplan.Plan {\n\treturn ", c9PlanFunctionName(naming, operationBinding.MethodName))
		writeOperationPlanLiteral(&b, plan)
		b.WriteString("\n}\n\n")
	}
	return b.String(), nil
}

func writeOperationPlanLiteral(b *strings.Builder, plan operationplan.Plan) {
	fmt.Fprintf(b, "operationplan.Plan{OperationID:%q, Domain:%q, Application:%q, UseCase:%q, RequestType:%q, ResponseType:%q, ", plan.OperationID, plan.Domain, plan.Application, plan.UseCase, plan.RequestType, plan.ResponseType)
	fmt.Fprintf(b, "Execution: operationplan.Execution{Transaction:%q, Idempotency:%q}, ", plan.Execution.Transaction, plan.Execution.Idempotency)
	fmt.Fprintf(b, "Security: operationplan.Security{Public:%t, TenantRequired:%t, Authentication:", plan.Security.Public, plan.Security.TenantRequired)
	writeOperationPlanStrings(b, plan.Security.Authentication)
	b.WriteString(", Permissions:")
	writeOperationPlanStrings(b, plan.Security.Permissions)
	fmt.Fprintf(b, ", PermissionMode:%q}, ", plan.Security.PermissionMode)
	fmt.Fprintf(b, "Composition: operationplan.Composition{Boundary:%q, RequiresOperations:", plan.Composition.Boundary)
	writeOperationPlanStrings(b, plan.Composition.RequiresOperations)
	b.WriteString(", PermissionClosure:")
	writeOperationPlanStrings(b, plan.Composition.PermissionClosure)
	b.WriteString("}, ApplicationRequires:")
	writeOperationPlanStrings(b, plan.ApplicationRequires)
	fmt.Fprintf(b, ", Bindings: operationplan.Bindings{RPC:%q, HTTP:[]operationplan.HTTPBinding{", plan.Bindings.RPC)
	for _, binding := range plan.Bindings.HTTP {
		fmt.Fprintf(b, "{Method:%q, Path:%q, Body:%q, ResponseBody:%q},", binding.Method, binding.Path, binding.Body, binding.ResponseBody)
	}
	b.WriteString("}}}")
}

func writeOperationPlanStrings(b *strings.Builder, values []string) {
	b.WriteString("[]string{")
	for _, value := range values {
		fmt.Fprintf(b, "%q,", value)
	}
	b.WriteString("}")
}

func renderC9RPCAdapter(service Service, packages []protoGoPackage, rootImport string, naming serviceCodegenNaming) (string, error) {
	imports := newImportSet()
	servicePackage, err := resolveServicePackage(service, packages, imports)
	if err != nil {
		return "", err
	}
	applicationAlias := imports.add(rootImport+"/"+service.Domain+"/application", "application")
	policyAlias := imports.add(rootImport+"/"+service.Domain+"/policy", "policy")
	imports.add("google.golang.org/grpc", "grpc")
	imports.add("google.golang.org/grpc/metadata", "grpcmetadata")
	imports.add("yunka.io/framework/execution", "execution")
	imports.add("yunka.io/framework/operation", "operation")
	imports.add("yunka.io/gateway/rpc/transport/grpc", "gatewaygrpc")
	serverName := c9ServerName(naming)
	registerName := c9RegisterName(naming)
	var methods strings.Builder
	for _, method := range service.Methods {
		request, err := resolveGoType(method.Request, packages, imports)
		if err != nil {
			return "", err
		}
		response, err := resolveGoType(method.Response, packages, imports)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&methods, "func (server *%s) %s(ctx context.Context, request *%s.%s) (*%s.%s, error) {\n", serverName, method.Name, request.Alias, request.Type, response.Alias, response.Type)
		methods.WriteString("\tif metadata, ok := grpcmetadata.FromIncomingContext(ctx); ok { if values := metadata.Get(\"idempotency-key\"); len(values) > 0 { ctx = execution.WithIdempotencyKey(ctx, values[0]) } }\n")
		fmt.Fprintf(&methods, "\tresponse, err := operation.ExecuteTyped(ctx, server.executor, %s.%s(), request, server.application.%s)\n", policyAlias, c9PlanFunction(naming, method), method.Name)
		methods.WriteString("\tif err != nil { return nil, gatewaygrpc.OperationError(err) }\n\treturn response, nil\n}\n\n")
	}
	var b strings.Builder
	b.WriteString(GeneratedApplicationMarker + "\n\npackage rpc\n\nimport (\n\t\"context\"\n\t\"errors\"\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "type %s struct {\n\t%s.Unimplemented%sServer\n\tapplication %s.%s\n\texecutor operation.Executor\n}\n\n", serverName, servicePackage.Alias, service.Name, applicationAlias, naming.ApplicationInterface)
	fmt.Fprintf(&b, "func %s(registrar grpc.ServiceRegistrar, application %s.%s, executor operation.Executor) error {\n", registerName, applicationAlias, naming.ApplicationInterface)
	b.WriteString("\tif registrar == nil { return errors.New(\"contract C9 RPC adapter: registrar is required\") }\n")
	b.WriteString("\tif application == nil { return errors.New(\"contract C9 RPC adapter: application is required\") }\n")
	b.WriteString("\tif executor == nil { return errors.New(\"contract C9 RPC adapter: operation executor is required\") }\n")
	fmt.Fprintf(&b, "\t%s.Register%sServer(registrar, &%s{application: application, executor: executor})\n\treturn nil\n}\n\n", servicePackage.Alias, service.Name, serverName)
	b.WriteString(methods.String())
	return b.String(), nil
}

func renderC9RESTAdapter(service Service, packages []protoGoPackage, messages map[string]Message, rootImport string, naming serviceCodegenNaming) (string, error) {
	imports := newImportSet()
	applicationAlias := imports.add(rootImport+"/"+service.Domain+"/application", "application")
	policyAlias := imports.add(rootImport+"/"+service.Domain+"/policy", "policy")
	imports.add("yunka.io/framework/execution", "execution")
	imports.add("yunka.io/framework/operation", "operation")
	imports.add("yunka.io/gateway/authz", "authz")
	imports.add("google.golang.org/protobuf/encoding/protojson", "protojson")

	handlerType := c9RESTHandlerName(naming)
	registerName := c9RegisterName(naming)
	errorName := c9RESTErrorName(naming)
	var handlers strings.Builder
	var registrations strings.Builder
	bindingCount := 0
	for _, method := range service.Methods {
		requestRef, err := resolveGoType(method.Request, packages, imports)
		if err != nil {
			return "", err
		}
		requestMessage := messages[method.Request]
		for index, binding := range method.HTTP {
			bindingCount++
			handlerName := "handleOperation" + method.Name
			if len(method.HTTP) > 1 {
				handlerName += strconv.Itoa(index + 1)
			}
			pattern := strings.ToUpper(binding.Method) + " " + binding.Path
			fmt.Fprintf(&registrations, "\tmux.HandleFunc(%q, handler.%s)\n", pattern, handlerName)
			fmt.Fprintf(&handlers, "func (handler *%s) %s(writer http.ResponseWriter, request *http.Request) {\n", handlerType, handlerName)
			fmt.Fprintf(&handlers, "\twire := &%s.%s{}\n", requestRef.Alias, requestRef.Type)
			pathFields, _ := simplePathFields(binding.Path)
			if binding.Body == "*" {
				imports.add("io", "io")
				handlers.WriteString("\tbody, err := io.ReadAll(request.Body)\n\tif err != nil { http.Error(writer, \"invalid request body\", http.StatusBadRequest); return }\n\tif len(body) > 0 { if err := protojson.Unmarshal(body, wire); err != nil { http.Error(writer, \"invalid request body\", http.StatusBadRequest); return } }\n")
			} else {
				pathSet := make(map[string]struct{}, len(pathFields))
				for _, value := range pathFields {
					pathSet[value] = struct{}{}
				}
				for _, field := range requestMessage.Fields {
					if _, pathField := pathSet[field.Name]; pathField || field.Repeated || field.Map || field.Kind == "message" || field.Kind == "enum" {
						continue
					}
					queryExpr := "request.URL.Query().Get(" + strconv.Quote(field.Name) + ")"
					fmt.Fprintf(&handlers, "\tif raw := %s; raw != \"\" {\n", queryExpr)
					if scalarAssignmentNeedsStrconv(field) {
						imports.add("strconv", "strconv")
					}
					if err := writeScalarAssignment(&handlers, "wire", field, "raw", false); err != nil {
						return "", err
					}
					handlers.WriteString("\t}\n")
				}
			}
			for _, fieldName := range pathFields {
				field, ok := findMessageField(requestMessage, fieldName)
				if !ok {
					return "", fmt.Errorf("contract C9 application codegen: %s path field %q not found in %s", method.FullName, fieldName, method.Request)
				}
				if scalarAssignmentNeedsStrconv(field) {
					imports.add("strconv", "strconv")
				}
				if err := writeScalarAssignment(&handlers, "wire", field, "request.PathValue("+strconv.Quote(fieldName)+")", true); err != nil {
					return "", fmt.Errorf("contract C9 application codegen: %s: %w", method.FullName, err)
				}
			}
			handlers.WriteString("\tcallContext := execution.WithIdempotencyKey(request.Context(), request.Header.Get(\"Idempotency-Key\"))\n")
			fmt.Fprintf(&handlers, "\toutput, err := operation.ExecuteTyped(callContext, handler.executor, %s.%s(), wire, handler.application.%s)\n", policyAlias, c9PlanFunction(naming, method), method.Name)
			fmt.Fprintf(&handlers, "\tif err != nil { %s(writer, err); return }\n", errorName)
			handlers.WriteString("\tpayload, err := protojson.Marshal(output)\n\tif err != nil { http.Error(writer, \"response encoding failed\", http.StatusInternalServerError); return }\n\twriter.Header().Set(\"Content-Type\", \"application/json\")\n\t_, _ = writer.Write(payload)\n}\n\n")
		}
	}
	if bindingCount == 0 {
		return GeneratedApplicationMarker + "\n\npackage rest\n", nil
	}
	var b strings.Builder
	b.WriteString(GeneratedApplicationMarker + "\n\npackage rest\n\nimport (\n\t\"errors\"\n\t\"net/http\"\n")
	b.WriteString(imports.render())
	b.WriteString(")\n\n")
	fmt.Fprintf(&b, "type %s struct { application %s.%s; executor operation.Executor }\n\n", handlerType, applicationAlias, naming.ApplicationInterface)
	fmt.Fprintf(&b, "func %s(mux *http.ServeMux, application %s.%s, executor operation.Executor) error {\n", registerName, applicationAlias, naming.ApplicationInterface)
	b.WriteString("\tif mux == nil { return errors.New(\"contract C9 REST adapter: mux is required\") }\n")
	b.WriteString("\tif application == nil { return errors.New(\"contract C9 REST adapter: application is required\") }\n")
	b.WriteString("\tif executor == nil { return errors.New(\"contract C9 REST adapter: operation executor is required\") }\n")
	fmt.Fprintf(&b, "\thandler := &%s{application: application, executor: executor}\n", handlerType)
	b.WriteString(registrations.String())
	b.WriteString("\treturn nil\n}\n\n")
	fmt.Fprintf(&b, "func %s(writer http.ResponseWriter, err error) {\n", errorName)
	b.WriteString("\tif authz.IsDenied(err) {\n\t\tstatusCode := http.StatusForbidden\n\t\tvar denied *authz.DeniedError\n\t\tif errors.As(err, &denied) && (denied.Decision.Reason == authz.ReasonUnauthenticated || denied.Decision.Reason == authz.ReasonAuthenticationMethod) { statusCode = http.StatusUnauthorized }\n\t\thttp.Error(writer, http.StatusText(statusCode), statusCode); return\n\t}\n")
	b.WriteString("\tif errors.Is(err, execution.ErrIdempotencyKeyRequired) { http.Error(writer, \"idempotency key required\", http.StatusBadRequest); return }\n")
	b.WriteString("\tif errors.Is(err, execution.ErrIdempotencyInProgress) || errors.Is(err, execution.ErrIdempotencyCompleted) { http.Error(writer, \"idempotency conflict\", http.StatusConflict); return }\n")
	b.WriteString("\tif errors.Is(err, operation.ErrExecutorUnavailable) || errors.Is(err, operation.ErrSecurityUnavailable) || errors.Is(err, operation.ErrSecurityNilContext) || errors.Is(err, operation.ErrIdempotencyUnavailable) { http.Error(writer, \"operation execution unavailable\", http.StatusInternalServerError); return }\n")
	b.WriteString("\thttp.Error(writer, \"application request failed\", http.StatusBadRequest)\n}\n\n")
	b.WriteString(handlers.String())
	return b.String(), nil
}
