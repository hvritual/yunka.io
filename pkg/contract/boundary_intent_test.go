package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	contractdsl "github.com/hvritual/yunka.io/pkg/contractdsl/v1"
	"google.golang.org/protobuf/proto"
)

func TestIssue161BoundaryIntentRealProtocRoundTrip(t *testing.T) {
	root := t.TempDir()
	source := `syntax="proto3"; package sales.v1;
 import "yunka/dsl/v1/options.proto";
 option go_package="example.com/sales/contracts/v1;salesv1";
 option (yunka.dsl.v1.domain)={name:"sales" version:"v1"};
 message Request { option (yunka.dsl.v1.dto)={kind:DTO_INPUT}; string id=1; }
 message Reply { option (yunka.dsl.v1.dto)={kind:DTO_OUTPUT}; string id=1; }
 service Orders {
  option (yunka.dsl.v1.application)={name:"orders" operations:{id:"sales.inspect" use_case:"inspect" public:true request_type:"sales.v1.Request" response_type:"sales.v1.Reply" application_method:"Inspect" boundary:{context:"sales.reporting" aggregate_not_applicable_reason:"Read-only cross-aggregate projection"}}};
  rpc Get(Request) returns(Reply) { option (yunka.dsl.v1.operation)={id:"sales.get" use_case:"get" public:true boundary:{context:"sales.orders" aggregate:"order"}}; }
 }`
	writeProtoFixture(t, filepath.Join(root, "orders.proto"), source)
	support, err := filepath.Abs(filepath.Join("..", "..", "contracts", "proto"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := Compile(context.Background(), CompileOptions{Dir: root, Protoc: testProtoc(t), ProtoPaths: []string{support}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.SchemaVersion != 5 {
		t.Fatal(result.Manifest.SchemaVersion)
	}
	if HasErrors(Lint(result.Manifest)) {
		t.Fatal(Lint(result.Manifest))
	}
	service := result.Manifest.Services[0]
	if !reflect.DeepEqual(service.Methods[0].Operation.Boundary, &BoundaryIntent{Context: "sales.orders", Aggregate: "order"}) {
		t.Fatalf("RPC intent lost: %#v", service.Methods[0].Operation)
	}
	internal := service.Application.Operations[0].Boundary
	if internal == nil || internal.Context != "sales.reporting" || internal.AggregateNotApplicableReason == "" {
		t.Fatalf("internal intent lost: %#v", internal)
	}
	artifacts, err := RenderArtifacts(result.Manifest, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(artifacts.Manifest, []byte(`"boundary"`)) {
		t.Fatal("manifest lost intent")
	}
	output := filepath.Join(root, "generated")
	if err := WriteArtifacts(output, artifacts); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadManifest(filepath.Join(output, ManifestFilename))
	if err != nil {
		t.Fatal(err)
	}
	again, err := RenderArtifacts(loaded, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again.Manifest, artifacts.Manifest) {
		t.Fatal("manifest round trip drift")
	}
	// Removing architecture-only intent must not change transport/execution code.
	loaded.Services[0].Methods[0].Operation.Boundary = nil
	loaded.Services[0].Application.Operations[0].Boundary = nil
	legacy, err := RenderArtifacts(loaded, ArtifactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(legacy.OperationPlans, artifacts.OperationPlans) || !bytes.Equal(legacy.OpenAPI, artifacts.OpenAPI) || !bytes.Equal(legacy.TypeScript, artifacts.TypeScript) {
		t.Fatal("architectural intent changed execution/API artifacts")
	}
	beforeCode, err := RenderC9ApplicationCode(result.Manifest, ApplicationCodeOptions{RootImport: "example.com/sales/internal"})
	if err != nil {
		t.Fatal(err)
	}
	afterCode, err := RenderC9ApplicationCode(loaded, ApplicationCodeOptions{RootImport: "example.com/sales/internal"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeCode, afterCode) {
		t.Fatal("boundary intent altered generated runtime behavior")
	}
}

func TestIssue161BoundaryIntentGeneratedWireAndValidation(t *testing.T) {
	// Exercises the canonical generated descriptor as well as the independent
	// descriptor-wire parser; a forgotten rpc-generate cannot pass this test.
	generated := &contractdsl.OperationDeclaration{Id: "sales.get", Boundary: &contractdsl.BoundaryIntent{Context: "sales.orders", Aggregate: "order"}}
	data, err := proto.Marshal(generated)
	if err != nil {
		t.Fatal(err)
	}
	value, err := parseOperationDeclaration(data)
	if err != nil {
		t.Fatal(err)
	}
	if value.Boundary == nil || value.Boundary.Context != "sales.orders" || value.Boundary.Aggregate != "order" {
		t.Fatal(value)
	}
	b1 := appendWireBytes(nil, 1, []byte("sales.orders"))
	b2 := appendWireBytes(nil, 2, []byte("order"))
	chunks := appendWireBytes(appendWireBytes(nil, 14, b1), 14, b2)
	merged, err := parseOperationDeclaration(chunks)
	if err != nil || !reflect.DeepEqual(merged.Boundary, value.Boundary) {
		t.Fatalf("singular message merge lost: %#v %v", merged, err)
	}
	bad := []*BoundaryIntent{
		{}, {Context: " "}, {Context: "Upper", Aggregate: "order"}, {Context: "sales/orders", Aggregate: "order"},
		{Context: "sales.orders"}, {Context: "sales.orders", Aggregate: "order", AggregateNotApplicableReason: "both"},
		{Context: "sales.orders", Aggregate: "*"}, {Context: "sales.orders", AggregateNotApplicableReason: " "},
	}
	for _, intent := range bad {
		if ValidateBoundaryIntent(intent) == nil {
			t.Fatalf("accepted %#v", intent)
		}
	}
	if err := ValidateBoundaryIntent(nil); err != nil {
		t.Fatalf("legacy missing intent rejected: %v", err)
	}
	if err := ValidateBoundaryIntent(&BoundaryIntent{Context: "sales.reporting", AggregateNotApplicableReason: "cross-aggregate projection"}); err != nil {
		t.Fatal(err)
	}
	for _, data := range [][]byte{appendWireBytes(nil, 14, nil), {112, 1}, appendWireBytes(nil, 14, []byte{8, 1}), appendWireBytes(nil, 14, appendWireBytes(nil, 4, []byte("unknown")))} {
		if _, err := parseOperationDeclaration(data); err == nil {
			t.Fatalf("accepted invalid wire %x", data)
		}
	}
}

func TestIssue161BoundaryIntentLegacyAndInvalidManifest(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "manifest.json")
	for version := 1; version <= ManifestVersion; version++ {
		data, _ := json.Marshal(Manifest{SchemaVersion: version})
		if err := os.WriteFile(p, data, 0600); err != nil {
			t.Fatal(err)
		}
		got, err := LoadManifest(p)
		if err != nil || got.SchemaVersion != 5 {
			t.Fatalf("legacy schema %d: %#v %v", version, got, err)
		}
	}
	manifest := Manifest{SchemaVersion: 5, Services: []Service{{Domain: "sales", FullName: "sales.v1.Orders", Application: &ApplicationDeclaration{Name: "orders", Operations: []OperationDeclaration{{ID: "sales.inspect", UseCase: "inspect", RequestType: "google.protobuf.Empty", ResponseType: "google.protobuf.Empty", ApplicationMethod: "Inspect", Public: true, Boundary: &BoundaryIntent{Context: "sales.orders"}}}}}}}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(p); err == nil || !strings.Contains(err.Error(), "boundary intent") {
		t.Fatalf("invalid load=%v", err)
	}
	if _, err := CompileOperationPlans(manifest); err == nil {
		t.Fatal("programmatic compilation ignored malformed intent")
	}
	if !HasErrors(Lint(manifest)) {
		t.Fatal("lint ignored malformed intent")
	}
	operation := OperationDeclaration{Boundary: &BoundaryIntent{Context: " sales.orders ", Aggregate: " order "}}
	clone := cloneOperationDeclaration(operation)
	normalizeOperationDeclaration(&clone)
	clone.Boundary.Context = "changed"
	if operation.Boundary.Context != " sales.orders " {
		t.Fatal("normalization aliases caller intent")
	}
}
