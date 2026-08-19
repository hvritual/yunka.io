package resilience

import "testing"

func TestPeekSnapshotDoesNotCreatePolicyState(t *testing.T) {
	policy := NewRPCPolicy(RPCPolicyConfig{
		Circuit:   CircuitBreakerConfig{Enabled: true},
		RateLimit: RateLimitConfig{Enabled: true, Rate: 1, Burst: 1},
		LoadShed:  LoadShedConfig{Enabled: true, InitialLimit: 1, MaxLimit: 1},
	})
	if _, active := policy.PeekSnapshot("demo.Op"); active {
		t.Fatal("peek unexpectedly created or found policy state")
	}
	// Legacy Snapshot intentionally materializes configured state; once present,
	// PeekSnapshot must observe it without further mutation.
	policy.Snapshot("demo.Op")
	if _, active := policy.PeekSnapshot("demo.Op"); !active {
		t.Fatal("peek did not observe existing state")
	}
}
