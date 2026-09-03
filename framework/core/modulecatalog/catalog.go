package modulecatalog

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Plan struct {
	Descriptors []Descriptor `json:"descriptors"`
}

func (plan Plan) Names() []string {
	result := make([]string, 0, len(plan.Descriptors))
	for _, descriptor := range plan.Descriptors {
		result = append(result, descriptor.Name)
	}
	return result
}

func (plan Plan) Requirements() RequirementSet {
	set := RequirementSet{Modules: plan.Names()}
	databaseNames := make(map[string]struct{})
	rpcNames := make(map[string]struct{})
	for _, descriptor := range plan.Descriptors {
		requirements := descriptor.Requirements
		if requirements.ConfigKey != "" {
			set.Configs = append(set.Configs, ConfigRequirement{Module: descriptor.Name, Key: requirements.ConfigKey})
		}
		set.Logger = set.Logger || requirements.Logger
		set.EventBus = set.EventBus || requirements.EventBus
		for _, database := range requirements.Databases {
			databaseNames[database.Name] = struct{}{}
		}
		for _, rpc := range requirements.RPC {
			rpcNames[rpc.Name] = struct{}{}
		}
	}
	for name := range databaseNames {
		set.Databases = append(set.Databases, DatabaseRequirement{Name: name})
	}
	for name := range rpcNames {
		set.RPC = append(set.RPC, RPCRequirement{Name: name})
	}
	sort.Slice(set.Configs, func(left, right int) bool {
		if set.Configs[left].Module != set.Configs[right].Module {
			return set.Configs[left].Module < set.Configs[right].Module
		}
		return set.Configs[left].Key < set.Configs[right].Key
	})
	sort.Slice(set.Databases, func(left, right int) bool { return set.Databases[left].Name < set.Databases[right].Name })
	sort.Slice(set.RPC, func(left, right int) bool { return set.RPC[left].Name < set.RPC[right].Name })
	return set
}

type Catalog struct {
	mu          sync.RWMutex
	descriptors map[string]Descriptor
	sealed      bool
	plan        Plan
}

func New() *Catalog {
	return &Catalog{descriptors: make(map[string]Descriptor)}
}

func (catalog *Catalog) Register(descriptor Descriptor) error {
	if catalog == nil {
		return fmt.Errorf("modulecatalog: nil catalog")
	}
	normalized, err := normalizeDescriptor(descriptor)
	if err != nil {
		return err
	}
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if catalog.sealed {
		return fmt.Errorf("modulecatalog: catalog is sealed")
	}
	if _, duplicate := catalog.descriptors[normalized.Name]; duplicate {
		return fmt.Errorf("modulecatalog: duplicate module %q", normalized.Name)
	}
	catalog.descriptors[normalized.Name] = cloneDescriptor(normalized)
	return nil
}

func (catalog *Catalog) MustRegister(descriptor Descriptor) {
	if err := catalog.Register(descriptor); err != nil {
		panic(err)
	}
}

func (catalog *Catalog) Seal() (Plan, error) {
	if catalog == nil {
		return Plan{}, fmt.Errorf("modulecatalog: nil catalog")
	}
	catalog.mu.Lock()
	defer catalog.mu.Unlock()
	if catalog.sealed {
		return clonePlan(catalog.plan), nil
	}
	plan, err := resolvePlan(catalog.descriptors)
	if err != nil {
		return Plan{}, err
	}
	catalog.plan = plan
	catalog.sealed = true
	return clonePlan(plan), nil
}

func (catalog *Catalog) IsSealed() bool {
	if catalog == nil {
		return false
	}
	catalog.mu.RLock()
	defer catalog.mu.RUnlock()
	return catalog.sealed
}

func (catalog *Catalog) Len() int {
	if catalog == nil {
		return 0
	}
	catalog.mu.RLock()
	defer catalog.mu.RUnlock()
	return len(catalog.descriptors)
}

func resolvePlan(descriptors map[string]Descriptor) (Plan, error) {
	providers := make(map[string]string)
	for name, descriptor := range descriptors {
		for _, capability := range descriptor.Provides {
			if previous, duplicate := providers[capability.Name]; duplicate {
				return Plan{}, fmt.Errorf("modulecatalog: capability %q has multiple providers: %q and %q", capability.Name, previous, name)
			}
			providers[capability.Name] = name
		}
	}

	indegree := make(map[string]int, len(descriptors))
	dependents := make(map[string][]string, len(descriptors))
	for name := range descriptors {
		indegree[name] = 0
	}
	for name, descriptor := range descriptors {
		for _, dependency := range descriptor.DependsOn {
			if _, exists := descriptors[dependency]; !exists {
				return Plan{}, fmt.Errorf("modulecatalog: module %q dependency %q is not registered", name, dependency)
			}
			indegree[name]++
			dependents[dependency] = append(dependents[dependency], name)
		}
	}
	for name := range dependents {
		sort.Strings(dependents[name])
	}

	ready := make([]string, 0, len(descriptors))
	for name, degree := range indegree {
		if degree == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	ordered := make([]Descriptor, 0, len(descriptors))
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		ordered = append(ordered, cloneDescriptor(descriptors[name]))
		for _, dependent := range dependents[name] {
			indegree[dependent]--
			if indegree[dependent] == 0 {
				ready = insertSorted(ready, dependent)
			}
		}
	}
	if len(ordered) != len(descriptors) {
		unresolved := make([]string, 0)
		for name, degree := range indegree {
			if degree > 0 {
				unresolved = append(unresolved, name)
			}
		}
		sort.Strings(unresolved)
		return Plan{}, fmt.Errorf("modulecatalog: dependency cycle among %s", strings.Join(unresolved, ", "))
	}
	return Plan{Descriptors: ordered}, nil
}

func insertSorted(values []string, value string) []string {
	index := sort.SearchStrings(values, value)
	values = append(values, "")
	copy(values[index+1:], values[index:])
	values[index] = value
	return values
}

func clonePlan(plan Plan) Plan {
	clone := Plan{Descriptors: make([]Descriptor, len(plan.Descriptors))}
	for index, descriptor := range plan.Descriptors {
		clone.Descriptors[index] = cloneDescriptor(descriptor)
	}
	return clone
}

var defaultCatalog = New()

func Default() *Catalog { return defaultCatalog }

func MustRegister(descriptor Descriptor) { defaultCatalog.MustRegister(descriptor) }
