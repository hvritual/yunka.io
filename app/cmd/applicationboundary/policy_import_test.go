package applicationboundary

import "testing"

func TestImportPolicyUsesGoPathRules(t *testing.T) {
	for _, path := range []string{"example.com/ok_pkg", "example.com/ok-pkg/v2", "std", "example.com/t~est"} {
		if !importIdentity(path) {
			t.Errorf("valid Go import path rejected: %q", path)
		}
	}
	for _, path := range []string{"example.com/bad?pkg", "example.com/a@v1", "example.com/a,b", "example.com/a%b", "example.com/a#b", "example.com/con", "example.com/x.", "example.com/..", "../local", "/root", "example.com//x", "example.com/x~1", "example.com/路径"} {
		t.Run(path, func(t *testing.T) {
			if importIdentity(path) {
				t.Fatalf("invalid import path accepted: %q", path)
			}
			for _, mutate := range []func(*Policy){
				func(p *Policy) { p.Factories[0].AllowedCallers = []string{path} },
				func(p *Policy) { p.Factories[0].Symbol.Package = path },
				func(p *Policy) { p.Factories[0].Results[0].Contract.Package = path },
			} {
				p := policyFor("Build")
				mutate(&p)
				if err := p.Validate(); err == nil {
					t.Fatal("malformed package identity was accepted")
				}
			}
		})
	}
}
