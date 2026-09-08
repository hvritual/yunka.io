package applicationboundary

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

type shape struct {
	typ    types.Type
	reason string
}
type valueRef struct {
	pkg      *SourcePackage
	fn       *function
	expr     ast.Expr
	object   *types.Var
	slot     int
	bindings map[*types.Var]valueRef
}

func unknown(reason string) []shape { return []shape{{reason: reason}} }

const maxProvenanceDepth = 32

// Provenance is deliberately bounded. Concrete Go values have fixed method sets.
// Interface conversions and single-assignment locals are followed; mutable
// interface cells, callbacks, recursive summaries and unsupported control flow
// are incomplete, not a guessed implementation. No application code is executed.
func (c *checker) expression(v valueRef, stack map[*types.Func]bool, depth int) []shape {
	if depth > maxProvenanceDepth {
		return unknown("constructor provenance depth exceeded")
	}
	if v.object != nil {
		return c.variable(v, v.object, stack, depth+1)
	}
	if v.expr == nil || v.pkg == nil || v.pkg.Info == nil {
		return unknown("missing expression/type evidence")
	}
	t := v.pkg.Info.TypeOf(v.expr)
	if tuple, ok := t.(*types.Tuple); ok {
		if v.slot >= tuple.Len() {
			return unknown("tuple result index unavailable")
		}
		t = tuple.At(v.slot).Type()
	}
	if t == nil {
		return unknown("expression has no type evidence")
	}
	if b, ok := t.(*types.Basic); ok && b.Kind() == types.UntypedNil {
		return []shape{{}}
	}
	if _, ok := t.(*types.TypeParam); ok {
		return unknown("type-parameter capability provenance is unsupported")
	}
	if _, ok := t.Underlying().(*types.Interface); !ok {
		return []shape{{typ: t}}
	}
	switch x := v.expr.(type) {
	case *ast.ParenExpr:
		v.expr = x.X
		return c.expression(v, stack, depth+1)
	case *ast.Ident:
		if obj, ok := v.pkg.Info.Uses[x].(*types.Var); ok {
			return c.variable(v, obj, stack, depth+1)
		}
	case *ast.CallExpr:
		// A small-interface conversion preserves the original object's methods.
		if tv, ok := v.pkg.Info.Types[x.Fun]; ok && tv.IsType() {
			if len(x.Args) != 1 {
				return unknown("invalid capability conversion")
			}
			v.expr = x.Args[0]
			v.slot = 0
			return c.expression(v, stack, depth+1)
		}
		obj := calledFunction(v.pkg, x.Fun)
		if obj == nil {
			return unknown("indirect constructor result has unknown dynamic implementation")
		}
		fn := c.functions[obj]
		if fn == nil || fn.decl.Body == nil {
			return []shape{{typ: t}}
		}
		sig := obj.Type().(*types.Signature)
		if sig.Recv() != nil || sig.Variadic() || sig.TypeParams().Len() != 0 {
			return unknown("method/generic/variadic interface-return provenance is unsupported")
		}
		bindings := map[*types.Var]valueRef{}
		if len(x.Args) == sig.Params().Len() {
			for i, arg := range x.Args {
				bindings[sig.Params().At(i)] = valueRef{pkg: v.pkg, fn: v.fn, expr: arg, bindings: v.bindings}
			}
		}
		return c.functionResult(fn, v.slot, bindings, stack, depth+1)
	case *ast.TypeAssertExpr:
		// A type assertion does not attenuate authority. The successful value retains
		// its concrete method set; an unresolved assertion cannot certify a wrapper.
		v.expr = x.X
		v.slot = 0
		return c.expression(v, stack, depth+1)
	}
	return []shape{{typ: t}}
}
func (c *checker) variable(v valueRef, obj *types.Var, stack map[*types.Func]bool, depth int) []shape {
	if depth > maxProvenanceDepth {
		return unknown("variable provenance depth exceeded")
	}
	if _, ok := obj.Type().Underlying().(*types.Interface); !ok {
		return []shape{{typ: obj.Type()}}
	}
	if obj.Pkg() == nil || obj.Parent() == obj.Pkg().Scope() || v.fn == nil || v.fn.decl.Body == nil {
		return []shape{{typ: obj.Type()}}
	}
	// Inventory every write, including closures. Address-taking can mutate an
	// interface without a visible assignment and therefore invalidates this proof.
	writes := []valueRef{}
	escapes := false
	ast.Inspect(v.fn.decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				ast.Inspect(x.X, func(n ast.Node) bool {
					if id, ok := n.(*ast.Ident); ok && v.pkg.Info.ObjectOf(id) == obj {
						escapes = true
					}
					return true
				})
			}
		case *ast.AssignStmt:
			for i, left := range x.Lhs {
				id, ok := left.(*ast.Ident)
				if !ok || v.pkg.Info.ObjectOf(id) != obj {
					continue
				}
				ref := valueRef{pkg: v.pkg, fn: v.fn, bindings: v.bindings}
				if len(x.Rhs) == len(x.Lhs) {
					ref.expr = x.Rhs[i]
				} else if len(x.Rhs) == 1 {
					ref.expr = x.Rhs[0]
					ref.slot = i
				}
				writes = append(writes, ref)
			}
		case *ast.ValueSpec:
			for i, id := range x.Names {
				if v.pkg.Info.Defs[id] != obj {
					continue
				}
				ref := valueRef{pkg: v.pkg, fn: v.fn, bindings: v.bindings}
				if len(x.Values) == len(x.Names) {
					ref.expr = x.Values[i]
				} else if len(x.Values) == 1 {
					ref.expr = x.Values[0]
					ref.slot = i
				}
				writes = append(writes, ref)
			}
		}
		return true
	})
	parameter := false
	params := v.fn.object.Type().(*types.Signature).Params()
	for i := 0; i < params.Len(); i++ {
		if params.At(i) == obj {
			parameter = true
			break
		}
	}
	// A parameter already has an incoming value. Even one subsequent assignment
	// is mutable: an earlier branch may have returned the incoming wide value.
	if escapes || len(writes) > 1 || (parameter && len(writes) > 0) {
		return unknown("mutable or address-escaped interface provenance is unsupported")
	}
	if len(writes) == 1 {
		if writes[0].expr == nil {
			return unknown("interface local has no unique initialization")
		}
		return c.expression(writes[0], stack, depth+1)
	}
	if binding, ok := v.bindings[obj]; ok {
		return c.expression(binding, stack, depth+1)
	}
	return []shape{{typ: obj.Type()}}
}
func (c *checker) functionResult(fn *function, index int, bindings map[*types.Var]valueRef, stack map[*types.Func]bool, depth int) []shape {
	if depth > maxProvenanceDepth || stack[fn.object] {
		return unknown("recursive or excessive constructor provenance is unsupported")
	}
	stack[fn.object] = true
	defer delete(stack, fn.object)
	sig := fn.object.Type().(*types.Signature)
	if sig.TypeParams().Len() != 0 {
		return unknown("generic constructor provenance is unsupported")
	}
	if index >= sig.Results().Len() {
		return unknown("constructor result index is unavailable")
	}
	var result []shape
	unsupported := false
	// The boolean means all paths through this statement terminate. This prevents
	// unreachable returns after nested blocks or exhaustive if arms from creating
	// false capability violations. Unsupported reachable flow remains incomplete.
	var walk func(ast.Stmt) bool
	walk = func(stmt ast.Stmt) bool {
		switch x := stmt.(type) {
		case nil:
			return false
		case *ast.BlockStmt:
			for _, s := range x.List {
				if walk(s) {
					return true
				}
			}
		case *ast.ReturnStmt:
			ref := valueRef{pkg: fn.pkg, fn: fn, bindings: bindings}
			if len(x.Results) == 0 {
				ref.object = sig.Results().At(index)
			} else if len(x.Results) == sig.Results().Len() {
				ref.expr = x.Results[index]
			} else if len(x.Results) == 1 {
				ref.expr = x.Results[0]
				ref.slot = index
			} else {
				unsupported = true
				return true
			}
			result = append(result, c.expression(ref, stack, depth+1)...)
			return true
		case *ast.IfStmt:
			value := fn.pkg.Info.Types[x.Cond].Value
			if value != nil && value.Kind() == constant.Bool {
				if constant.BoolVal(value) {
					return walk(x.Body)
				}
				return walk(x.Else)
			}
			bodyTerminates := walk(x.Body)
			elseTerminates := walk(x.Else)
			return bodyTerminates && elseTerminates
		case *ast.DeclStmt, *ast.AssignStmt, *ast.ExprStmt, *ast.EmptyStmt:
			// Initializers are evaluated only when they feed the selected result. Bodies
			// of function literals do not return from this constructor.
		default:
			unsupported = true
		}
		return false
	}
	walk(fn.decl.Body)
	if unsupported {
		return unknown("constructor control flow is outside the bounded return analysis")
	}
	return result
}
