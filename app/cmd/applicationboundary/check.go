package applicationboundary

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
)

type function struct {
	pkg    *SourcePackage
	decl   *ast.FuncDecl
	object *types.Func
}
type checker struct {
	program   Program
	report    Report
	packages  map[string]*types.Package
	functions map[*types.Func]*function
	selected  map[*types.Func]Factory
}

// Analyze requires typed source for every selected factory. Missing facts and
// unresolved dynamic flows produce INCOMPLETE. It never guesses a business role.
func Analyze(program Program, policy Policy) Report {
	c := checker{program: program, report: newReport(policy), packages: map[string]*types.Package{}, functions: map[*types.Func]*function{}, selected: map[*types.Func]Factory{}}
	if err := policy.Validate(); err != nil {
		c.add(Unknown, "AG-TYPE-000", "policy", token.NoPos, err.Error())
		return c.done()
	}
	if program.Fset == nil || len(program.Packages) == 0 {
		c.add(Unknown, "AG-TYPE-000", "source", token.NoPos, "no typed source packages supplied")
		return c.done()
	}
	var addPackage func(*types.Package)
	addPackage = func(p *types.Package) {
		if p == nil || c.packages[p.Path()] != nil {
			return
		}
		c.packages[p.Path()] = p
		for _, i := range p.Imports() {
			addPackage(i)
		}
	}
	for i := range program.Packages {
		p := &program.Packages[i]
		if p.Types == nil || p.Info == nil || p.Info.Types == nil || p.Info.Defs == nil || p.Info.Uses == nil || len(p.Files) == 0 {
			c.add(Unknown, "AG-TYPE-000", "source", token.NoPos, "incomplete package syntax/type information")
			continue
		}
		addPackage(p.Types)
		c.report.Packages = append(c.report.Packages, p.Types.Path())
		for _, file := range p.Files {
			for _, decl := range file.Decls {
				f, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				obj, ok := p.Info.Defs[f.Name].(*types.Func)
				if ok {
					c.functions[obj.Origin()] = &function{p, f, obj.Origin()}
				}
			}
		}
	}
	for _, f := range policy.Factories {
		p := c.packages[f.Symbol.Package]
		if p == nil {
			c.add(Unknown, "AG-TYPE-000", f.Symbol.String(), token.NoPos, "selected factory package was not loaded")
			continue
		}
		obj, ok := p.Scope().Lookup(f.Symbol.Name).(*types.Func)
		if !ok || obj.Type().(*types.Signature).Recv() != nil {
			c.add(Unknown, "AG-TYPE-000", f.Symbol.String(), token.NoPos, "selected package-level factory function was not found")
			continue
		}
		fn := c.functions[obj.Origin()]
		if fn == nil || fn.decl.Body == nil {
			c.add(Unknown, "AG-TYPE-000", f.Symbol.String(), obj.Pos(), "selected factory body is unavailable")
			continue
		}
		c.selected[obj.Origin()] = f
		c.report.CheckedFactories++
		sig := obj.Type().(*types.Signature)
		for _, s := range f.Results {
			iface, valid := c.slot(f, s, sig.Results(), obj.Pos())
			if !valid {
				continue
			}
			shapes := c.functionResult(fn, s.Index, nil, map[*types.Func]bool{}, 0)
			c.checkShapes(f.Symbol.String(), obj.Pos(), iface, shapes)
		}
		for _, s := range f.Arguments {
			c.slot(f, s, sig.Params(), obj.Pos())
		}
	}
	for i := range program.Packages {
		p := &program.Packages[i]
		if p.Info == nil || p.Types == nil {
			continue
		}
		for _, file := range p.Files {
			c.references(p, file)
		}
	}
	return c.done()
}
func (c *checker) done() Report { c.report.finish(); return c.report }
func (c *checker) add(class, rule, subject string, pos token.Pos, message string) {
	f := Finding{Rule: rule, Class: class, Subject: subject, Message: message, Position: pos}
	if c.program.Fset != nil && pos.IsValid() {
		p := c.program.Fset.PositionFor(pos, false)
		f.File = filepath.ToSlash(p.Filename)
		if c.program.Root != "" {
			if rel, e := filepath.Rel(c.program.Root, p.Filename); e == nil && !strings.HasPrefix(rel, "..") {
				f.File = filepath.ToSlash(rel)
			}
		}
		f.Line = p.Line
		f.Column = p.Column
	}
	c.report.Findings = append(c.report.Findings, f)
}
func (c *checker) slot(f Factory, s Slot, tuple *types.Tuple, pos token.Pos) (*types.Interface, bool) {
	subject := f.Symbol.String()
	p := c.packages[s.Contract.Package]
	if p == nil {
		c.add(Unknown, "AG-TYPE-000", subject, pos, "contract package was not loaded: "+s.Contract.Package)
		return nil, false
	}
	obj, ok := p.Scope().Lookup(s.Contract.Name).(*types.TypeName)
	if !ok {
		c.add(Unknown, "AG-TYPE-000", subject, pos, "contract type was not found: "+s.Contract.String())
		return nil, false
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok || iface.NumMethods() == 0 || !iface.IsMethodSet() {
		c.add(Unknown, "AG-TYPE-000", subject, pos, "contract must be a non-empty ordinary interface: "+s.Contract.String())
		return nil, false
	}
	iface.Complete()
	if s.Index >= tuple.Len() {
		c.add(Unknown, "AG-TYPE-000", subject, pos, fmt.Sprintf("contract slot %d does not exist", s.Index))
		return nil, false
	}
	if !types.Identical(tuple.At(s.Index).Type(), obj.Type()) {
		c.add(ProvenViolation, "AG-TYPE-002", subject, pos, fmt.Sprintf("slot %d must declare exact contract %s, got %s", s.Index, s.Contract, typeName(tuple.At(s.Index).Type())))
		return iface, true
	}
	return iface, true
}
func typeName(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Path() })
}
func calledFunction(p *SourcePackage, e ast.Expr) *types.Func {
	for {
		switch n := e.(type) {
		case *ast.ParenExpr:
			e = n.X
		case *ast.IndexExpr:
			e = n.X
		case *ast.IndexListExpr:
			e = n.X
		default:
			goto unwrapped
		}
	}
unwrapped:
	var obj types.Object
	switch n := e.(type) {
	case *ast.Ident:
		obj = p.Info.Uses[n]
	case *ast.SelectorExpr:
		if sel := p.Info.Selections[n]; sel != nil {
			obj = sel.Obj()
		} else {
			obj = p.Info.Uses[n.Sel]
		}
	}
	f, _ := obj.(*types.Func)
	if f != nil {
		return f.Origin()
	}
	return nil
}
func (c *checker) references(p *SourcePackage, file *ast.File) {
	// Build exact enclosing direct-call identities. Handles passed/returned as
	// values cannot silently evade checks through aliases or callback registries.
	direct := map[*ast.Ident]*ast.CallExpr{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := call.Fun
		for {
			switch x := fun.(type) {
			case *ast.ParenExpr:
				fun = x.X
			case *ast.IndexExpr:
				fun = x.X
			case *ast.IndexListExpr:
				fun = x.X
			default:
				goto done
			}
		}
	done:
		switch x := fun.(type) {
		case *ast.Ident:
			direct[x] = call
		case *ast.SelectorExpr:
			direct[x.Sel] = call
		}
		return true
	})
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		obj, ok := p.Info.Uses[id].(*types.Func)
		if !ok {
			return true
		}
		f, ok := c.selected[obj.Origin()]
		if !ok {
			return true
		}
		c.report.CheckedReferences++
		allowed := false
		for _, pkg := range f.AllowedCallers {
			if pkg == p.Types.Path() {
				allowed = true
				break
			}
		}
		if !allowed {
			c.add(ProvenViolation, "AG-TYPE-001", f.Symbol.String(), id.Pos(), "factory referenced outside permitted composition packages: "+p.Types.Path())
		}
		call := direct[id]
		if call == nil {
			c.add(Unknown, "AG-TYPE-005", f.Symbol.String(), id.Pos(), "factory escapes a direct call; indirect invocation provenance is unsupported")
			return true
		}
		sig := obj.Type().(*types.Signature)
		for _, s := range f.Arguments {
			iface, valid := c.slot(f, s, sig.Params(), id.Pos())
			if !valid {
				continue
			}
			if s.Index >= len(call.Args) || call.Ellipsis.IsValid() {
				c.add(Unknown, "AG-TYPE-000", f.Symbol.String(), id.Pos(), "tuple/variadic argument provenance is unsupported")
				continue
			}
			fn := c.enclosing(p, call.Pos())
			v := c.expression(valueRef{pkg: p, fn: fn, expr: call.Args[s.Index]}, map[*types.Func]bool{}, 0)
			c.checkShapes(f.Symbol.String(), call.Args[s.Index].Pos(), iface, v)
		}
		return true
	})
}
func (c *checker) enclosing(p *SourcePackage, pos token.Pos) *function {
	for _, f := range c.functions {
		if f.pkg == p && f.decl.Pos() <= pos && pos <= f.decl.End() {
			return f
		}
	}
	return nil
}
func (c *checker) checkShapes(subject string, pos token.Pos, contract *types.Interface, shapes []shape) {
	sawValue := false
	for _, s := range shapes {
		if s.reason != "" {
			c.add(Unknown, "AG-TYPE-000", subject, pos, s.reason)
			continue
		}
		if s.typ == nil {
			continue
		}
		sawValue = true
		if !types.Implements(s.typ, contract) {
			c.add(ProvenViolation, "AG-TYPE-003", subject, pos, "value does not implement selected contract: "+typeName(s.typ))
			continue
		}
		allowed := map[string]bool{}
		for i := 0; i < contract.NumMethods(); i++ {
			allowed[contract.Method(i).Id()] = true
		}
		set := c.reachableMethodSet(s.typ)
		var extra []string
		for i := 0; i < set.Len(); i++ {
			m := set.At(i).Obj()
			if m.Exported() && !allowed[m.Id()] {
				extra = append(extra, m.Name())
			}
		}
		sort.Strings(extra)
		if len(extra) > 0 {
			c.add(ProvenViolation, "AG-TYPE-003", subject, pos, "capability exposes extra methods on "+typeName(s.typ)+": "+strings.Join(extra, ", "))
		}
		if _, ok := s.typ.Underlying().(*types.Interface); ok {
			if len(extra) == 0 {
				c.add(Unknown, "AG-TYPE-000", subject, pos, "dynamic implementation is unknown behind "+typeName(s.typ))
			}
			continue
		}
		for _, name := range exportedFields(s.typ) {
			c.add(ProvenViolation, "AG-TYPE-004", subject, pos, "opaque capability exposes field "+name+" on "+typeName(s.typ))
		}
	}
	if !sawValue {
		c.add(Unknown, "AG-TYPE-000", subject, pos, "no non-nil implementation evidence")
	}
}

// A private embedded representation may promote public fields. Enumerate possible
// names, then use Go's selector resolution from outside the owning package so
// shadowed/ambiguous fields are not incorrectly reported as publicly accessible.
func exportedFields(root types.Type) []string {
	candidates := map[string]bool{}
	seen := map[types.Type]bool{}
	var visit func(types.Type)
	visit = func(t types.Type) {
		t = types.Unalias(t)
		if p, ok := t.(*types.Pointer); ok {
			t = types.Unalias(p.Elem())
		}
		if seen[t] {
			return
		}
		seen[t] = true
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			return
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if f.Exported() {
				candidates[f.Name()] = true
			}
			if f.Embedded() {
				visit(f.Type())
			}
		}
	}
	visit(root)
	var names []string
	for name := range candidates {
		obj, _, _ := types.LookupFieldOrMethod(root, false, nil, name)
		if f, ok := obj.(*types.Var); ok && f.IsField() && f.Exported() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// A publicly nameable non-pointer result can be asserted out of its interface,
// stored in an addressable local and then used through pointer-receiver methods.
// Exported aliases make otherwise-private concrete types nameable as well.
func (c *checker) reachableMethodSet(t types.Type) *types.MethodSet {
	concrete := types.Unalias(t)
	named, ok := concrete.(*types.Named)
	if !ok || types.IsInterface(concrete) {
		return types.NewMethodSet(t)
	}
	nameable := named.Obj().Exported()
	for _, p := range c.packages {
		for _, name := range p.Scope().Names() {
			obj, ok := p.Scope().Lookup(name).(*types.TypeName)
			if ok && obj.Exported() && types.Identical(types.Unalias(obj.Type()), concrete) {
				nameable = true
			}
		}
	}
	if nameable {
		return types.NewMethodSet(types.NewPointer(concrete))
	}
	return types.NewMethodSet(t)
}
