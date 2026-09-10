package implementation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/hvritual/yunka.io/pkg/contract"
	"yunka.io/app/cmd/applicationboundary"
	"yunka.io/app/cmd/projectflow"
)

type portShape struct {
	name, path string
	source     []byte
	imports    map[string]string
	methods    []*ast.Field
}

// Select only exact canonical rendered ports. The service comment identifies the
// source service; ambiguous matches fail instead of guessing a naming convention.
func selectPort(files []contract.GeneratedApplicationFile, service contract.Service) (portShape, error) {
	var candidates []portShape
	for _, file := range files {
		if path.Dir(file.Path) != service.Domain+"/application" || !strings.HasSuffix(file.Path, "_application_port_gen.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file.Path, file.Content, parser.ParseComments)
		if err != nil {
			return portShape{}, err
		}
		imports := map[string]string{}
		for _, spec := range parsed.Imports {
			value, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return portShape{}, err
			}
			alias := path.Base(value)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			if alias == "_" || alias == "." {
				return portShape{}, fmt.Errorf("add implementation: unsupported canonical import alias")
			}
			imports[alias] = value
		}
		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Doc == nil || gen.Doc.Text() != service.Name+" is generated from PB and contains no business implementation.\n" {
				continue
			}
			for _, spec := range gen.Specs {
				typ, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				iface, ok := typ.Type.(*ast.InterfaceType)
				if !ok {
					continue
				}
				candidates = append(candidates, portShape{name: typ.Name.Name, path: file.Path, source: file.Content, imports: imports, methods: iface.Methods.List})
			}
		}
	}
	if len(candidates) != 1 {
		return portShape{}, fmt.Errorf("add implementation: expected one canonical port for %s, found %d", service.FullName, len(candidates))
	}
	p := candidates[0]
	if len(p.methods) == 0 {
		return portShape{}, fmt.Errorf("add implementation: declare at least one Operation before creating implementation")
	}
	return p, nil
}

func expression(node ast.Node) (string, error) {
	var b bytes.Buffer
	err := format.Node(&b, token.NewFileSet(), node)
	return b.String(), err
}
func importLines(aliases map[string]string) string {
	var names []string
	for name := range aliases {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		fmt.Fprintf(&b, "%s %q\n", name, aliases[name])
	}
	return b.String()
}
func neededImports(node ast.Node, all map[string]string) map[string]string {
	used := map[string]string{}
	ast.Inspect(node, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				if imp, found := all[id.Name]; found {
					used[id.Name] = imp
				}
			}
		}
		return true
	})
	return used
}
func freshAlias(base string, imports map[string]string) string {
	for {
		if _, found := imports[base]; !found {
			return base
		}
		base += "_"
	}
}

func starterFiles(port portShape, owner, contractImport, key, caller string) (map[string][]byte, error) {
	return starterFilesWithDependencies(port, owner, contractImport, key, caller, nil)
}

func starterFilesWithDependencies(port portShape, owner, contractImport, key, caller string, dependencies []dependencyShape) (map[string][]byte, error) {
	port, err := isolatePortAliases(port)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	addGo := func(name, body string) error {
		data, err := format.Source([]byte("// Editable Yunka implementation starter. Not generated code; owned by the developer.\n" + body))
		if err != nil {
			return fmt.Errorf("add implementation: format %s: %w", name, err)
		}
		files[name] = data
		return nil
	}
	ownerBody := fmt.Sprintf("package owner\nimport ( _port %q; _impl %q )\n\n// Build is a resource-free composition entry point, not a business invocation API.\nfunc Build() _port.%s {return _impl.New()}\n", contractImport, owner+"/internal/usecase", port.name)
	if len(dependencies) > 0 {
		params := make([]string, 0, len(dependencies))
		args := make([]string, 0, len(dependencies))
		for i, dependency := range dependencies {
			params = append(params, fmt.Sprintf("dependency%d _port.%s", i, dependency.ContractName))
			args = append(args, fmt.Sprintf("dependency%d", i))
		}
		ownerBody = fmt.Sprintf("package owner\nimport ( _port %q; _impl %q )\n\n// Build accepts only generated source-edge child capabilities.\nfunc Build(%s) (_port.%s,error) {return _impl.New(%s)}\n", contractImport, owner+"/internal/usecase", strings.Join(params, ","), port.name, strings.Join(args, ","))
	}
	if err := addGo("build.go", ownerBody); err != nil {
		return nil, err
	}
	allImports := map[string]string{}
	for _, method := range port.methods {
		for a, p := range neededImports(method.Type, port.imports) {
			allImports[a] = p
		}
	}
	portAlias := freshAlias("_starterport", allImports)
	allImports[portAlias] = contractImport
	var fields, delegates, initializers strings.Builder
	names := map[string]bool{}
	for i, method := range port.methods {
		fn, ok := method.Type.(*ast.FuncType)
		if !ok || len(method.Names) != 1 || !ast.IsExported(method.Names[0].Name) || fn.Params == nil || fn.Results == nil || len(fn.Params.List) != 2 || len(fn.Results.List) != 2 {
			return nil, fmt.Errorf("add implementation: unsupported canonical method signature")
		}
		name := method.Names[0].Name
		filename, err := projectflow.ImplementationHandlerFilename(name)
		if err != nil {
			return nil, err
		}
		if names[filename] {
			return nil, fmt.Errorf("add implementation: case-folded method path conflict: %s", name)
		}
		names[filename] = true
		if len(fn.Params.List[0].Names) > 0 || len(fn.Params.List[1].Names) > 0 {
			return nil, fmt.Errorf("add implementation: unexpected named canonical parameters")
		}
		first, ok := fn.Params.List[0].Type.(*ast.SelectorExpr)
		if !ok {
			return nil, fmt.Errorf("add implementation: missing context parameter")
		}
		id, ok := first.X.(*ast.Ident)
		if !ok || port.imports[id.Name] != "context" || first.Sel.Name != "Context" {
			return nil, fmt.Errorf("add implementation: first parameter must be context.Context")
		}
		if _, ok := fn.Params.List[1].Type.(*ast.StarExpr); !ok {
			return nil, fmt.Errorf("add implementation: non-pointer request")
		}
		if _, ok := fn.Results.List[0].Type.(*ast.StarExpr); !ok {
			return nil, fmt.Errorf("add implementation: non-pointer response")
		}
		errorType, ok := fn.Results.List[1].Type.(*ast.Ident)
		if !ok || errorType.Name != "error" {
			return nil, fmt.Errorf("add implementation: missing error result")
		}
		signature, err := expression(fn)
		if err != nil {
			return nil, err
		}
		signature = strings.TrimPrefix(signature, "func")
		imports := neededImports(fn, port.imports)
		var dependencyFields strings.Builder
		dependencyAlias := ""
		for dependencyIndex, dependency := range dependencies {
			if !dependency.Uses(name) {
				continue
			}
			if dependencyAlias == "" {
				dependencyAlias = freshAlias("_starterdependency", imports)
				imports[dependencyAlias] = dependency.ContractImport
			}
			fmt.Fprintf(&dependencyFields, "dependency%d %s.%s\n", dependencyIndex, dependencyAlias, dependency.ContractName)
		}
		errAlias := freshAlias("_startererrors", imports)
		imports[errAlias] = "errors"
		if dependencyFields.Len() == 0 {
			body := fmt.Sprintf("package usecase\nimport(\n%s)\n\n// handle%s owns this use case only. Add narrow dependencies here, not to the facade.\ntype handle%s struct{}\n\nfunc (h *handle%s) execute%s {return nil,%s.New(%q)}\n", importLines(imports), name, name, name, signature, errAlias, key+"."+name+": not implemented")
			if err := addGo("internal/usecase/"+filename, body); err != nil {
				return nil, err
			}
		} else {
			body := fmt.Sprintf("package usecase\nimport(\n%s)\n\n// handle%s owns this use case only. Dependencies are exact source-edge child capabilities.\ntype handle%s struct{\n%s}\n\nfunc (h *handle%s) execute%s {return nil,%s.New(%q)}\n", importLines(imports), name, name, dependencyFields.String(), name, signature, errAlias, key+"."+name+": not implemented")
			if err := addGo("internal/usecase/"+filename, body); err != nil {
				return nil, err
			}
		}
		fmt.Fprintf(&fields, "h%d handle%s\n", i, name)
		fmt.Fprintf(&initializers, "h%d: handle%s{", i, name)
		for dependencyIndex, dependency := range dependencies {
			if dependency.Uses(name) {
				fmt.Fprintf(&initializers, "dependency%d: dependency%d,", dependencyIndex, dependencyIndex)
			}
		}
		initializers.WriteString("},")
		ctxType, err := expression(fn.Params.List[0].Type)
		if err != nil {
			return nil, err
		}
		reqType, err := expression(fn.Params.List[1].Type)
		if err != nil {
			return nil, err
		}
		respType, err := expression(fn.Results.List[0].Type)
		if err != nil {
			return nil, err
		}
		fmt.Fprintf(&delegates, "func (s *service) %s(ctx %s, request %s) (%s,error) {return s.h%d.execute(ctx,request)}\n", name, ctxType, reqType, respType, i)
	}
	var wiring string
	if len(dependencies) == 0 {
		wiring = fmt.Sprintf("package usecase\nimport(\n%s)\n\ntype service struct {\n%s}\n\n// New exposes only the canonical interface; never return or unwrap service.\nfunc New() %s.%s {return &service{}}\n\nvar _ %s.%s = (*service)(nil)\n\n%s", importLines(allImports), fields.String(), portAlias, port.name, portAlias, port.name, delegates.String())
	} else {
		errAlias := freshAlias("_startererrors", allImports)
		allImports[errAlias] = "errors"
		params := make([]string, 0, len(dependencies))
		var checks strings.Builder
		for i, dependency := range dependencies {
			params = append(params, fmt.Sprintf("dependency%d %s.%s", i, portAlias, dependency.ContractName))
			fmt.Fprintf(&checks, "if dependency%d == nil {return nil,%s.New(%q)}\n", i, errAlias, key+": dependency "+dependency.Key+" is required")
		}
		wiring = fmt.Sprintf("package usecase\nimport(\n%s)\n\ntype service struct {\n%s}\n\n// New accepts only generated source-edge child capabilities and exposes only the canonical interface.\nfunc New(%s) (%s.%s,error) {%sreturn &service{%s},nil}\n\nvar _ %s.%s = (*service)(nil)\n\n%s", importLines(allImports), fields.String(), strings.Join(params, ","), portAlias, port.name, checks.String(), initializers.String(), portAlias, port.name, delegates.String())
	}
	if err := addGo("internal/usecase/wiring.go", wiring); err != nil {
		return nil, err
	}
	factory := applicationboundary.Factory{Symbol: applicationboundary.Symbol{Package: owner, Name: "Build"}, AllowedCallers: []string{caller}, Results: []applicationboundary.Slot{{Index: 0, Contract: applicationboundary.Symbol{Package: contractImport, Name: port.name}}}}
	for i, dependency := range dependencies {
		factory.Arguments = append(factory.Arguments, applicationboundary.Slot{Index: i, Contract: applicationboundary.Symbol{Package: dependency.ContractImport, Name: dependency.ContractName}})
	}
	policy := applicationboundary.Policy{SchemaVersion: applicationboundary.SchemaVersion, Factories: []applicationboundary.Factory{factory}}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return nil, err
	}
	files["architecture.types.json"] = append(data, '\n')
	dependencyNote := "Cross-Application/infrastructure-dependent templates are a later AG-06 task."
	if len(dependencies) > 0 {
		dependencyNote = "Canonical Application `requires` edges use generated source-edge ChildCapability interfaces; infrastructure capability templates remain unsupported and are never guessed."
	}
	files["README.md"] = []byte(fmt.Sprintf(`# %s implementation starter

Developer-owned files. The canonical interface remains %s.%s.

Build is callable only from the explicitly selected composition package %s.
Connect it through the existing generated Assembly factory and Executor; do not
register a new transport handler or call Build inside a business use case.
Handlers deliberately return errors until implemented. This is not a running
business service, a sample database, a production fallback or a completed feature.

Each hidden handler owns one use case. Introduce only its needed ports and typed
capabilities; never inject a full Repository merely behind a narrower interface.
Generated PB/ports/Assembly remain generator-owned. Do not move or edit them.
After a contract change, use canonical generation and compile the implementation;
this create-only starter does not silently rewrite existing methods or policy.
%s

Validate using the existing commands (policy path is relative to module root):

    yunka generate
    yunka check --format agent-json
    yunka audit types --root . --policy <owner-path>/architecture.types.json --format agent-json
    go test ./...

Run the reviewed source policy separately with yunka audit source. This per-owner
type policy is not a whole-repository source policy or a new architecture graph.
Record task intent and decisions with the two templates below before adding code.
`, key, contractImport, port.name, caller, dependencyNote))
	files["docs/task-template.md"] = []byte(taskTemplate)
	files["docs/adr-template.md"] = []byte(adrTemplate)
	return files, nil
}

const taskTemplate = `# Implementation task

## Goal / non-goals
State the use case and the behavior that must remain unchanged.

## Canonical inputs and exact base
Record Operation IDs, current contract sources and the Git baseline. Do not copy a
second authoritative Application graph. Derive generated paths with Yunka context.

## Change boundary
Record exact editable paths, generator-owned paths, permitted dependencies and
whether this is contract, implementation or both. Run change plan/begin/check;
a template is not mutation authorization and cannot relax an existing ChangeSet.

## Acceptance
Specify named positive/negative behavioral tests, narrow capability and hidden
import checks, canonical generate/check, the reviewed type/source policies, and
real transaction/auth/tenant tests where affected. Unexecuted is not PASS.

## Rollback and evidence
Record the rollback commit and separate schema/data migration requirements.
Bind actual results to the final SHA/tree. Review and main readback are separate.
`
const adrTemplate = `# Architecture decision

## Context and decision to make
Explain the actual pressure, evidence and boundaries involved.

## Alternatives
Compare keeping the current design with concrete alternatives and their costs.

## Decision and consequences
State ownership, allowed dependencies, compatibility obligations and non-goals.
Do not infer new permissions, transactions or business rules from folder names.

## Confirmation
Name the positive/negative checks that demonstrate the decision and any unresolved
limitations. Include review ownership and temporary exception expiration.
`
