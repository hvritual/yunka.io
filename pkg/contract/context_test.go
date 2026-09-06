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
		Files: []File{{Name: "delivery/v1/service.proto"}, {Name: "delivery/v1/model/work_item.proto"}},
		Messages: []Message{{Name: "Request", FullName: "delivery.v1.Request", SourceFile: "delivery/v1/model/work_item.proto"}, {Name: "Response", FullName: "delivery.v1.Response", SourceFile: "delivery/v1/model/work_item.proto"}},
		Services: []Service{{Name: "DeliveryService", FullName: "delivery.v1.DeliveryService", SourceFile: "delivery/v1/service.proto", Methods: []Method{{Name: "Update", SourceFile: "delivery/v1/service.proto", Request: "delivery.v1.Request", Response: "delivery.v1.Response", Operation: &OperationDeclaration{ID: "delivery.items.update"}}}}},
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
