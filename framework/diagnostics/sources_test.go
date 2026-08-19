package diagnostics

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"yunka.io/pkg/contract"
	"yunka.io/pkg/selector"
)

type fakeSelectorSnapshotter struct{}

func (fakeSelectorSnapshotter) Snapshot(service string) []selector.NodeSnapshot {
	return []selector.NodeSnapshot{{
		Service: service, NodeID: "n1", Address: "10.0.0.1:9000",
		EWMA: 12 * time.Millisecond, Score: 12, Selections: 3, Successes: 2, Failures: 1,
		LastOutcome: selector.OutcomeFailure,
	}}
}

func TestContractSourceSummarizesOperations(t *testing.T) {
	source := Contract(contract.Manifest{
		SchemaVersion: contract.ManifestVersion,
		Services: []contract.Service{{FullName: "demo.v1.Device", Methods: []contract.Method{{
			Name: "Get", FullName: "demo.v1.Device.Get", Request: "demo.GetRequest", Response: "demo.GetResponse",
			HTTP: []contract.HTTPBinding{{Method: "GET", Path: "/v1/devices/{id}"}},
		}}}},
	})
	value, err := source.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	snapshot := value.(ContractSnapshot)
	if snapshot.Services != 1 || snapshot.Methods != 1 || snapshot.HTTPBindings != 1 || snapshot.Operations[0].RPCPath != "/demo.v1.Device/Get" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}

func TestSelectorSourceProducesStableJSONShape(t *testing.T) {
	source := Selector("selector", fakeSelectorSnapshotter{}, "z", "a", "a")
	value, err := source.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot SelectorSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Services) != 2 || snapshot.Services[0].Name != "a" || snapshot.Services[0].Nodes[0].EWMAMillis != 12 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
}
