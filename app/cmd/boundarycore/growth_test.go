package boundarycore

import (
	"testing"

	"github.com/hvritual/yunka.io/pkg/contract"
)

func TestEvaluateGrowthRejectsContextChangeAndUnknownAddition(t *testing.T) {
	base := growthManifest(&contract.BoundaryIntent{Context: "sales.orders", Aggregate: "order"})
	current := detachedManifest(base)
	current.Services[0].Methods[0].Operation.Boundary.Context = "billing.invoices"
	current.Services[0].Methods = append(current.Services[0].Methods, contract.Method{
		Name: "List", FullName: "sales.v1.Orders.List", SourceFile: "sales.proto", Request: "sales.v1.ReadRequest", Response: "sales.v1.ReadResponse",
		Operation: &contract.OperationDeclaration{ID: "orders.list", UseCase: "list_orders", Public: true, PermissionMode: "all", Execution: &contract.ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}},
	})
	events := EvaluateGrowth("0123456789012345678901234567890123456789", base, current)
	foundContext, foundAddition := false, false
	for _, event := range events {
		if event.Kind == GrowthContextChanged && event.Outcome == CreateNewService {
			foundContext = true
		}
		if event.Kind == GrowthOperationAdded && event.Outcome == ArchitectureReviewRequired {
			foundAddition = true
		}
	}
	if !foundContext || !foundAddition {
		t.Fatalf("events=%+v", events)
	}
}

func growthManifest(boundary *contract.BoundaryIntent) contract.Manifest {
	return contract.Manifest{SchemaVersion: contract.ManifestVersion,
		Files: []contract.File{{Name: "sales.proto", Package: "sales.v1", Domain: &contract.DomainDeclaration{Name: "sales"}}},
		Messages: []contract.Message{
			{Name: "ReadRequest", FullName: "sales.v1.ReadRequest", SourceFile: "sales.proto", DTO: &contract.DTODeclaration{Kind: "input"}, Fields: []contract.Field{}},
			{Name: "ReadResponse", FullName: "sales.v1.ReadResponse", SourceFile: "sales.proto", DTO: &contract.DTODeclaration{Kind: "output"}, Fields: []contract.Field{}},
		},
		Services: []contract.Service{{Name: "Orders", FullName: "sales.v1.Orders", SourceFile: "sales.proto", Domain: "sales", Application: &contract.ApplicationDeclaration{Name: "orders"}, Methods: []contract.Method{{
			Name: "Read", FullName: "sales.v1.Orders.Read", SourceFile: "sales.proto", Request: "sales.v1.ReadRequest", Response: "sales.v1.ReadResponse",
			Operation: &contract.OperationDeclaration{ID: "orders.read", UseCase: "read_order", Public: true, PermissionMode: "all", Execution: &contract.ExecutionPolicy{Transaction: "read_only", Idempotency: "none"}, Boundary: boundary},
		}}}},
	}
}
