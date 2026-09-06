package contract

import (
	"reflect"
	"testing"
)

func TestResolveOperationContractContextClosesOverDTOsAndImports(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files: []File{
			{Name: "delivery/v1/service.proto", Dependencies: []string{"delivery/v1/work_item.proto", "delivery/v1/common.proto"}},
			{Name: "delivery/v1/work_item.proto", Dependencies: []string{"delivery/v1/common.proto"}},
			{Name: "delivery/v1/common.proto"},
			{Name: "delivery/v1/unrelated.proto"},
		},
		Messages: []Message{
			{Name: "UpdateItemRequest", FullName: "delivery.v1.UpdateItemRequest", SourceFile: "delivery/v1/work_item.proto", Fields: []Field{{Name: "item", Kind: "message", Type: "delivery.v1.WorkItem"}}},
			{Name: "UpdateItemResponse", FullName: "delivery.v1.UpdateItemResponse", SourceFile: "delivery/v1/work_item.proto"},
			{Name: "WorkItem", FullName: "delivery.v1.WorkItem", SourceFile: "delivery/v1/work_item.proto", Fields: []Field{{Name: "metadata", Kind: "message", Type: "delivery.v1.Metadata"}}},
			{Name: "Metadata", FullName: "delivery.v1.Metadata", SourceFile: "delivery/v1/common.proto"},
			{Name: "Noise", FullName: "delivery.v1.Noise", SourceFile: "delivery/v1/unrelated.proto"},
		},
		Services: []Service{{
			Name: "DeliveryService", FullName: "delivery.v1.DeliveryService", SourceFile: "delivery/v1/service.proto",
			Methods: []Method{{
				Name: "UpdateItem", FullName: "delivery.v1.DeliveryService.UpdateItem", SourceFile: "delivery/v1/service.proto",
				Request: "delivery.v1.UpdateItemRequest", Response: "delivery.v1.UpdateItemResponse",
				Operation: &OperationDeclaration{ID: "delivery.items.update"},
			}},
		}},
	}

	context, err := ResolveOperationContractContext(manifest, "delivery.items.update")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"delivery/v1/common.proto", "delivery/v1/service.proto", "delivery/v1/work_item.proto"}
	if !reflect.DeepEqual(context.SourceFiles, want) {
		t.Fatalf("source files = %#v, want %#v", context.SourceFiles, want)
	}
}

func TestResolveOperationContractContextSurvivesFileMoveByProvenance(t *testing.T) {
	manifest := Manifest{
		SchemaVersion: ManifestVersion,
		Files:         []File{{Name: "delivery/v1/service.proto"}, {Name: "delivery/v1/model/work_item.proto"}},
		Messages:      []Message{{Name: "Request", FullName: "delivery.v1.Request", SourceFile: "delivery/v1/model/work_item.proto"}, {Name: "Response", FullName: "delivery.v1.Response", SourceFile: "delivery/v1/model/work_item.proto"}},
		Services:      []Service{{Name: "DeliveryService", FullName: "delivery.v1.DeliveryService", SourceFile: "delivery/v1/service.proto", Methods: []Method{{Name: "Update", SourceFile: "delivery/v1/service.proto", Request: "delivery.v1.Request", Response: "delivery.v1.Response", Operation: &OperationDeclaration{ID: "delivery.items.update"}}}}},
	}
	context, err := ResolveOperationContractContext(manifest, "delivery.items.update")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"delivery/v1/model/work_item.proto", "delivery/v1/service.proto"}
	if !reflect.DeepEqual(context.SourceFiles, want) {
		t.Fatalf("source files = %#v, want %#v", context.SourceFiles, want)
	}
}

func TestIssue160ContextRejectsMissingOrInvalidProvenance(t *testing.T) {
	for _, source := range []string{"", "missing.proto", "../outside.proto"} {
		manifest := Manifest{Files: []File{{Name: "service.proto"}}, Services: []Service{{FullName: "example.API", SourceFile: source, Methods: []Method{{Name: "Echo", Operation: &OperationDeclaration{ID: "example.echo"}}}}}}
		if _, err := ResolveOperationContractContext(manifest, "example.echo"); err == nil {
			t.Fatalf("accepted source=%q", source)
		}
	}
}

func TestIssue160InternalOperationContext(t *testing.T) {
	manifest := Manifest{Files: []File{{Name: "service.proto"}, {Name: "dto.proto"}}, Messages: []Message{{FullName: "example.Request", SourceFile: "dto.proto"}, {FullName: "example.Response", SourceFile: "dto.proto"}}, Services: []Service{{FullName: "example.API", SourceFile: "service.proto", Application: &ApplicationDeclaration{Name: "local", Operations: []OperationDeclaration{{ID: "example.internal", RequestType: "example.Request", ResponseType: "example.Response"}}}}}}
	values, err := OperationContractContexts(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || !reflect.DeepEqual(values[0].SourceFiles, []string{"dto.proto", "service.proto"}) {
		t.Fatalf("internal contexts=%#v", values)
	}
}

func TestIssue160DeclarationFilesDoNotFollowUnrelatedServiceImports(t *testing.T) {
	manifest := Manifest{
		Files:    []File{{Name: "service.proto", Dependencies: []string{"dto.proto", "other.proto"}}, {Name: "dto.proto", Dependencies: []string{"common.proto"}}, {Name: "common.proto"}, {Name: "other.proto"}},
		Messages: []Message{{FullName: "x.Input", SourceFile: "dto.proto", Fields: []Field{{Kind: "message", Type: "x.Shared"}}}, {FullName: "x.Output", SourceFile: "dto.proto"}, {FullName: "x.Shared", SourceFile: "common.proto", Fields: []Field{{Kind: "enum", Type: "x.State"}}}, {FullName: "x.Other", SourceFile: "other.proto"}},
		Enums:    []Enum{{FullName: "x.State", SourceFile: "common.proto"}},
		Services: []Service{{FullName: "x.API", SourceFile: "service.proto", Methods: []Method{{Operation: &OperationDeclaration{ID: "x.one"}, Request: "x.Input", Response: "x.Output"}}}},
	}
	value, err := ResolveOperationContractContext(manifest, "x.one")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(value.DeclarationFiles, []string{"common.proto", "dto.proto", "service.proto"}) {
		t.Fatalf("declarations=%v", value.DeclarationFiles)
	}
	if !reflect.DeepEqual(value.SourceFiles, []string{"common.proto", "dto.proto", "other.proto", "service.proto"}) {
		t.Fatalf("imports=%v", value.SourceFiles)
	}
	if !reflect.DeepEqual(value.MessageTypes, []string{"x.Input", "x.Output", "x.Shared"}) || !reflect.DeepEqual(value.EnumTypes, []string{"x.State"}) {
		t.Fatalf("type graph=%#v", value)
	}
}
