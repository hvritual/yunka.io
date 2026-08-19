package selector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"yunka.io/pkg/registry"
)

type fakeRegistry struct {
	services map[string][]*registry.Service
}

func (registry *fakeRegistry) GetService(name string) ([]*registry.Service, error) {
	services := registry.services[name]
	if len(services) == 0 {
		return nil, ErrNotFound
	}
	return services, nil
}

func testServices() []*registry.Service {
	return []*registry.Service{{
		Name: "svc", Version: "v1",
		Nodes: []*registry.Node{
			{Id: "a", Address: "10.0.0.1:1", Metadata: map[string]string{"region": "eu", "zone": "a"}},
			{Id: "b", Address: "10.0.0.2:1", Metadata: map[string]string{"region": "eu", "zone": "b"}},
		},
	}}
}

func adaptiveForTest(t *testing.T, config AdaptiveOptions) *registrySelector {
	t.Helper()
	config.Enabled = true
	if config.Mode == "" {
		config.Mode = AdaptiveP2C
	}
	selector := NewAdaptiveSelector(Registry(&fakeRegistry{services: map[string][]*registry.Service{"svc": testServices()}}), WithAdaptiveConfig(config))
	return selector.(*registrySelector)
}

func snapshotByID(t *testing.T, selector *registrySelector, id string) NodeSnapshot {
	t.Helper()
	for _, snapshot := range selector.Snapshot("svc") {
		if snapshot.NodeID == id {
			return snapshot
		}
	}
	t.Fatalf("snapshot %s not found", id)
	return NodeSnapshot{}
}

func TestP2CPrefersLowerEWMA(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 1, InitialLatency: 100 * time.Millisecond, EWMAAlpha: 1})
	// Populate selector state, then feed deterministic latency for each node.
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	selector.mu.Lock()
	for _, state := range selector.states["svc"] {
		if state.node.Id == "a" {
			state.ewma = float64(10 * time.Millisecond)
		}
		if state.node.Id == "b" {
			state.ewma = float64(200 * time.Millisecond)
		}
	}
	selector.mu.Unlock()

	next, err := selector.Select("svc", WithAdaptiveMode(AdaptiveP2C))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		node, err := next()
		if err != nil {
			t.Fatal(err)
		}
		if node.Id != "a" {
			t.Fatalf("P2C chose %s, want a", node.Id)
		}
	}
}

func TestLeastRequestUsesTrackedInflight(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 2})
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	selector.mu.Lock()
	for _, state := range selector.states["svc"] {
		if state.node.Id == "a" {
			state.inflight = 4
		}
		if state.node.Id == "b" {
			state.inflight = 1
		}
	}
	selector.mu.Unlock()
	selection, err := selector.Pick("svc", WithAdaptiveMode(AdaptiveLeastRequest))
	if err != nil {
		t.Fatal(err)
	}
	if selection.Node.Id != "b" {
		t.Fatalf("picked %s, want b", selection.Node.Id)
	}
	selection.DoneWithDuration(nil, 10*time.Millisecond, OutcomeSuccess)
	if got := snapshotByID(t, selector, "b").InFlight; got != 1 {
		t.Fatalf("inflight=%d want 1", got)
	}
}

func TestOutlierEjectionAndRecovery(t *testing.T) {
	now := time.Unix(100, 0)
	selector := adaptiveForTest(t, AdaptiveOptions{
		Seed:    3,
		Now:     func() time.Time { return now },
		Outlier: OutlierOptions{ConsecutiveFailures: 2, BaseEjectionTime: 10 * time.Second, MaxEjectionTime: time.Minute, MaxEjectionPercent: 50, FailClosed: false},
	})
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	failed := &registry.Node{Id: "a", Address: "10.0.0.1:1"}
	selector.Mark("svc", failed, errors.New("boom"))
	selector.Mark("svc", failed, errors.New("boom"))
	if !snapshotByID(t, selector, "a").Ejected {
		t.Fatal("node a should be ejected")
	}

	next, err := selector.Select("svc")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		node, _ := next()
		if node.Id == "a" {
			t.Fatal("ejected node selected")
		}
	}

	now = now.Add(11 * time.Second)
	next, err = selector.Select("svc")
	if err != nil {
		t.Fatal(err)
	}
	seenA := false
	for i := 0; i < 100; i++ {
		node, _ := next()
		if node.Id == "a" {
			seenA = true
			break
		}
	}
	if !seenA {
		t.Fatal("recovered node a never became selectable")
	}
}

func TestOutlierNeverEjectsSingleNode(t *testing.T) {
	services := []*registry.Service{{Name: "svc", Version: "v1", Nodes: []*registry.Node{{Id: "only", Address: "1"}}}}
	selector := NewAdaptiveSelector(Registry(&fakeRegistry{services: map[string][]*registry.Service{"svc": services}}), WithAdaptiveConfig(AdaptiveOptions{Enabled: true, Outlier: OutlierOptions{ConsecutiveFailures: 1, MaxEjectionPercent: 50}})).(*registrySelector)
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	selector.Mark("svc", services[0].Nodes[0], errors.New("boom"))
	if snapshotByIDFor(selector, "svc", "only").Ejected {
		t.Fatal("single node must not be ejected")
	}
}

func snapshotByIDFor(selector *registrySelector, service, id string) NodeSnapshot {
	for _, snapshot := range selector.Snapshot(service) {
		if snapshot.NodeID == id {
			return snapshot
		}
	}
	return NodeSnapshot{}
}

func TestSelectionDoneIsIdempotent(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 4, EWMAAlpha: 1})
	selection, err := selector.Pick("svc")
	if err != nil {
		t.Fatal(err)
	}
	selection.DoneWithDuration(errors.New("boom"), 20*time.Millisecond, OutcomeFailure)
	selection.DoneWithDuration(errors.New("boom"), 20*time.Millisecond, OutcomeFailure)
	snapshot := snapshotByID(t, selector, selection.Node.Id)
	if snapshot.InFlight != 0 {
		t.Fatalf("inflight=%d", snapshot.InFlight)
	}
	if snapshot.ConsecutiveFailures != 1 {
		t.Fatalf("failures=%d want 1", snapshot.ConsecutiveFailures)
	}
}

func TestLocalityFilters(t *testing.T) {
	services := testServices()
	region := FilterRegion("eu")(services)
	if len(region) != 1 || len(region[0].Nodes) != 2 {
		t.Fatalf("region filter=%v", region)
	}
	zone := FilterZone("a")(services)
	if len(zone) != 1 || len(zone[0].Nodes) != 1 || zone[0].Nodes[0].Id != "a" {
		t.Fatalf("zone filter=%v", zone)
	}
}

func TestConcurrentPickDoneAndSnapshot(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 11, EWMAAlpha: 0.3})
	const workers = 64
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				selection, err := selector.Pick("svc")
				if err != nil {
					t.Errorf("pick: %v", err)
					return
				}
				if (i+j)%17 == 0 {
					selection.DoneWithDuration(errors.New("transient"), time.Millisecond, OutcomeFailure)
				} else {
					selection.DoneWithDuration(nil, time.Millisecond, OutcomeSuccess)
				}
				_ = selector.Snapshot("svc")
			}
		}(i)
	}
	wg.Wait()
	for _, snapshot := range selector.Snapshot("svc") {
		if snapshot.InFlight != 0 {
			t.Fatalf("node %s inflight=%d", snapshot.NodeID, snapshot.InFlight)
		}
	}
}

func TestMaxEjectionPercentKeepsCapacity(t *testing.T) {
	services := []*registry.Service{{Name: "svc", Version: "v1", Nodes: []*registry.Node{
		{Id: "a", Address: "1"}, {Id: "b", Address: "2"}, {Id: "c", Address: "3"}, {Id: "d", Address: "4"},
	}}}
	selector := NewAdaptiveSelector(
		Registry(&fakeRegistry{services: map[string][]*registry.Service{"svc": services}}),
		WithAdaptiveConfig(AdaptiveOptions{Enabled: true, Seed: 12, Outlier: OutlierOptions{ConsecutiveFailures: 1, MaxEjectionPercent: 50}}),
	).(*registrySelector)
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	for _, node := range services[0].Nodes {
		selector.Mark("svc", node, errors.New("boom"))
	}
	ejected := 0
	for _, snapshot := range selector.Snapshot("svc") {
		if snapshot.Ejected {
			ejected++
		}
	}
	if ejected != 2 {
		t.Fatalf("ejected=%d want 2", ejected)
	}
}

func TestCanceledCallDoesNotPenalizeNode(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 13})
	selection, err := selector.Pick("svc")
	if err != nil {
		t.Fatal(err)
	}
	selection.DoneWithDuration(context.Canceled, time.Millisecond, OutcomeAuto)
	snapshot := snapshotByID(t, selector, selection.Node.Id)
	if snapshot.ConsecutiveFailures != 0 || snapshot.LastOutcome != OutcomeIgnore {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestNewSelectorKeepsLegacyMode(t *testing.T) {
	selector := NewSelector(Registry(&fakeRegistry{services: map[string][]*registry.Service{"svc": testServices()}}))
	if selector.Options().Adaptive.Enabled {
		t.Fatal("NewSelector must not enable adaptive selection")
	}
	if selector.String() != "registry" {
		t.Fatalf("selector string=%q", selector.String())
	}
}

func TestNewAdaptiveSelectorForcesAdaptiveMode(t *testing.T) {
	selector := NewAdaptiveSelector(
		Registry(&fakeRegistry{services: map[string][]*registry.Service{"svc": testServices()}}),
		WithAdaptiveConfig(AdaptiveOptions{Mode: AdaptiveEWMA}),
	)
	if !selector.Options().Adaptive.Enabled || selector.Options().Adaptive.Mode != AdaptiveEWMA {
		t.Fatalf("adaptive options=%+v", selector.Options().Adaptive)
	}
}

func TestEWMAPicksLowestScore(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 14, Mode: AdaptiveEWMA})
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	selector.mu.Lock()
	for _, state := range selector.states["svc"] {
		if state.node.Id == "a" {
			state.ewma = float64(80 * time.Millisecond)
		}
		if state.node.Id == "b" {
			state.ewma = float64(20 * time.Millisecond)
		}
	}
	selector.mu.Unlock()
	selection, err := selector.Pick("svc")
	if err != nil {
		t.Fatal(err)
	}
	if selection.Node.Id != "b" {
		t.Fatalf("picked=%s want b", selection.Node.Id)
	}
	selection.DoneWithDuration(nil, 20*time.Millisecond, OutcomeSuccess)
}

func TestRegistryReconcilePrunesRemovedNodes(t *testing.T) {
	fake := &fakeRegistry{services: map[string][]*registry.Service{"svc": testServices()}}
	selector := NewAdaptiveSelector(Registry(fake), WithAdaptiveConfig(AdaptiveOptions{Enabled: true, Seed: 15})).(*registrySelector)
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	if len(selector.Snapshot("svc")) != 2 {
		t.Fatalf("initial snapshot=%v", selector.Snapshot("svc"))
	}
	fake.services["svc"] = []*registry.Service{{Name: "svc", Version: "v1", Nodes: []*registry.Node{{Id: "a", Address: "10.0.0.1:1"}}}}
	if _, err := selector.Select("svc"); err != nil {
		t.Fatal(err)
	}
	if snapshot := selector.Snapshot("svc"); len(snapshot) != 1 || snapshot[0].NodeID != "a" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestSnapshotCountersAndScore(t *testing.T) {
	selector := adaptiveForTest(t, AdaptiveOptions{Seed: 16, EWMAAlpha: 1})
	selection, err := selector.Pick("svc")
	if err != nil {
		t.Fatal(err)
	}
	selection.DoneWithDuration(nil, 5*time.Millisecond, OutcomeSuccess)
	snapshot := snapshotByID(t, selector, selection.Node.Id)
	if snapshot.Selections != 1 || snapshot.Successes != 1 || snapshot.Failures != 0 || snapshot.Ignored != 0 {
		t.Fatalf("snapshot counters=%+v", snapshot)
	}
	if snapshot.Score <= 0 {
		t.Fatalf("score=%f", snapshot.Score)
	}
}
