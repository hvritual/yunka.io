package applicationboundary

import "testing"

func TestPromotedOpaqueFields(t *testing.T) {
	for _, tc := range []struct{ name, body, status string }{
		{"promoted_exported_field", `type state struct{ Target *wide };func(*state)Read(){};type view struct{state};func Build()Reader{return &view{}}`, Fail},
		{"ambiguous_unreachable_field", `type left struct{Target *wide};type right struct{Target *wide};type view struct{left;right};func(*view)Read(){};func Build()Reader{return &view{}}`, Pass},
		{"private_state", `type state struct{ target *wide };func(*state)Read(){};type view struct{state};func Build()Reader{return &view{}}`, Pass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: definitions + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.status == Fail && (len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-004") {
				t.Fatalf("wrong visible-field diagnosis: %+v", r.Findings)
			}
		})
	}
}

func TestUnreachableReturnDoesNotInventCapability(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"constant_if", `func Build()Reader{if true{return &narrow{}};return &wide{}}`},
		{"both_arms", `func Build(ok bool)Reader{if ok{return &narrow{}}else{return &narrow{}};return &wide{}}`},
		{"nested_block", `func Build()Reader{{return &narrow{}};return &wide{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: definitions + tc.body}), policyFor("Build"))
			if r.Status != Pass {
				t.Fatalf("unreachable return counted as capability: %+v", r.Findings)
			}
		})
	}
}
