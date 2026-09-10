package projectflow

import "testing"

func TestAG062ImplementationLayoutIsCanonicalAndShared(t *testing.T) {
	project := ProjectDescriptor{GeneratedGoRoot: "internal"}
	layout, err := DescribeImplementationLayout(project, "sales", "order.v2", "ListOrders")
	if err != nil {
		t.Fatal(err)
	}
	if layout.Root != "internal/sales/application/order.v2" || layout.Build != layout.Root+"/build.go" || layout.TypePolicy != layout.Root+"/architecture.types.json" || layout.Handler != layout.Root+"/internal/usecase/listorders_handler.go" {
		t.Fatalf("layout=%#v", layout)
	}
	for _, bad := range [][2]string{{"../sales", "orders"}, {"sales", "internal"}, {"sales/a", "orders"}} {
		if _, err := DescribeImplementationLayout(project, bad[0], bad[1], "List"); err == nil {
			t.Fatalf("invalid layout accepted: %#v", bad)
		}
	}
	if _, err := ImplementationHandlerFilename("Bad/Method"); err == nil {
		t.Fatal("invalid handler method accepted")
	}
}
