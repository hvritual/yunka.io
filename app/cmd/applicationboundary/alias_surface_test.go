package applicationboundary

import "testing"

func TestAddressableExportedUnnamedAliases(t *testing.T) {
	const base = `package engine
type Reader interface{Read()}
type hidden struct{}
func(hidden)Read(){}
func(*hidden)Delete(){}
`
	for _, tc := range []struct{ name, body, status, rule string }{
		{"public_struct_alias", `type Public = struct{hidden};func Build()Reader{return Public{}}`, Fail, "AG-TYPE-003"},
		{"public_alias_chain", `type private = struct{hidden};type Public = private;func Build()Reader{return private{}}`, Fail, "AG-TYPE-003"},
		{"literal_with_public_alias", `type Public = struct{hidden};func Build()Reader{return struct{hidden}{}}`, Fail, "AG-TYPE-003"},
		{"private_struct_alias", `type private = struct{hidden};func Build()Reader{return private{}}`, Pass, ""},
		{"private_literal", `func Build()Reader{return struct{hidden}{}}`, Pass, ""},
		{"pointer_alias", `type Public = *struct{hidden};func Build()Reader{return Public(&struct{hidden}{})}`, Fail, "AG-TYPE-003"},
		{"unrelated_generic_alias", `type Public[T any] = []T;func Build()Reader{return struct{hidden}{}}`, Pass, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: base + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("alias surface: got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.rule != "" && (len(r.Findings) != 1 || r.Findings[0].Rule != tc.rule) {
				t.Fatalf("wrong alias diagnosis: %+v", r.Findings)
			}
		})
	}
}

func TestGenericAliasNameabilityDoesNotInventProof(t *testing.T) {
	const base = `package engine
type Reader interface{Read()}
type hidden[T any] struct{}
func(hidden[T])Read(){}
func(*hidden[T])Delete(){}
`
	for _, tc := range []struct{ name, body, status, rule string }{
		{"explicit_generic_struct_alias", `type Public[T any] = struct{hidden[T]};func Build()Reader{return Public[int]{}}`, Fail, "AG-TYPE-003"},
		{"unproven_generic_struct_spelling", `type Public[T any] = struct{hidden[T]};func Build()Reader{return struct{hidden[int]}{}}`, Incomplete, "AG-TYPE-000"},
		{"unproven_generic_named_spelling", `type Public[T any] = hidden[T];func Build()Reader{return hidden[int]{}}`, Incomplete, "AG-TYPE-000"},
		{"private_generic_value", `func Build()Reader{return hidden[int]{}}`, Pass, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Analyze(programFor(t, map[string]string{testPackage: base + tc.body}), policyFor("Build"))
			if r.Status != tc.status {
				t.Fatalf("generic alias: got %s want %s: %+v", r.Status, tc.status, r.Findings)
			}
			if tc.rule != "" && (len(r.Findings) != 1 || r.Findings[0].Rule != tc.rule) {
				t.Fatalf("wrong generic-alias diagnosis: %+v", r.Findings)
			}
		})
	}
}
