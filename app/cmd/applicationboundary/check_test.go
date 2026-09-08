package applicationboundary

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"sort"
	"strings"
	"testing"
)

const testPackage = "example.test/engine"
const definitions = `package engine
type Reader interface { Read() }
type wide struct{}
func (*wide) Read() {}
func (*wide) Delete() {}
type narrow struct { target *wide }
func (*narrow) Read() {}
`

func policyFor(name string) Policy {
	return Policy{SchemaVersion: 1, Factories: []Factory{{Symbol: Symbol{testPackage, name}, AllowedCallers: []string{testPackage}, Results: []Slot{{0, Symbol{testPackage, "Reader"}}}}}}
}

type sourceImporter struct {
	t       *testing.T
	fset    *token.FileSet
	sources map[string]string
	loaded  map[string]*SourcePackage
	loading map[string]bool
}

func (i *sourceImporter) Import(path string) (*types.Package, error) {
	if p := i.loaded[path]; p != nil {
		return p.Types, nil
	}
	source, ok := i.sources[path]
	if !ok {
		return importer.Default().Import(path)
	}
	if i.loading[path] {
		return nil, fmt.Errorf("import cycle: %s", path)
	}
	i.loading[path] = true
	defer delete(i.loading, path)
	f, err := parser.ParseFile(i.fset, "/source/"+path+"/source.go", source, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, err
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Scopes: map[ast.Node]*types.Scope{}}
	pkg, err := (&types.Config{Importer: i}).Check(path, i.fset, []*ast.File{f}, info)
	if err != nil {
		return nil, err
	}
	i.loaded[path] = &SourcePackage{Types: pkg, Info: info, Files: []*ast.File{f}}
	return pkg, nil
}
func programFor(t *testing.T, sources map[string]string) Program {
	t.Helper()
	i := &sourceImporter{t: t, fset: token.NewFileSet(), sources: sources, loaded: map[string]*SourcePackage{}, loading: map[string]bool{}}
	var names []string
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if _, e := i.Import(name); e != nil {
			t.Fatalf("fixture is not well typed: %v", e)
		}
	}
	p := Program{Fset: i.fset, Root: "/source"}
	for _, name := range names {
		p.Packages = append(p.Packages, *i.loaded[name])
	}
	return p
}
func TestCapabilityProvenance(t *testing.T) {
	cases := []struct{ name, body, status, rule string }{
		{"private_wrapper", `func Build() Reader { return &narrow{} }`, Pass, ""},
		{"private_helpers_allowed", `func (*narrow) hidden() {} ;func Build() Reader {return &narrow{}}`, Pass, ""},
		{"wide_value", `func Build() Reader {return &wide{}}`, Fail, "AG-TYPE-003"},
		{"narrow_conversion_of_wide", `func Build() Reader {return Reader(&wide{})}`, Fail, "AG-TYPE-003"},
		{"concrete_public_return", `func Build() *narrow {return &narrow{}}`, Fail, "AG-TYPE-002"},
		{"embedded_methods", `type view struct {*wide};func Build() Reader {return &view{}}`, Fail, "AG-TYPE-003"},
		{"unwrap", `func (*narrow) Unwrap() *wide{return &wide{}};func Build() Reader {return &narrow{}}`, Fail, "AG-TYPE-003"},
		{"exported_state", `type view struct {Target *wide};func (*view)Read(){};func Build()Reader{return &view{}}`, Fail, "AG-TYPE-004"},
		{"type_alias", `type Alias = Reader;type Value = narrow;func Build()Alias{return &Value{}}`, Pass, ""},
		{"single_interface_local", `func Build()Reader{var r Reader = &narrow{};return r}`, Pass, ""},
		{"single_wide_interface_local", `func Build()Reader{var r Reader = &wide{};return r}`, Fail, "AG-TYPE-003"},
		{"reassignment_incomplete_not_false_violation", `func Build()Reader{var r Reader=&wide{};r=&narrow{};return r}`, Incomplete, "AG-TYPE-000"},
		{"address_escape", `func replace(p *Reader){*p=&wide{}};func Build()Reader{var r Reader=&narrow{};replace(&r);return r}`, Incomplete, "AG-TYPE-000"},
		{"closure_write", `func Build()Reader{var r Reader=&narrow{};f:=func(){r=&wide{}};f();return r}`, Incomplete, "AG-TYPE-000"},
		{"helper_return", `func makeView()Reader{return &narrow{}};func Build()Reader{return makeView()}`, Pass, ""},
		{"helper_binding", `func identity(x Reader)Reader{return x};func Build()Reader{return identity(&narrow{})}`, Pass, ""},
		{"parameter_reassignment", `func helper(x Reader,b bool)Reader{if b{return x};x=&narrow{};return x};func Build()Reader{return helper(&wide{},true)}`, Incomplete, "AG-TYPE-000"},
		{"helper_binding_wide", `func identity(x Reader)Reader{return x};func Build()Reader{return identity(&wide{})}`, Fail, "AG-TYPE-003"},
		{"tuple_return", `func helper()(Reader,error){return &narrow{},nil};func Build()(Reader,error){return helper()}`, Pass, ""},
		{"tuple_local", `func helper()(Reader,error){return &narrow{},nil};func Build()Reader{r,_:=helper();return r}`, Pass, ""},
		{"named_result", `func Build()(r Reader){r=&narrow{};return}`, Pass, ""},
		{"nil_error_branch", `func Build(ok bool)(Reader,error){if !ok{return nil,nil};return &narrow{},nil}`, Pass, ""},
		{"all_nil", `func Build()Reader{return nil}`, Incomplete, "AG-TYPE-000"},
		{"constant_dead_branch", `func Build()Reader{if false{return &wide{}};return &narrow{}}`, Pass, ""},
		{"unknown_callback", `func Build(fn func()Reader)Reader{return fn()}`, Incomplete, "AG-TYPE-000"},
		{"unknown_input", `func Build(r Reader)Reader{return r}`, Incomplete, "AG-TYPE-000"},
		{"recursive_helper", `func helper()Reader{return helper()};func Build()Reader{return helper()}`, Incomplete, "AG-TYPE-000"},
		{"unsupported_switch", `func Build(x int)Reader{switch x{case 1:return &narrow{};default:return &wide{}}}`, Incomplete, "AG-TYPE-000"},
		{"unrelated_complex_logic", `func logic(x int)int{for x>0{x--;switch x{case 2:return x}};return x};func Build()Reader{return &narrow{}}`, Pass, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := programFor(t, map[string]string{testPackage: definitions + tc.body})
			r := Analyze(p, policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("status=%s want=%s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.rule != "" {
				found := false
				for _, f := range r.Findings {
					if f.Rule == tc.rule {
						found = true
					}
					if f.Line == 0 {
						t.Error("missing source location")
					}
				}
				if !found {
					t.Fatalf("missing expected rule %s: %+v", tc.rule, r.Findings)
				}
			}
			a, _ := r.Marshal()
			b, _ := Analyze(p, policyFor("Build")).Marshal()
			if string(a) != string(b) {
				t.Fatal("report is nondeterministic")
			}
		})
	}
}
func TestFactoryReferencesUseTypeIdentity(t *testing.T) {
	for _, alias := range []string{"engine", "renamed", "."} {
		t.Run(alias, func(t *testing.T) {
			name := alias + ".Build"
			if alias == "." {
				name = "Build"
			}
			p := programFor(t, map[string]string{testPackage: definitions + `func Build()Reader{return &narrow{}}`, "example.test/wiring": `package wiring;import ` + alias + ` "` + testPackage + `";func wire(){_=` + name + `()}`})
			policy := policyFor("Build")
			r := Analyze(p, policy)
			if r.Status != Fail || len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-001" {
				t.Fatalf("wrong caller result: %+v", r)
			}
			policy.Factories[0].AllowedCallers = append(policy.Factories[0].AllowedCallers, "example.test/wiring")
			r = Analyze(p, policy)
			if r.Status != Pass || r.CheckedReferences != 1 {
				t.Fatalf("legal composition denied: %+v", r)
			}
		})
	}
	t.Run("indirect_handle", func(t *testing.T) {
		r := Analyze(programFor(t, map[string]string{testPackage: definitions + `func Build()Reader{return &narrow{}};var alias=Build`}), policyFor("Build"))
		if r.Status != Incomplete || r.Findings[0].Rule != "AG-TYPE-005" {
			t.Fatalf("escaped handle accepted: %+v", r)
		}
	})
	t.Run("foreign_same_name", func(t *testing.T) {
		p := programFor(t, map[string]string{testPackage: definitions + `func Build()Reader{return &narrow{}}`, "example.test/other": `package other;func Build()int{return 1};func x(){_ = Build()}`})
		r := Analyze(p, policyFor("Build"))
		if r.Status != Pass || r.CheckedReferences != 0 {
			t.Fatalf("spelling used instead of identity: %+v", r)
		}
	})
}
func TestArgumentActualSurface(t *testing.T) {
	for _, tc := range []struct{ name, wire, status string }{
		{"narrow", `func wire(){_ = Build(&narrow{})}`, Pass},
		{"wide", `func wire(){_ = Build(&wide{})}`, Fail},
		{"conversion", `func wire(){_ = Build(Reader(&wide{}))}`, Fail},
		{"attenuator", `func attenuate(r Reader)Reader{return &narrow{}};func wire(){_ = Build(attenuate(&wide{}))}`, Pass},
		{"unknown", `func wire(r Reader){_ = Build(r)}`, Incomplete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := policyFor("Build")
			policy.Factories[0].Arguments = []Slot{{0, Symbol{testPackage, "Reader"}}}
			r := Analyze(programFor(t, map[string]string{testPackage: definitions + `func Build(r Reader)Reader{return &narrow{}};` + tc.wire}), policy)
			if r.Status != tc.status {
				t.Fatalf("got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
		})
	}
}
func TestCrossPackageConstructorProvenance(t *testing.T) {
	sources := map[string]string{
		"example.test/contracts": `package contracts;type Reader interface{Read()}`,
		"example.test/hidden":    `package hidden;import "example.test/contracts";type service struct{};func(*service)Read(){};func New()contracts.Reader{return &service{}}`,
		testPackage:              `package engine;import("example.test/contracts";"example.test/hidden");func Build()contracts.Reader{return hidden.New()}`,
	}
	policy := policyFor("Build")
	policy.Factories[0].Results[0].Contract = Symbol{"example.test/contracts", "Reader"}
	p := programFor(t, sources)
	if r := Analyze(p, policy); r.Status != Pass {
		t.Fatalf("cross-package narrow result: %+v", r)
	}
	sources["example.test/hidden"] += `;func(*service)Delete(){}`
	if r := Analyze(programFor(t, sources), policy); r.Status != Fail {
		t.Fatalf("hidden extra method not found: %+v", r)
	}
}
func TestMissingSubjectsCannotPass(t *testing.T) {
	p := programFor(t, map[string]string{testPackage: definitions + `func Build()Reader{return &narrow{}}`})
	for _, mutate := range []func(*Policy){func(p *Policy) { p.Factories[0].Symbol.Name = "Missing" }, func(p *Policy) { p.Factories[0].Symbol.Package = "missing" }, func(p *Policy) { p.Factories[0].Results[0].Contract.Name = "Missing" }, func(p *Policy) { p.Factories[0].Results[0].Index = 3 }, func(p *Policy) { p.Factories = nil }} {
		policy := policyFor("Build")
		mutate(&policy)
		if r := Analyze(p, policy); r.Status != Incomplete {
			t.Fatalf("missing evidence accepted: %+v", r)
		}
	}
	if r := Analyze(Program{}, policyFor("Build")); r.Status != Incomplete {
		t.Fatal("empty program passed")
	}
}
func TestStrictPolicy(t *testing.T) {
	valid := `{"schemaVersion":1,"factories":[{"symbol":{"package":"example.test/engine","name":"Build"},"allowedCallerPackages":["example.test/engine"],"results":[{"index":0,"contract":{"package":"example.test/engine","name":"Reader"}}]}]}`
	if _, e := ReadPolicy(strings.NewReader(valid)); e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{`{}`, `{"schemaVersion":1,"factories":[]}`, strings.Replace(valid, `"schemaVersion":1`, `"schemaVersion":1,"schemaVersion":1`, 1), strings.Replace(valid, `"name":"Build"`, `"name":"Build","extra":1`, 1), valid + `{}`, strings.ReplaceAll(valid, `example.test/engine`, `example.test/**`), strings.Replace(valid, `"index":0`, `"index":-1`, 1), strings.Repeat(" ", MaxPolicyBytes+1)} {
		if _, e := ReadPolicy(strings.NewReader(s)); e == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}
