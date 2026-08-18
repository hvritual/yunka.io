package mdns

import (
	"testing"

	"yunka.io/pkg/registry"
)

func TestDiffMDNSSnapshots(t *testing.T) {
	service := func(address string) *registry.Service {
		return &registry.Service{
			Name:    "coffee",
			Version: "v1",
			Nodes: []*registry.Node{{
				Id:      "machine-1",
				Address: address,
			}},
		}
	}
	key := "coffee\x00v1\x00machine-1"

	tests := []struct {
		name    string
		before  mdnsSnapshot
		after   mdnsSnapshot
		action  string
		wantLen int
	}{
		{name: "create", before: mdnsSnapshot{}, after: mdnsSnapshot{key: service("10.0.0.1:80")}, action: "create", wantLen: 1},
		{name: "update", before: mdnsSnapshot{key: service("10.0.0.1:80")}, after: mdnsSnapshot{key: service("10.0.0.2:80")}, action: "update", wantLen: 1},
		{name: "delete", before: mdnsSnapshot{key: service("10.0.0.1:80")}, after: mdnsSnapshot{}, action: "delete", wantLen: 1},
		{name: "unchanged", before: mdnsSnapshot{key: service("10.0.0.1:80")}, after: mdnsSnapshot{key: service("10.0.0.1:80")}, wantLen: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results := diffMDNSSnapshots(test.before, test.after)
			if len(results) != test.wantLen {
				t.Fatalf("len=%d want=%d", len(results), test.wantLen)
			}
			if test.wantLen > 0 && results[0].Action != test.action {
				t.Fatalf("action=%q want=%q", results[0].Action, test.action)
			}
		})
	}
}
