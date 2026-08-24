package architecturepolicy

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Diagnostic struct {
	Path    string
	Message string
}

func CheckC7(root string) ([]Diagnostic, error) {
	root, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return nil, err
	}
	var diagnostics []Diagnostic
	autoload, err := checkAutoloadPackages(root)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, autoload...)
	composition, err := checkTypedComposition(root)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, composition...)
	legacy, err := checkLegacyRuntimeRemoved(root)
	if err != nil {
		return nil, err
	}
	diagnostics = append(diagnostics, legacy...)
	sort.Slice(diagnostics, func(left, right int) bool {
		if diagnostics[left].Path != diagnostics[right].Path {
			return diagnostics[left].Path < diagnostics[right].Path
		}
		return diagnostics[left].Message < diagnostics[right].Message
	})
	return diagnostics, nil
}

func checkAutoloadPackages(root string) ([]Diagnostic, error) {
	modules := filepath.Join(root, "modules")
	if _, err := os.Stat(modules); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var diagnostics []Diagnostic
	err := filepath.WalkDir(modules, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || filepath.Base(filepath.Dir(path)) != "autoload" {
			return nil
		}
		fileDiagnostics, err := CheckAutoloadFile(path)
		if err != nil {
			return err
		}
		for _, diagnostic := range fileDiagnostics {
			relative, _ := filepath.Rel(root, path)
			diagnostic.Path = filepath.ToSlash(relative)
			diagnostics = append(diagnostics, diagnostic)
		}
		return nil
	})
	return diagnostics, err
}

func CheckAutoloadFile(path string) ([]Diagnostic, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var diagnostics []Diagnostic
	if file.Name.Name != "autoload" {
		diagnostics = append(diagnostics, Diagnostic{Message: "autoload file must use package autoload"})
	}
	imports := make(map[string]string)
	for _, imported := range file.Imports {
		pathValue := strings.Trim(imported.Path.Value, `"`)
		name := filepath.Base(pathValue)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		if name == "_" || name == "." {
			diagnostics = append(diagnostics, Diagnostic{Message: "autoload imports may not be blank or dot imports"})
			continue
		}
		imports[name] = pathValue
	}
	if imports["modulecatalog"] != "yunka.io/framework/core/modulecatalog" {
		diagnostics = append(diagnostics, Diagnostic{Message: "autoload must import yunka.io/framework/core/modulecatalog as modulecatalog"})
	}

	initCount := 0
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if function.Name.Name != "init" {
			diagnostics = append(diagnostics, Diagnostic{Message: "autoload package may only declare init"})
			continue
		}
		initCount++
		if function.Recv != nil || function.Type.Params.NumFields() != 0 || function.Body == nil || len(function.Body.List) != 1 {
			diagnostics = append(diagnostics, Diagnostic{Message: "autoload init must contain exactly one descriptor registration"})
			continue
		}
		expression, ok := function.Body.List[0].(*ast.ExprStmt)
		if !ok || !isDescriptorRegistration(expression.X, imports) {
			diagnostics = append(diagnostics, Diagnostic{Message: "autoload init must only call modulecatalog.MustRegister(module.GeneratedDescriptor())"})
		}
	}
	if initCount != 1 {
		diagnostics = append(diagnostics, Diagnostic{Message: fmt.Sprintf("autoload package must declare exactly one init, found %d", initCount)})
	}
	for _, declaration := range file.Decls {
		if general, ok := declaration.(*ast.GenDecl); ok && general.Tok != token.IMPORT {
			diagnostics = append(diagnostics, Diagnostic{Message: "autoload package may not declare variables, constants, or types"})
		}
	}
	return diagnostics, nil
}

func isDescriptorRegistration(expression ast.Expr, imports map[string]string) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "MustRegister" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok || identifier.Name != "modulecatalog" || imports[identifier.Name] != "yunka.io/framework/core/modulecatalog" {
		return false
	}
	descriptorCall, ok := call.Args[0].(*ast.CallExpr)
	if !ok || len(descriptorCall.Args) != 0 {
		return false
	}
	descriptorSelector, ok := descriptorCall.Fun.(*ast.SelectorExpr)
	if !ok || descriptorSelector.Sel.Name != "GeneratedDescriptor" {
		return false
	}
	moduleAlias, ok := descriptorSelector.X.(*ast.Ident)
	if !ok || moduleAlias.Name == "modulecatalog" {
		return false
	}
	_, imported := imports[moduleAlias.Name]
	return imported
}

func checkTypedComposition(root string) ([]Diagnostic, error) {
	paths := []string{
		filepath.Join(root, "framework", "core", "modulecatalog"),
		filepath.Join(root, "framework", "kernel"),
		filepath.Join(root, "framework", "platform"),
		filepath.Join(root, "framework", "requestscope"),
	}
	forbiddenCalls := map[string]struct{}{
		"GetApp": {}, "GetClient": {}, "GetItem": {}, "GetConfV2": {},
		"GetService": {}, "GetRepo": {}, "GetInfra": {},
	}
	var diagnostics []Diagnostic
	for _, directory := range paths {
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			aliases := make(map[string]string)
			for _, imported := range file.Imports {
				pathValue := strings.Trim(imported.Path.Value, `"`)
				name := filepath.Base(pathValue)
				if imported.Name != nil {
					name = imported.Name.Name
				}
				aliases[name] = pathValue
				if pathValue == "reflect" {
					relative, _ := filepath.Rel(root, path)
					diagnostics = append(diagnostics, Diagnostic{Path: filepath.ToSlash(relative), Message: "typed composition may not import reflect"})
				}
				switch pathValue {
				case "yunka.io/framework/core/module", "yunka.io/pkg/di":
					relative, _ := filepath.Rel(root, path)
					diagnostics = append(diagnostics, Diagnostic{Path: filepath.ToSlash(relative), Message: "typed composition may not import legacy reflection containers"})
				case "yunka.io/framework/core/request":
					relative, _ := filepath.Rel(root, path)
					diagnostics = append(diagnostics, Diagnostic{Path: filepath.ToSlash(relative), Message: "typed request scopes may not depend on legacy request.Runtime"})
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				relative, _ := filepath.Rel(root, path)
				relativePath := filepath.ToSlash(relative)
				switch current := node.(type) {
				case *ast.SelectorExpr:
					if identifier, ok := current.X.(*ast.Ident); ok && aliases[identifier.Name] == "sync" && current.Sel.Name == "Pool" {
						diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "typed composition may not use sync.Pool"})
					}
				case *ast.CallExpr:
					name := ""
					switch function := current.Fun.(type) {
					case *ast.Ident:
						name = function.Name
					case *ast.SelectorExpr:
						name = function.Sel.Name
					}
					if _, forbidden := forbiddenCalls[name]; forbidden {
						diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "typed composition may not call " + name})
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return diagnostics, nil
}

func checkLegacyRuntimeRemoved(root string) ([]Diagnostic, error) {
	removedPaths := []string{
		"framework/core/module",
		"framework/ingress",
		"pkg/di",
		"app/cmd/controller",
		"framework/core/request/runtime.go",
		"framework/core/request/work_runtime.go",
		"framework/core/request/runtime_context.go",
		"framework/core/global.go",
		"framework/core/initiator.go",
		"framework/core/module.go",
		"framework/core/service.go",
		"framework/core/repository.go",
		"framework/infras/orm",
		"gateway/dispatcher/bridge/node_single.go",
		"gateway/dispatcher/proxy/proxy_multi.go",
		"gateway/rpc/bridge/provider.go",
		"gateway/rpc/bridge/runtime.go",
	}
	var diagnostics []Diagnostic
	for _, relative := range removedPaths {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			diagnostics = append(diagnostics, Diagnostic{Path: relative, Message: "C7.3 removed legacy path is present"})
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}

	forbiddenImports := map[string]struct{}{
		"yunka.io/framework/core/module": {},
		"yunka.io/framework/ingress":     {},
		"yunka.io/pkg/di":                {},
		"yunka.io/framework/infras/orm":  {},
	}
	forbiddenIdentifiers := map[string]struct{}{
		"Runtime": {}, "BaseRuntime": {}, "WorkRuntime": {},
		"ModuleGatewayProvider": {}, "WorkRuntimeFactory": {},
		"BaseService": {}, "BaseRepository": {}, "RuntimeInit": {}, "ServiceItem": {},
	}
	forbiddenCalls := map[string]struct{}{
		"SetRuntime": {}, "GetRuntime": {}, "PutService": {}, "GetRepo": {}, "GetInfra": {},
		"RegisterPrepare": {}, "RegisterInitiator": {}, "GetApp": {}, "AppRegisterRpc": {},
	}
	scanRoots := []string{
		"framework/core",
		"framework/infras",
		"gateway/dispatcher",
		"gateway/rpc/bridge",
		"app/cmd",
	}
	for _, relativeRoot := range scanRoots {
		directory := filepath.Join(root, filepath.FromSlash(relativeRoot))
		if _, err := os.Stat(directory); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			fileSet := token.NewFileSet()
			file, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return err
			}
			relative, _ := filepath.Rel(root, path)
			relativePath := filepath.ToSlash(relative)
			aliases := make(map[string]string)
			for _, imported := range file.Imports {
				importPath := strings.Trim(imported.Path.Value, `"`)
				alias := filepath.Base(importPath)
				if imported.Name != nil {
					alias = imported.Name.Name
				}
				aliases[alias] = importPath
				if _, forbidden := forbiddenImports[importPath]; forbidden {
					diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "C7.3 production code imports removed package " + importPath})
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				switch current := node.(type) {
				case *ast.SelectorExpr:
					if identifier, ok := current.X.(*ast.Ident); ok {
						if aliases[identifier.Name] == "sync" && current.Sel.Name == "Pool" && isRequestLifetimePath(relativePath) {
							diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "C7.3 request lifetime may not use sync.Pool"})
						}
						if aliases[identifier.Name] == "yunka.io/framework/core/request" && current.Sel.Name == "Runtime" {
							diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "C7.3 request.Runtime is removed"})
						}
					}
				case *ast.TypeSpec:
					if _, forbidden := forbiddenIdentifiers[current.Name.Name]; forbidden {
						diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "C7.3 legacy type " + current.Name.Name + " is removed"})
					}
				case *ast.CallExpr:
					name := ""
					switch function := current.Fun.(type) {
					case *ast.Ident:
						name = function.Name
					case *ast.SelectorExpr:
						name = function.Sel.Name
					}
					if _, forbidden := forbiddenCalls[name]; forbidden {
						diagnostics = append(diagnostics, Diagnostic{Path: relativePath, Message: "C7.3 legacy call " + name + " is removed"})
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return diagnostics, nil
}

func isRequestLifetimePath(relative string) bool {
	return strings.HasPrefix(relative, "framework/core/request/") ||
		strings.HasPrefix(relative, "gateway/dispatcher/proxy/") ||
		strings.HasPrefix(relative, "gateway/dispatcher/bridge/") ||
		strings.HasPrefix(relative, "gateway/rpc/bridge/")
}
