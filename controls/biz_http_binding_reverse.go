//go:build ignore

// This runner-only tool consumes the historical Biz protobuf source without
// mutating its source, generated tree, or framework candidate under test.
// It is not part of Yunka product builds or a second contract compiler.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
)

type methodEvidence struct {
	Name       string `json:"method"`
	Request    string `json:"request"`
	Route      string `json:"route"`
	Field      string `json:"field"`
	JSONName   string `json:"jsonName"`
	Repeated   bool   `json:"repeated"`
	RESTSource string `json:"restSource"`
	OpenAPI    bool   `json:"openapiArrayQuery"`
	TS         bool   `json:"typeScriptMetadata"`
}

func main() {
	var biz, framework, googleProto, protoc string
	flag.StringVar(&biz, "biz", "", "exact historical Biz checkout")
	flag.StringVar(&framework, "framework", "", "exact framework candidate checkout")
	flag.StringVar(&googleProto, "google-proto", "", "pinned google.api.http schema root")
	flag.StringVar(&protoc, "protoc", "protoc", "locked compiler binary")
	flag.Parse()
	if err := run(biz, framework, googleProto, protoc); err != nil {
		fmt.Fprintln(os.Stderr, "BIZ_CONTRACT_REVERSE_QUALIFICATION_FAILED:", err)
		os.Exit(2)
	}
}

func run(biz, framework, googleProto, protoc string) error {
	if biz == "" || framework == "" || googleProto == "" {
		return errors.New("required checkout and schema roots missing")
	}
	root := filepath.Join(biz, "contracts", "proto")
	compiled, err := contract.Compile(context.Background(), contract.CompileOptions{
		Dir: root, Files: []string{"commercial/v1/entitlement.proto"},
		ProtoPaths: []string{filepath.Join(framework, "contracts", "proto"), googleProto},
		Protoc: protoc,
	})
	if err != nil {
		return fmt.Errorf("compile original Biz descriptor: %w", err)
	}
	manifest := compiled.Manifest
	var selected contract.Service
	found := false
	for _, service := range manifest.Services {
		if service.FullName == "commercial.v1.EntitlementManagementApplication" {
			selected, found = service, true
			break
		}
	}
	if !found || selected.Application == nil {
		return errors.New("original Biz EntitlementManagementApplication missing")
	}
	target := map[string]string{
		"ExplainEntitlements": "/v1/platform/tenants/{tenant_id}/entitlements",
		"GetMyEntitlements":  "/v1/tenant/entitlements",
	}
	result := make([]methodEvidence, 0, len(target))
	for _, method := range selected.Methods {
		path, check := target[method.Name]
		if !check {
			continue
		}
		if len(method.HTTP) != 1 || method.HTTP[0].Method != "GET" ||
			method.HTTP[0].Path != path || method.HTTP[0].Body != "" {
			return fmt.Errorf("%s HTTP binding mismatches original contract", method.Name)
		}
		var request *contract.Message
		for i := range manifest.Messages {
			if manifest.Messages[i].FullName == method.Request {
				request = &manifest.Messages[i]
				break
			}
		}
		if request == nil {
			return fmt.Errorf("%s canonical request missing", method.Name)
		}
		count := 0
		for _, field := range request.Fields {
			if field.Name == "capability_codes" {
				count++
				if !field.Repeated || field.Kind != "scalar" ||
					field.Type != "string" || field.JSONName != "capabilityCodes" {
					return fmt.Errorf("%s repeated protobuf field semantics changed: %#v", method.Name, field)
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("%s expected one source capability_codes field, got %d", method.Name, count)
		}
		result = append(result, methodEvidence{
			Name: method.Name, Request: method.Request, Route: path,
			Field: "capability_codes", JSONName: "capabilityCodes", Repeated: true,
		})
	}
	if len(result) != 2 {
		return fmt.Errorf("missing source methods; verified %d of 2", len(result))
	}
	// Compile and validate the untouched original manifest wherever feasible.
	// Cross-domain requires are unrelated to these two query-bindings and may
	// be absent when only this one original source file is compiled.
	openapi, err := contract.GenerateOpenAPI(manifest, contract.OpenAPIOptions{})
	if err != nil {
		return fmt.Errorf("OpenAPI original source projection: %w", err)
	}
	var oas struct {
		Paths map[string]map[string]struct {
			Parameters []struct {
				In     string `json:"in"`
				Name   string `json:"name"`
				Schema struct {
					Type  string `json:"type"`
					Items struct {
						Type string `json:"type"`
					} `json:"items"`
				} `json:"schema"`
			} `json:"parameters"`
		} `json:"paths"`
	}
	if err = json.Unmarshal(openapi, &oas); err != nil {
		return err
	}
	ts, err := contract.GenerateTypeScript(manifest, contract.TypeScriptOptions{})
	if err != nil {
		return fmt.Errorf("TypeScript original source projection: %w", err)
	}
	for i := range result {
		for _, param := range oas.Paths[result[i].Route]["get"].Parameters {
			if param.In == "query" && param.Name == "capabilityCodes" &&
				param.Schema.Type == "array" && param.Schema.Items.Type == "string" {
				result[i].OpenAPI = true
			}
		}
		if !result[i].OpenAPI {
			return fmt.Errorf("%s original OpenAPI projection did not publish array query", result[i].Name)
		}
		result[i].TS = strings.Contains(string(ts), `{ field: "capability_codes", jsonName: "capabilityCodes", repeated: true }`)
		if !result[i].TS {
			return fmt.Errorf("%s original TypeScript projection lost repeated query semantics", result[i].Name)
		}
	}

	fullGraph := true
	files, renderErr := contract.RenderC9ApplicationCode(manifest, contract.ApplicationCodeOptions{
		RootImport: "github.com/hvritual/biz/internal",
	})
	if renderErr != nil {
		// Scope only the unrelated App dependency topology while retaining the
		// two actual annotated methods and descriptor-derived request fields.
		// Never claim this is a full Biz binary or a consumer-version upgrade.
		fullGraph = false
		selected.Application.Requires = nil
		methods := make([]contract.Method, 0, 2)
		for _, method := range selected.Methods {
			if _, ok := target[method.Name]; !ok {
				continue
			}
			if method.Operation != nil {
				copyOperation := *method.Operation
				copyOperation.RequiresOperations = nil
				method.Operation = &copyOperation
			}
			methods = append(methods, method)
		}
		selected.Methods = methods
		manifest.Services = []contract.Service{selected}
		files, err = contract.RenderC9ApplicationCode(manifest, contract.ApplicationCodeOptions{
			RootImport: "github.com/hvritual/biz/internal",
		})
		if err != nil {
			return fmt.Errorf("original Biz scoped C9 projection after full render %v: %w", renderErr, err)
		}
	}
	for _, file := range files {
		if !strings.Contains(file.Path, "/transport/rest/") {
			continue
		}
		source := string(file.Content)
		for i := range result {
			if !strings.Contains(source, "handleOperation"+result[i].Name+"(") {
				continue
			}
			if !strings.Contains(source, `query["capabilityCodes"]`) ||
				!strings.Contains(source, `query["capability_codes"]`) ||
				!strings.Contains(source, "append(wire.CapabilityCodes, value)") {
				return fmt.Errorf("generated Biz %s REST handler omitted list/alias forwarding", result[i].Name)
			}
			result[i].RESTSource = file.Path
		}
	}
	for _, sample := range result {
		if sample.RESTSource == "" {
			return fmt.Errorf("%s generated REST handler missing", sample.Name)
		}
	}
	evidence := struct {
		Scope           string           `json:"scope"`
		DescriptorSHA   string           `json:"descriptorSHA256"`
		FullGraphRender bool             `json:"fullGraphRender"`
		GraphError      string           `json:"unrelatedGraphRenderError,omitempty"`
		Methods         []methodEvidence `json:"methods"`
	}{
		Scope: "Original Biz entitlement.proto: real protobuf descriptor and scoped projection (no Consumer runtime execution)",
		DescriptorSHA: compiled.DescriptorSHA, FullGraphRender: fullGraph,
		Methods: result,
	}
	if !fullGraph {
		evidence.GraphError = renderErr.Error()
	}
	encoded, err := json.MarshalIndent(evidence, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}
