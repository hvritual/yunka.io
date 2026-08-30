package modulecatalog

import "testing"

func TestAssemblyInputsExposeDescriptorFactsWithoutRuntimeBuilders(t *testing.T) {
	plan := Plan{Descriptors: []Descriptor{{
		Name: "device", Version: "v1", DependsOn: []string{"site"},
		Requirements: Requirements{
			ConfigKey: "device", Logger: true, EventBus: true,
			Databases: []DatabaseRequirement{{Name: "primary"}},
			RPC:       []RPCRequirement{{Name: "inventory"}},
		},
	}, {Name: "site"}}}
	inputs := AssemblyInputs(plan)
	if len(inputs) != 2 {
		t.Fatalf("unexpected module input count: %d", len(inputs))
	}
	device := inputs[0]
	if device.Name != "device" || device.Version != "v1" || len(device.DependsOn) != 1 || device.DependsOn[0] != "site" {
		t.Fatalf("descriptor identity/dependency lost: %#v", device)
	}
	if device.Requirements.ConfigKey != "device" || !device.Requirements.Logger || !device.Requirements.EventBus || len(device.Requirements.Databases) != 1 || device.Requirements.Databases[0] != "primary" || len(device.Requirements.RPC) != 1 || device.Requirements.RPC[0] != "inventory" {
		t.Fatalf("descriptor requirements lost: %#v", device.Requirements)
	}
}
