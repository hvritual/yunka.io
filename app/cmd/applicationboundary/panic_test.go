package applicationboundary

import "testing"

func TestBuiltinPanicCannotProveReturnedCapability(t *testing.T) {
	for _, tc := range []struct{ name, body, status string }{
		{"named_result", `func Build()(r Reader){panic("stop");r=&narrow{};return}`, Incomplete},
		{"direct_return", `func Build()Reader{panic("stop");return &narrow{}}`, Incomplete},
		{"parenthesized_builtin", `func Build()Reader{(panic)("stop");return &narrow{}}`, Incomplete},
		{"if_initializer", `func Build()Reader{if panic("stop");true{return &narrow{}};return &narrow{}}`, Incomplete},
		{"shadowed_function", `func panic(string){};func Build()Reader{panic("not builtin");return &narrow{}}`, Pass},
		{"dead_panic_branch", `func Build()Reader{if false{panic("dead")};return &narrow{}}`, Pass},
		{"nonpanic_return_path", `func Build(ok bool)Reader{if !ok{panic("stop")};return &narrow{}}`, Pass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: definitions + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("panic termination: got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.status == Incomplete && (len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-000") {
				t.Fatalf("wrong no-return evidence diagnosis: %+v", r.Findings)
			}
		})
	}
}
