package applicationboundary

import "testing"

func TestUnrelatedGenericStructAliasesDoNotBlockPrivateValues(t *testing.T) {
	const base = `package engine
type Reader interface{Read()}
type hidden struct{}
func(hidden)Read(){}
func(*hidden)Delete(){}
`
	cases := []struct{ name, body, status string }{
		{"different_field_name", `type Public[T any]=struct{Value T};func Build()Reader{return struct{hidden}{}}`, Pass},
		{"different_arity", `type Public[T any]=struct{hidden;value T};func Build()Reader{return struct{hidden}{}}`, Pass},
		{"different_embedding", `type Public[T any]=struct{hidden hidden;value T};func Build()Reader{return struct{hidden;value int}{}}`, Pass},
		{"different_tags", "type Public[T any]=struct{hidden `json:\"left\"`;value T};func Build()Reader{return struct{hidden `json:\"right\"`;value int}{}}", Pass},
		{"inconsistent_repeated_argument", `type Public[T any]=struct{hidden;left T;right T};func Build()Reader{return struct{hidden;left int;right string}{}}`, Pass},
		{"different_array_length", `type Public[T any]=struct{hidden;value [2]T};func Build()Reader{return struct{hidden;value [3]int}{}}`, Pass},
		{"different_container", `type Public[T any]=struct{hidden;value []T};func Build()Reader{return struct{hidden;value map[int]string}{}}`, Pass},
		{"different_function_arity", `type Public[T any]=struct{hidden;value func(T)};func Build()Reader{return struct{hidden;value func(int,int)}{}}`, Pass},
		{"different_interface_method", `type Public[T any]=struct{hidden;value interface{Left(T)}};func Build()Reader{return struct{hidden;value interface{Right(int)}}{}}`, Pass},
		{"possible_repeated_argument", `type Public[T any]=struct{hidden;left T;right T};func Build()Reader{return struct{hidden;left int;right int}{}}`, Incomplete},
		{"possible_unused_argument", `type Public[T,E any]=struct{hidden;value T};func Build()Reader{return struct{hidden;value int}{}}`, Incomplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: base + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("generic structure: got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.status == Pass && len(r.Findings) != 0 {
				t.Fatalf("unrelated alias generated a finding: %+v", r.Findings)
			}
			if tc.status == Incomplete && (len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-000") {
				t.Fatalf("possible alias silently passed or was invented as a violation: %+v", r.Findings)
			}
		})
	}
}

func TestGenericAliasConstraintsDoNotInventNameability(t *testing.T) {
	const base = `package engine
type Reader interface{Read()}
type hidden[T any] struct{}
func(hidden[T])Read(){}
func(*hidden[T])Delete(){}
`
	cases := []struct{ name, body, status string }{
		{"impossible_constraint", `type Public[T ~int]=struct{hidden[T]};func Build()Reader{return struct{hidden[string]}{}}`, Pass},
		{"possible_constraint", `type Public[T ~int]=struct{hidden[T]};func Build()Reader{return struct{hidden[int]}{}}`, Incomplete},
		{"invalid_known_argument_with_unused_parameter", `type Public[T ~int,E any]=struct{hidden[T]};func Build()Reader{return struct{hidden[string]}{}}`, Pass},
		{"impossible_dependent_constraint", `type Public[E any,T ~[]E]=struct{hidden[T];value E};func Build()Reader{return struct{hidden[[]string];value int}{}}`, Pass},
		{"possible_dependent_constraint", `type Public[E any,T ~[]E]=struct{hidden[T];value E};func Build()Reader{return struct{hidden[[]int];value int}{}}`, Incomplete},
		{"unused_dependent_constraint", `type Public[E any,T ~[]E]=struct{hidden[T]};func Build()Reader{return struct{hidden[[]int]}{}}`, Incomplete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: base + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("generic constraint: got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.status == Pass && len(r.Findings) != 0 {
				t.Fatalf("impossible instantiation generated a finding: %+v", r.Findings)
			}
			if tc.status == Incomplete && (len(r.Findings) != 1 || r.Findings[0].Rule != "AG-TYPE-000") {
				t.Fatalf("wrong unresolved-instantiation diagnosis: %+v", r.Findings)
			}
		})
	}
}
