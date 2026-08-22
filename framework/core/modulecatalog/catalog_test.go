package modulecatalog

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"gorm.io/gorm"
)

type testInstance string

func (instance testInstance) Name() string { return string(instance) }

func descriptor(name string, dependencies ...string) Descriptor {
	return Descriptor{
		Name:      name,
		DependsOn: dependencies,
		Build: func(context BuildContext) (Instance, error) {
			return testInstance(context.Descriptor().Name), nil
		},
	}
}

func TestCatalogDeterministicOrderAcrossRegistrationOrder(t *testing.T) {
	orders := [][]Descriptor{
		{descriptor("api", "db"), descriptor("worker", "db"), descriptor("db")},
		{descriptor("db"), descriptor("worker", "db"), descriptor("api", "db")},
		{descriptor("worker", "db"), descriptor("api", "db"), descriptor("db")},
	}
	for _, registration := range orders {
		catalog := New()
		for _, current := range registration {
			if err := catalog.Register(current); err != nil {
				t.Fatal(err)
			}
		}
		plan, err := catalog.Seal()
		if err != nil {
			t.Fatal(err)
		}
		if got, want := plan.Names(), []string{"db", "api", "worker"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("order=%v want=%v", got, want)
		}
	}
}

func TestCatalogRejectsDuplicateMissingDependencyCycleAndLateRegistration(t *testing.T) {
	catalog := New()
	if err := catalog.Register(descriptor("api")); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Register(descriptor("api")); err == nil {
		t.Fatal("duplicate module accepted")
	}
	if _, err := catalog.Seal(); err != nil {
		t.Fatal(err)
	}
	if err := catalog.Register(descriptor("late")); err == nil {
		t.Fatal("late registration accepted")
	}

	missing := New()
	if err := missing.Register(descriptor("api", "db")); err != nil {
		t.Fatal(err)
	}
	if _, err := missing.Seal(); err == nil {
		t.Fatal("missing dependency accepted")
	}

	cycle := New()
	for _, current := range []Descriptor{descriptor("a", "b"), descriptor("b", "a")} {
		if err := cycle.Register(current); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := cycle.Seal(); err == nil {
		t.Fatal("cycle accepted")
	}
}

func TestCatalogCopiesDescriptorsAndPlans(t *testing.T) {
	dependencies := []string{"db"}
	catalog := New()
	if err := catalog.Register(descriptor("db")); err != nil {
		t.Fatal(err)
	}
	api := descriptor("api", dependencies...)
	if err := catalog.Register(api); err != nil {
		t.Fatal(err)
	}
	dependencies[0] = "mutated"
	api.DependsOn[0] = "mutated"
	plan, err := catalog.Seal()
	if err != nil {
		t.Fatal(err)
	}
	plan.Descriptors[1].DependsOn[0] = "changed-after-seal"
	again, err := catalog.Seal()
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Descriptors[1].DependsOn[0]; got != "db" {
		t.Fatalf("catalog mutation leaked: %q", got)
	}
}

func TestPlanAggregatesRequirementsDeterministically(t *testing.T) {
	catalog := New()
	for _, current := range []Descriptor{
		{Name: "api", DependsOn: []string{"db"}, Requirements: Requirements{ConfigKey: "modules.api", Logger: true, Databases: []DatabaseRequirement{{Name: "primary"}}, RPC: []RPCRequirement{{Name: "gateway"}}}, Build: descriptor("api").Build},
		{Name: "db", Requirements: Requirements{ConfigKey: "modules.db", Databases: []DatabaseRequirement{{Name: "primary"}}, EventBus: true}, Build: descriptor("db").Build},
	} {
		if err := catalog.Register(current); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := catalog.Seal()
	if err != nil {
		t.Fatal(err)
	}
	got := plan.Requirements()
	want := RequirementSet{
		Modules: []string{"db", "api"},
		Configs: []ConfigRequirement{{Module: "api", Key: "modules.api"}, {Module: "db", Key: "modules.db"}},
		Logger:  true, Databases: []DatabaseRequirement{{Name: "primary"}}, EventBus: true,
		RPC: []RPCRequirement{{Name: "gateway"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("requirements=%#v want=%#v", got, want)
	}
}

type countingDatabases struct {
	mu    sync.Mutex
	calls map[string]int
}

func (provider *countingDatabases) GORM(name string) (*gorm.DB, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.calls[name]++
	return &gorm.DB{}, nil
}

type fakeConnection struct{}

func (fakeConnection) Invoke(context.Context, string, any, any, ...grpc.CallOption) error { return nil }
func (fakeConnection) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

type countingRPC struct {
	mu    sync.Mutex
	calls map[string]int
}

func (provider *countingRPC) Connection(name string) (grpc.ClientConnInterface, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.calls[name]++
	return fakeConnection{}, nil
}

func TestStaticContextFactoryPreparesNamedCapabilitiesOnce(t *testing.T) {
	databases := &countingDatabases{calls: map[string]int{}}
	rpc := &countingRPC{calls: map[string]int{}}
	factory := NewStaticContextFactory(Capabilities{Databases: databases, RPC: rpc})
	requirements := RequirementSet{
		Modules:   []string{"orders"},
		Databases: []DatabaseRequirement{{Name: "primary"}},
		RPC:       []RPCRequirement{{Name: "gateway"}},
	}
	if err := factory.Prepare(requirements); err != nil {
		t.Fatal(err)
	}
	if err := factory.Prepare(requirements); err != nil {
		t.Fatal(err)
	}
	descriptor := Descriptor{
		Name:         "orders",
		Requirements: Requirements{Databases: []DatabaseRequirement{{Name: "primary"}}, RPC: []RPCRequirement{{Name: "gateway"}}},
		Build:        func(BuildContext) (Instance, error) { return testInstance("orders"), nil },
	}
	context, err := factory.ForModule(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := context.Databases().GORM("primary"); err != nil {
		t.Fatal(err)
	}
	if _, err := context.RPC().Connection("gateway"); err != nil {
		t.Fatal(err)
	}
	if databases.calls["primary"] != 1 || rpc.calls["gateway"] != 1 {
		t.Fatalf("database calls=%v RPC calls=%v", databases.calls, rpc.calls)
	}
	changed := requirements
	changed.RPC = []RPCRequirement{{Name: "other"}}
	if err := factory.Prepare(changed); err == nil {
		t.Fatal("factory accepted different requirement set after preparation")
	}
}

func TestStaticContextFactoryValidatesRequirements(t *testing.T) {
	factory := NewStaticContextFactory(Capabilities{})
	if err := factory.Prepare(RequirementSet{Modules: []string{"orders"}, Logger: true}); err == nil {
		t.Fatal("missing logger accepted")
	}
}
