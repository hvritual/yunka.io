package implementation

import (
	"go/ast"
	"go/parser"
	"sort"
	"strconv"
)

// isolatePortAliases gives editable starter signatures their own import namespace.
// A canonical alias may legally be named service, New, handle<Method>, or a local
// parameter. Reusing that spelling can conflict with starter declarations even
// though the generated port itself compiles. Only selector qualifiers are renamed;
// Go import paths, selected types, methods and the original canonical bytes remain
// unchanged. Clone signature ASTs so repeated plans do not mutate their input.
func isolatePortAliases(port portShape) (portShape, error) {
	result := port
	result.imports = make(map[string]string, len(port.imports))
	result.methods = make([]*ast.Field, 0, len(port.methods))
	var names []string
	for name := range port.imports {
		names = append(names, name)
	}
	sort.Strings(names)
	renames := make(map[string]string, len(names))
	for i, name := range names {
		alias := "_startertype" + strconv.Itoa(i)
		renames[name] = alias
		result.imports[alias] = port.imports[name]
	}
	for _, method := range port.methods {
		signature, err := expression(method.Type)
		if err != nil {
			return portShape{}, err
		}
		typ, err := parser.ParseExpr(signature)
		if err != nil {
			return portShape{}, err
		}
		ast.Inspect(typ, func(n ast.Node) bool {
			if selector, ok := n.(*ast.SelectorExpr); ok {
				if id, ok := selector.X.(*ast.Ident); ok {
					if alias, exists := renames[id.Name]; exists {
						id.Name = alias
					}
				}
			}
			return true
		})
		field := &ast.Field{Type: typ}
		for _, name := range method.Names {
			field.Names = append(field.Names, ast.NewIdent(name.Name))
		}
		result.methods = append(result.methods, field)
	}
	return result, nil
}
