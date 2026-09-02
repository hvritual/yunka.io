package diagnostics

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	frameworkoperation "github.com/hvritual/yunka.io/framework/operation"
	"github.com/hvritual/yunka.io/pkg/operationplan"
)

func TestOperationSourceExposesSafePlanSummaryOnly(t *testing.T) {
	plans := operationplan.Set{SchemaVersion: operationplan.SchemaVersion, Operations: []operationplan.Plan{{
		OperationID: "device.get", Domain: "device", Application: "management", UseCase: "get_device",
		RequestType: "device.v1.GetDeviceRequest", ResponseType: "device.v1.DeviceDTO",
		Security:  operationplan.Security{TenantRequired: true, Authentication: []string{"jwt"}, Permissions: []string{"device.secret.read"}, PermissionMode: "all"},
		Execution: operationplan.Execution{Transaction: "read_only", Idempotency: "none"},
		Bindings:  operationplan.Bindings{RPC: "/device.v1.DeviceApplication/GetDevice"},
	}}}
	source, err := NewOperationSource(plans, frameworkoperation.NewExecutor(nil))
	if err != nil {
		t.Fatal(err)
	}
	value, err := source.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, `"operationId":"device.get"`) || !strings.Contains(text, `"digest":"`) || !strings.Contains(text, `"transaction":"read_only"`) {
		t.Fatalf("payload=%s", text)
	}
	for _, forbidden := range []string{"device.secret.read", "jwt", "principal", "tenant-a", "requestType", "responseType"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("diagnostics leaked %q: %s", forbidden, text)
		}
	}
}
