package applicationboundary

import "testing"

func TestAddressableExportedConcreteMethods(t *testing.T) {
	for _, tc := range []struct{ name, body, status string }{
		{"exported_value_pointer_method", `type Public struct{};func(Public)Read(){};func(*Public)Delete(){};func Build()Reader{return Public{}}`, Fail},
		{"exported_alias_pointer_method", `type private struct{};func(private)Read(){};func(*private)Delete(){};type Public = private;func Build()Reader{return private{}}`, Fail},
		{"private_value_no_public_alias", `type private struct{};func(private)Read(){};func(*private)Delete(){};func Build()Reader{return private{}}`, Pass},
		{"exported_value_still_narrow", `type Public struct{};func(Public)Read(){};func Build()Reader{return Public{}}`, Pass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: definitions + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("status=%s want=%s findings=%+v", r.Status, tc.status, r.Findings)
			}
			if tc.status == Fail && (len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-003") {
				t.Fatalf("wrong pointer-surface finding: %+v", r.Findings)
			}
		})
	}
}

func TestUnprovenNamedResultInitialization(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"dead_if", `func Build()(r Reader){if false{r=&narrow{}};return}`},
		{"uncalled_closure", `func Build()(r Reader){f:=func(){r=&narrow{}};_ = f;return}`},
		{"write_after_return", `func Build()(r Reader){return;r=&narrow{};return}`},
		{"conditional_return_before_write", `func Build()(r Reader){if true{return};r=&narrow{};return}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: definitions + tc.body}), policyFor("Build"))
			if r.Status != Incomplete {
				t.Fatalf("unproven initialization reported %s: %+v", r.Status, r.Findings)
			}
		})
	}
}

func TestNamedWideInterfaceKeepsItsDeclaredMethods(t *testing.T) {
	p := policyFor("Build")
	p.Factories[0].Arguments = []Slot{{0, Symbol{testPackage, "Reader"}}}
	source := definitions + `type Full interface{Reader;Delete()};func Build(r Reader)Reader{return &narrow{}};func wire(f Full){_=Build(f)}`
	r := Analyze(programFor(t, map[string]string{testPackage: source}), p)
	if r.Status != Fail || len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-003" {
		t.Fatalf("wide interface methods were erased: %+v", r)
	}
}
