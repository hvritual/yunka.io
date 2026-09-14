package auditcore

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
	"unicode"
)

const nameExceptionDirective = "yunka:audit-name-exception"

type SourceDeclaration struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Receiver   string `json:"receiver,omitempty"`
	Exception  string `json:"exception,omitempty"`
	Documented bool   `json:"documented"`
	Contract   bool   `json:"contract,omitempty"`
}

func collectSourceDeclarations(file *ast.File, testFile bool) []SourceDeclaration {
	if file == nil {
		return []SourceDeclaration{}
	}
	var declarations []SourceDeclaration
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.FuncDecl:
			if value.Name == nil {
				continue
			}
			name := strings.TrimSpace(value.Name.Name)
			if name == "" {
				continue
			}
			kind := "func"
			receiver := ""
			if value.Recv != nil {
				if !ast.IsExported(name) {
					continue
				}
				kind = "method"
				receiver = receiverIdentity(value.Recv)
			} else if testFile && durableTestIdentity(name) {
				kind = "test"
			} else if !ast.IsExported(name) {
				continue
			}
			declarations = append(declarations, SourceDeclaration{
				Kind:       kind,
				Name:       name,
				Receiver:   receiver,
				Exception:  nameException(value.Doc),
				Documented: hasDocumentation(value.Doc),
			})
		case *ast.GenDecl:
			kind := ""
			switch value.Tok {
			case token.TYPE:
				kind = "type"
			case token.VAR:
				kind = "var"
			case token.CONST:
				kind = "const"
			default:
				continue
			}
			for _, spec := range value.Specs {
				switch typed := spec.(type) {
				case *ast.TypeSpec:
					if typed.Name == nil || !ast.IsExported(typed.Name.Name) {
						continue
					}
					exception := nameException(typed.Doc)
					if exception == "" {
						exception = nameException(value.Doc)
					}
					_, contract := typed.Type.(*ast.InterfaceType)
					declarations = append(declarations, SourceDeclaration{
						Kind:       kind,
						Name:       typed.Name.Name,
						Exception:  exception,
						Documented: hasDocumentation(typed.Doc) || hasDocumentation(value.Doc),
						Contract:   contract,
					})
				case *ast.ValueSpec:
					exception := nameException(typed.Doc)
					if exception == "" {
						exception = nameException(value.Doc)
					}
					documented := hasDocumentation(typed.Doc) || hasDocumentation(value.Doc)
					for _, name := range typed.Names {
						if name == nil || !ast.IsExported(name.Name) {
							continue
						}
						declarations = append(declarations, SourceDeclaration{
							Kind:       kind,
							Name:       name.Name,
							Exception:  exception,
							Documented: documented,
						})
					}
				}
			}
		}
	}
	normalizeDeclarations(declarations)
	if declarations == nil {
		return []SourceDeclaration{}
	}
	return declarations
}

func normalizeDeclarations(values []SourceDeclaration) {
	for index := range values {
		values[index].Kind = strings.TrimSpace(values[index].Kind)
		values[index].Name = strings.TrimSpace(values[index].Name)
		values[index].Receiver = strings.TrimSpace(values[index].Receiver)
		values[index].Exception = strings.TrimSpace(values[index].Exception)
	}
	sort.Slice(values, func(i, j int) bool {
		left, right := values[i], values[j]
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		if left.Receiver != right.Receiver {
			return left.Receiver < right.Receiver
		}
		return left.Name < right.Name
	})
}

func hasDocumentation(group *ast.CommentGroup) bool {
	if group == nil {
		return false
	}
	return strings.TrimSpace(group.Text()) != ""
}

func receiverIdentity(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	return receiverExpressionIdentity(fields.List[0].Type)
}

func receiverExpressionIdentity(expression ast.Expr) string {
	switch value := expression.(type) {
	case *ast.Ident:
		return strings.TrimSpace(value.Name)
	case *ast.StarExpr:
		return receiverExpressionIdentity(value.X)
	case *ast.IndexExpr:
		return receiverExpressionIdentity(value.X)
	case *ast.IndexListExpr:
		return receiverExpressionIdentity(value.X)
	default:
		return ""
	}
}

func durableTestIdentity(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			return true
		}
	}
	return false
}

func durableTestSubject(name string) string {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
			return strings.TrimPrefix(name, prefix)
		}
	}
	return name
}

func nameException(group *ast.CommentGroup) string {
	if group == nil {
		return ""
	}
	for _, comment := range group.List {
		if comment == nil {
			continue
		}
		text := strings.TrimSpace(comment.Text)
		text = strings.TrimSpace(strings.TrimPrefix(text, "//"))
		text = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/"))
		if !strings.HasPrefix(text, nameExceptionDirective) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(text, nameExceptionDirective))
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		category := strings.TrimSpace(fields[0])
		if category != "business-concept" && category != "compatibility-boundary" {
			continue
		}
		reason := strings.TrimSpace(strings.TrimPrefix(rest, category))
		if reason == "" {
			continue
		}
		return category + ": " + reason
	}
	return ""
}

func leadingIdentityWord(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var word []rune
	var previous rune
	for _, current := range []rune(value) {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			if len(word) > 0 {
				break
			}
			previous = 0
			continue
		}
		if len(word) > 0 && unicode.IsUpper(current) && unicode.IsLower(previous) {
			break
		}
		word = append(word, current)
		previous = current
	}
	return strings.ToLower(string(word))
}
