package assemblyplan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	SchemaVersion = 1
	RootTarget    = "root"
)

type Ownership string

const (
	OwnershipCanonical    Ownership = "canonical"
	OwnershipReused       Ownership = "reused"
	OwnershipDerived      Ownership = "derived"
	OwnershipRuntimeLocal Ownership = "runtime_local"
)

type Evidence struct {
	Ownership Ownership `json:"ownership"`
	Source    string    `json:"source"`
	Ref       string    `json:"ref"`
}

type Application struct {
	ID           string                  `json:"id"`
	Domain       string                  `json:"domain"`
	Name         string                  `json:"name"`
	Capabilities []CapabilityRequirement `json:"capabilities,omitempty"`
	Evidence     Evidence                `json:"evidence"`
}

type Dependency struct {
	From     string   `json:"from"`
	To       string   `json:"to"`
	Evidence Evidence `json:"evidence"`
}

type Operation struct {
	ID          string             `json:"id"`
	Application string             `json:"application"`
	Bindings    []BindingReference `json:"bindings,omitempty"`
	Evidence    Evidence           `json:"evidence"`
}

type BindingReference struct {
	OperationID string   `json:"operationId"`
	Transport   string   `json:"transport"`
	Index       int      `json:"index"`
	Evidence    Evidence `json:"evidence"`
}

type ModuleRequirements struct {
	ConfigKey string   `json:"configKey,omitempty"`
	Logger    bool     `json:"logger,omitempty"`
	Databases []string `json:"databases,omitempty"`
	EventBus  bool     `json:"eventBus,omitempty"`
	RPC       []string `json:"rpc,omitempty"`
}

type Module struct {
	Name         string             `json:"name"`
	Version      string             `json:"version,omitempty"`
	Requirements ModuleRequirements `json:"requirements,omitempty"`
	Evidence     Evidence           `json:"evidence"`
}

type Requirement struct {
	Module   string   `json:"module"`
	Kind     string   `json:"kind"`
	Name     string   `json:"name,omitempty"`
	Evidence Evidence `json:"evidence"`
}

type BootstrapTarget struct {
	Name               string   `json:"name"`
	Applications       []string `json:"applications,omitempty"`
	Modules            []string `json:"modules,omitempty"`
	ExternalOperations []string `json:"externalOperations,omitempty"`
	InternalOperations []string `json:"internalOperations,omitempty"`
	Evidence           Evidence `json:"evidence"`
}

type Plan struct {
	SchemaVersion                int               `json:"schemaVersion"`
	Identity                     string            `json:"identity"`
	Applications                 []Application     `json:"applications,omitempty"`
	ApplicationDependencies      []Dependency      `json:"applicationDependencies,omitempty"`
	ApplicationDependencyClosure []Dependency      `json:"applicationDependencyClosure,omitempty"`
	Operations                   []Operation       `json:"operations,omitempty"`
	OperationDependencies        []Dependency      `json:"operationDependencies,omitempty"`
	Modules                      []Module          `json:"modules,omitempty"`
	ModuleDependencies           []Dependency      `json:"moduleDependencies,omitempty"`
	Requirements                 []Requirement     `json:"requirements,omitempty"`
	Targets                      []BootstrapTarget `json:"targets"`
}

type ApplicationInput struct {
	ID           string
	Domain       string
	Name         string
	DependsOn    []string
	Capabilities []CapabilityRequirement
	Evidence     Evidence
}

type BindingInput struct {
	Transport string
	Index     int
	Evidence  Evidence
}

type OperationInput struct {
	ID                 string
	Application        string
	RequiresOperations []string
	Bindings           []BindingInput
	Evidence           Evidence
}

type ModuleInput struct {
	Name         string
	Version      string
	DependsOn    []string
	Requirements ModuleRequirements
	Evidence     Evidence
}

type Input struct {
	Identity     string
	Applications []ApplicationInput
	Operations   []OperationInput
	Modules      []ModuleInput
}

func Compile(input Input) (Plan, error) {
	input = normalizeInput(input)
	if input.Identity == "" {
		input.Identity = RootTarget
	}

	plan := Plan{SchemaVersion: SchemaVersion, Identity: input.Identity}
	for _, item := range input.Applications {
		plan.Applications = append(plan.Applications, Application{
			ID: item.ID, Domain: item.Domain, Name: item.Name,
			Capabilities: append([]CapabilityRequirement(nil), item.Capabilities...), Evidence: item.Evidence,
		})
		for _, dependency := range item.DependsOn {
			plan.ApplicationDependencies = append(plan.ApplicationDependencies, Dependency{
				From: item.ID,
				To:   dependency,
				Evidence: Evidence{
					Ownership: OwnershipReused,
					Source:    item.Evidence.Source,
					Ref:       item.Evidence.Ref + "/requires/" + dependency,
				},
			})
		}
	}
	plan.ApplicationDependencyClosure = dependencyClosure(plan.ApplicationDependencies, "application-dependency-closure")

	for _, item := range input.Operations {
		operation := Operation{ID: item.ID, Application: item.Application, Evidence: item.Evidence}
		for _, binding := range item.Bindings {
			operation.Bindings = append(operation.Bindings, BindingReference{
				OperationID: item.ID,
				Transport:   binding.Transport,
				Index:       binding.Index,
				Evidence:    binding.Evidence,
			})
		}
		plan.Operations = append(plan.Operations, operation)
		for _, dependency := range item.RequiresOperations {
			plan.OperationDependencies = append(plan.OperationDependencies, Dependency{
				From: item.ID,
				To:   dependency,
				Evidence: Evidence{
					Ownership: OwnershipReused,
					Source:    item.Evidence.Source,
					Ref:       item.Evidence.Ref + "/requiresOperations/" + dependency,
				},
			})
		}
	}

	for _, item := range input.Modules {
		module := Module{
			Name:         item.Name,
			Version:      item.Version,
			Requirements: item.Requirements,
			Evidence:     item.Evidence,
		}
		plan.Modules = append(plan.Modules, module)
		for _, dependency := range item.DependsOn {
			plan.ModuleDependencies = append(plan.ModuleDependencies, Dependency{
				From: item.Name,
				To:   dependency,
				Evidence: Evidence{
					Ownership: OwnershipReused,
					Source:    item.Evidence.Source,
					Ref:       item.Evidence.Ref + "/dependsOn/" + dependency,
				},
			})
		}
		plan.Requirements = append(plan.Requirements, requirementsForModule(module)...)
	}

	plan = Normalize(plan)
	plan.Targets = []BootstrapTarget{deriveRootTarget(plan)}
	plan = Normalize(plan)
	if err := Validate(plan); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func Normalize(plan Plan) Plan {
	if plan.SchemaVersion == 0 {
		plan.SchemaVersion = SchemaVersion
	}
	plan.Identity = strings.TrimSpace(plan.Identity)

	for index := range plan.Applications {
		item := &plan.Applications[index]
		item.ID = strings.TrimSpace(item.ID)
		item.Domain = strings.TrimSpace(item.Domain)
		item.Name = strings.TrimSpace(item.Name)
		item.Capabilities = normalizeCapabilityRequirements(item.Capabilities)
		item.Evidence = normalizeEvidence(item.Evidence)
	}
	sort.Slice(plan.Applications, func(i, j int) bool { return plan.Applications[i].ID < plan.Applications[j].ID })

	plan.ApplicationDependencies = normalizeDependencies(plan.ApplicationDependencies)
	plan.ApplicationDependencyClosure = normalizeDependencies(plan.ApplicationDependencyClosure)
	plan.OperationDependencies = normalizeDependencies(plan.OperationDependencies)
	plan.ModuleDependencies = normalizeDependencies(plan.ModuleDependencies)

	for index := range plan.Operations {
		item := &plan.Operations[index]
		item.ID = strings.TrimSpace(item.ID)
		item.Application = strings.TrimSpace(item.Application)
		item.Evidence = normalizeEvidence(item.Evidence)
		for bindingIndex := range item.Bindings {
			binding := &item.Bindings[bindingIndex]
			binding.OperationID = strings.TrimSpace(binding.OperationID)
			binding.Transport = strings.ToLower(strings.TrimSpace(binding.Transport))
			binding.Evidence = normalizeEvidence(binding.Evidence)
		}
		sort.Slice(item.Bindings, func(i, j int) bool {
			if item.Bindings[i].Transport != item.Bindings[j].Transport {
				return item.Bindings[i].Transport < item.Bindings[j].Transport
			}
			return item.Bindings[i].Index < item.Bindings[j].Index
		})
	}
	sort.Slice(plan.Operations, func(i, j int) bool { return plan.Operations[i].ID < plan.Operations[j].ID })

	for index := range plan.Modules {
		item := &plan.Modules[index]
		item.Name = strings.TrimSpace(item.Name)
		item.Version = strings.TrimSpace(item.Version)
		item.Requirements.ConfigKey = strings.TrimSpace(item.Requirements.ConfigKey)
		item.Requirements.Databases = stableStrings(item.Requirements.Databases)
		item.Requirements.RPC = stableStrings(item.Requirements.RPC)
		item.Evidence = normalizeEvidence(item.Evidence)
	}
	sort.Slice(plan.Modules, func(i, j int) bool { return plan.Modules[i].Name < plan.Modules[j].Name })

	for index := range plan.Requirements {
		item := &plan.Requirements[index]
		item.Module = strings.TrimSpace(item.Module)
		item.Kind = strings.ToLower(strings.TrimSpace(item.Kind))
		item.Name = strings.TrimSpace(item.Name)
		item.Evidence = normalizeEvidence(item.Evidence)
	}
	sort.Slice(plan.Requirements, func(i, j int) bool {
		left, right := plan.Requirements[i], plan.Requirements[j]
		if left.Module != right.Module {
			return left.Module < right.Module
		}
		if left.Kind != right.Kind {
			return left.Kind < right.Kind
		}
		return left.Name < right.Name
	})

	for index := range plan.Targets {
		target := &plan.Targets[index]
		target.Name = strings.TrimSpace(target.Name)
		target.Applications = stableStrings(target.Applications)
		target.Modules = stableStrings(target.Modules)
		target.ExternalOperations = stableStrings(target.ExternalOperations)
		target.InternalOperations = stableStrings(target.InternalOperations)
		target.Evidence = normalizeEvidence(target.Evidence)
	}
	sort.Slice(plan.Targets, func(i, j int) bool { return plan.Targets[i].Name < plan.Targets[j].Name })
	return plan
}

func Validate(plan Plan) error {
	plan = Normalize(plan)
	if plan.SchemaVersion != SchemaVersion {
		return fmt.Errorf("assemblyplan: unsupported schemaVersion %d", plan.SchemaVersion)
	}
	if plan.Identity == "" {
		return errors.New("assemblyplan: identity is required")
	}

	applications := make(map[string]Application, len(plan.Applications))
	for _, item := range plan.Applications {
		if item.ID == "" || item.Domain == "" || item.Name == "" {
			return errors.New("assemblyplan: application id, domain, and name are required")
		}
		if item.ID != item.Domain+"/"+item.Name {
			return fmt.Errorf("assemblyplan: application %s does not match domain/name %s/%s", item.ID, item.Domain, item.Name)
		}
		if _, duplicate := applications[item.ID]; duplicate {
			return fmt.Errorf("assemblyplan: duplicate application %s", item.ID)
		}
		if err := validateCapabilityRequirements(item.ID, item.Capabilities); err != nil {
			return err
		}
		if err := validateEvidence(item.Evidence, "application "+item.ID); err != nil {
			return err
		}
		applications[item.ID] = item
	}
	if err := validateDependencies("application", plan.ApplicationDependencies, stringSetKeys(applications)); err != nil {
		return err
	}
	if cycle := firstCycle(plan.ApplicationDependencies, stringSetKeys(applications)); len(cycle) > 0 {
		return fmt.Errorf("assemblyplan: application dependency cycle: %s", strings.Join(cycle, " -> "))
	}
	wantClosure := normalizeDependencies(dependencyClosure(plan.ApplicationDependencies, "application-dependency-closure"))
	if !equalDependencies(wantClosure, plan.ApplicationDependencyClosure) {
		return errors.New("assemblyplan: stale application dependency closure")
	}

	operations := make(map[string]Operation, len(plan.Operations))
	for _, item := range plan.Operations {
		if item.ID == "" {
			return errors.New("assemblyplan: operation id is required")
		}
		if _, duplicate := operations[item.ID]; duplicate {
			return fmt.Errorf("assemblyplan: duplicate operation %s", item.ID)
		}
		if _, ok := applications[item.Application]; !ok {
			return fmt.Errorf("assemblyplan: operation %s references unknown application %s", item.ID, item.Application)
		}
		if err := validateEvidence(item.Evidence, "operation "+item.ID); err != nil {
			return err
		}
		seenBindings := map[string]struct{}{}
		for _, binding := range item.Bindings {
			if binding.OperationID != item.ID {
				return fmt.Errorf("assemblyplan: binding operation %s does not match owner %s", binding.OperationID, item.ID)
			}
			if binding.Index < 0 {
				return fmt.Errorf("assemblyplan: operation %s has negative binding index", item.ID)
			}
			switch binding.Transport {
			case "http", "rpc":
			default:
				return fmt.Errorf("assemblyplan: operation %s has unsupported transport %q", item.ID, binding.Transport)
			}
			key := binding.Transport + ":" + strconv.Itoa(binding.Index)
			if _, duplicate := seenBindings[key]; duplicate {
				return fmt.Errorf("assemblyplan: operation %s has duplicate binding %s", item.ID, key)
			}
			seenBindings[key] = struct{}{}
			if err := validateEvidence(binding.Evidence, "binding "+item.ID+"/"+key); err != nil {
				return err
			}
		}
		operations[item.ID] = item
	}
	if err := validateDependencies("operation", plan.OperationDependencies, stringSetKeys(operations)); err != nil {
		return err
	}
	if cycle := firstCycle(plan.OperationDependencies, stringSetKeys(operations)); len(cycle) > 0 {
		return fmt.Errorf("assemblyplan: operation dependency cycle: %s", strings.Join(cycle, " -> "))
	}

	modules := make(map[string]Module, len(plan.Modules))
	for _, item := range plan.Modules {
		if item.Name == "" {
			return errors.New("assemblyplan: module name is required")
		}
		if _, duplicate := modules[item.Name]; duplicate {
			return fmt.Errorf("assemblyplan: duplicate module %s", item.Name)
		}
		if err := validateEvidence(item.Evidence, "module "+item.Name); err != nil {
			return err
		}
		modules[item.Name] = item
	}
	if err := validateDependencies("module", plan.ModuleDependencies, stringSetKeys(modules)); err != nil {
		return err
	}
	if cycle := firstCycle(plan.ModuleDependencies, stringSetKeys(modules)); len(cycle) > 0 {
		return fmt.Errorf("assemblyplan: module dependency cycle: %s", strings.Join(cycle, " -> "))
	}

	wantRequirements := make([]Requirement, 0)
	for _, module := range plan.Modules {
		wantRequirements = append(wantRequirements, requirementsForModule(module)...)
	}
	wantRequirements = normalizeRequirements(wantRequirements)
	if !equalRequirements(wantRequirements, plan.Requirements) {
		return errors.New("assemblyplan: module capability requirement inventory is stale or incomplete")
	}
	for _, requirement := range plan.Requirements {
		if _, ok := modules[requirement.Module]; !ok {
			return fmt.Errorf("assemblyplan: requirement references unknown module %s", requirement.Module)
		}
		if err := validateRequirement(requirement); err != nil {
			return err
		}
	}

	if len(plan.Targets) != 1 || plan.Targets[0].Name != RootTarget {
		return fmt.Errorf("assemblyplan: schema v1 requires exactly one %q bootstrap target", RootTarget)
	}
	wantTarget := deriveRootTarget(plan)
	if !equalTarget(wantTarget, plan.Targets[0]) {
		return errors.New("assemblyplan: root bootstrap target is stale or incomplete")
	}
	return nil
}

func CanonicalJSON(plan Plan) ([]byte, error) {
	plan = Normalize(plan)
	if err := Validate(plan); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func Digest(plan Plan) (string, error) {
	data, err := CanonicalJSON(plan)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func Inspect(plan Plan) (Summary, error) {
	plan = Normalize(plan)
	if err := Validate(plan); err != nil {
		return Summary{}, err
	}
	target := plan.Targets[0]
	return Summary{
		Identity:             plan.Identity,
		Applications:         append([]string(nil), target.Applications...),
		Modules:              append([]string(nil), target.Modules...),
		ExternalOperations:   append([]string(nil), target.ExternalOperations...),
		InternalOperations:   append([]string(nil), target.InternalOperations...),
		RequirementCount:     len(plan.Requirements),
		ApplicationEdgeCount: len(plan.ApplicationDependencies),
		OperationEdgeCount:   len(plan.OperationDependencies),
		ModuleEdgeCount:      len(plan.ModuleDependencies),
	}, nil
}

type Summary struct {
	Identity             string   `json:"identity"`
	Applications         []string `json:"applications,omitempty"`
	Modules              []string `json:"modules,omitempty"`
	ExternalOperations   []string `json:"externalOperations,omitempty"`
	InternalOperations   []string `json:"internalOperations,omitempty"`
	RequirementCount     int      `json:"requirementCount"`
	ApplicationEdgeCount int      `json:"applicationEdgeCount"`
	OperationEdgeCount   int      `json:"operationEdgeCount"`
	ModuleEdgeCount      int      `json:"moduleEdgeCount"`
}

func normalizeInput(input Input) Input {
	input.Identity = strings.TrimSpace(input.Identity)
	for index := range input.Applications {
		item := &input.Applications[index]
		item.ID = strings.TrimSpace(item.ID)
		item.Domain = strings.TrimSpace(item.Domain)
		item.Name = strings.TrimSpace(item.Name)
		item.DependsOn = stableStrings(item.DependsOn)
		item.Capabilities = normalizeCapabilityRequirements(item.Capabilities)
		item.Evidence = normalizeEvidence(item.Evidence)
	}
	sort.Slice(input.Applications, func(i, j int) bool { return input.Applications[i].ID < input.Applications[j].ID })
	for index := range input.Operations {
		item := &input.Operations[index]
		item.ID = strings.TrimSpace(item.ID)
		item.Application = strings.TrimSpace(item.Application)
		item.RequiresOperations = stableStrings(item.RequiresOperations)
		item.Evidence = normalizeEvidence(item.Evidence)
		for bindingIndex := range item.Bindings {
			item.Bindings[bindingIndex].Transport = strings.ToLower(strings.TrimSpace(item.Bindings[bindingIndex].Transport))
			item.Bindings[bindingIndex].Evidence = normalizeEvidence(item.Bindings[bindingIndex].Evidence)
		}
		sort.Slice(item.Bindings, func(i, j int) bool {
			if item.Bindings[i].Transport != item.Bindings[j].Transport {
				return item.Bindings[i].Transport < item.Bindings[j].Transport
			}
			return item.Bindings[i].Index < item.Bindings[j].Index
		})
	}
	sort.Slice(input.Operations, func(i, j int) bool { return input.Operations[i].ID < input.Operations[j].ID })
	for index := range input.Modules {
		item := &input.Modules[index]
		item.Name = strings.TrimSpace(item.Name)
		item.Version = strings.TrimSpace(item.Version)
		item.DependsOn = stableStrings(item.DependsOn)
		item.Requirements.ConfigKey = strings.TrimSpace(item.Requirements.ConfigKey)
		item.Requirements.Databases = stableStrings(item.Requirements.Databases)
		item.Requirements.RPC = stableStrings(item.Requirements.RPC)
		item.Evidence = normalizeEvidence(item.Evidence)
	}
	sort.Slice(input.Modules, func(i, j int) bool { return input.Modules[i].Name < input.Modules[j].Name })
	return input
}

func normalizeEvidence(evidence Evidence) Evidence {
	evidence.Ownership = Ownership(strings.TrimSpace(string(evidence.Ownership)))
	evidence.Source = strings.TrimSpace(evidence.Source)
	evidence.Ref = strings.TrimSpace(evidence.Ref)
	return evidence
}

func validateEvidence(evidence Evidence, owner string) error {
	switch evidence.Ownership {
	case OwnershipCanonical, OwnershipReused, OwnershipDerived, OwnershipRuntimeLocal:
	default:
		return fmt.Errorf("assemblyplan: %s has invalid evidence ownership %q", owner, evidence.Ownership)
	}
	if evidence.Source == "" || evidence.Ref == "" {
		return fmt.Errorf("assemblyplan: %s requires evidence source and ref", owner)
	}
	return nil
}

func normalizeDependencies(values []Dependency) []Dependency {
	result := append([]Dependency(nil), values...)
	for index := range result {
		result[index].From = strings.TrimSpace(result[index].From)
		result[index].To = strings.TrimSpace(result[index].To)
		result[index].Evidence = normalizeEvidence(result[index].Evidence)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].From != result[j].From {
			return result[i].From < result[j].From
		}
		return result[i].To < result[j].To
	})
	return result
}

func validateDependencies(kind string, dependencies []Dependency, nodes []string) error {
	known := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		known[node] = struct{}{}
	}
	seen := map[string]struct{}{}
	for _, dependency := range normalizeDependencies(dependencies) {
		if _, ok := known[dependency.From]; !ok {
			return fmt.Errorf("assemblyplan: %s dependency has unknown source %s", kind, dependency.From)
		}
		if _, ok := known[dependency.To]; !ok {
			return fmt.Errorf("assemblyplan: %s %s requires unknown %s %s", kind, dependency.From, kind, dependency.To)
		}
		if dependency.From == dependency.To {
			return fmt.Errorf("assemblyplan: %s %s cannot depend on itself", kind, dependency.From)
		}
		key := dependency.From + "\x00" + dependency.To
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("assemblyplan: duplicate %s dependency %s -> %s", kind, dependency.From, dependency.To)
		}
		seen[key] = struct{}{}
		if err := validateEvidence(dependency.Evidence, kind+" dependency "+dependency.From+" -> "+dependency.To); err != nil {
			return err
		}
	}
	return nil
}

func dependencyClosure(direct []Dependency, source string) []Dependency {
	adjacency := map[string][]string{}
	nodes := map[string]struct{}{}
	for _, edge := range normalizeDependencies(direct) {
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		nodes[edge.From] = struct{}{}
		nodes[edge.To] = struct{}{}
	}
	for node := range adjacency {
		adjacency[node] = stableStrings(adjacency[node])
	}
	ids := stringSetKeys(nodes)
	var result []Dependency
	for _, from := range ids {
		visited := map[string]bool{}
		var visit func(string)
		visit = func(current string) {
			for _, next := range adjacency[current] {
				if visited[next] {
					continue
				}
				visited[next] = true
				visit(next)
			}
		}
		visit(from)
		for to := range visited {
			if to == from {
				continue
			}
			result = append(result, Dependency{
				From: from,
				To:   to,
				Evidence: Evidence{
					Ownership: OwnershipDerived,
					Source:    source,
					Ref:       from + "->" + to,
				},
			})
		}
	}
	return normalizeDependencies(result)
}

func firstCycle(dependencies []Dependency, nodes []string) []string {
	adjacency := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		adjacency[node] = nil
	}
	for _, edge := range dependencies {
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
	}
	for node := range adjacency {
		adjacency[node] = stableStrings(adjacency[node])
	}
	state := map[string]uint8{}
	position := map[string]int{}
	stack := []string{}
	var cycle []string
	var visit func(string) bool
	visit = func(node string) bool {
		if state[node] == 2 {
			return false
		}
		if state[node] == 1 {
			start := position[node]
			cycle = append(append([]string(nil), stack[start:]...), node)
			return true
		}
		state[node] = 1
		position[node] = len(stack)
		stack = append(stack, node)
		for _, next := range adjacency[node] {
			if visit(next) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		delete(position, node)
		state[node] = 2
		return false
	}
	for _, node := range stableStrings(nodes) {
		if visit(node) {
			return cycle
		}
	}
	return nil
}

func requirementsForModule(module Module) []Requirement {
	base := func(kind, name, ref string) Requirement {
		return Requirement{
			Module: module.Name,
			Kind:   kind,
			Name:   name,
			Evidence: Evidence{
				Ownership: OwnershipReused,
				Source:    module.Evidence.Source,
				Ref:       module.Evidence.Ref + ref,
			},
		}
	}
	var result []Requirement
	if module.Requirements.ConfigKey != "" {
		result = append(result, base("config", module.Requirements.ConfigKey, "/requirements/configKey"))
	}
	if module.Requirements.Logger {
		result = append(result, base("logger", "", "/requirements/logger"))
	}
	for _, database := range stableStrings(module.Requirements.Databases) {
		result = append(result, base("database", database, "/requirements/databases/"+database))
	}
	if module.Requirements.EventBus {
		result = append(result, base("event_bus", "", "/requirements/eventBus"))
	}
	for _, rpc := range stableStrings(module.Requirements.RPC) {
		result = append(result, base("rpc", rpc, "/requirements/rpc/"+rpc))
	}
	return normalizeRequirements(result)
}

func normalizeRequirements(values []Requirement) []Requirement {
	result := append([]Requirement(nil), values...)
	for index := range result {
		result[index].Module = strings.TrimSpace(result[index].Module)
		result[index].Kind = strings.ToLower(strings.TrimSpace(result[index].Kind))
		result[index].Name = strings.TrimSpace(result[index].Name)
		result[index].Evidence = normalizeEvidence(result[index].Evidence)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Module != result[j].Module {
			return result[i].Module < result[j].Module
		}
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].Name < result[j].Name
	})
	return result
}

func validateRequirement(requirement Requirement) error {
	switch requirement.Kind {
	case "config", "database", "rpc":
		if requirement.Name == "" {
			return fmt.Errorf("assemblyplan: %s requirement for module %s requires a name", requirement.Kind, requirement.Module)
		}
	case "logger", "event_bus":
		if requirement.Name != "" {
			return fmt.Errorf("assemblyplan: %s requirement for module %s must not carry a name", requirement.Kind, requirement.Module)
		}
	default:
		return fmt.Errorf("assemblyplan: module %s has unsupported requirement kind %q", requirement.Module, requirement.Kind)
	}
	return validateEvidence(requirement.Evidence, "requirement "+requirement.Module+"/"+requirement.Kind)
}

func deriveRootTarget(plan Plan) BootstrapTarget {
	target := BootstrapTarget{
		Name: RootTarget,
		Evidence: Evidence{
			Ownership: OwnershipDerived,
			Source:    "assemblyplan",
			Ref:       "targets/root",
		},
	}
	for _, application := range plan.Applications {
		target.Applications = append(target.Applications, application.ID)
	}
	for _, module := range plan.Modules {
		target.Modules = append(target.Modules, module.Name)
	}
	for _, operation := range plan.Operations {
		if len(operation.Bindings) == 0 {
			target.InternalOperations = append(target.InternalOperations, operation.ID)
		} else {
			target.ExternalOperations = append(target.ExternalOperations, operation.ID)
		}
	}
	target.Applications = stableStrings(target.Applications)
	target.Modules = stableStrings(target.Modules)
	target.ExternalOperations = stableStrings(target.ExternalOperations)
	target.InternalOperations = stableStrings(target.InternalOperations)
	return target
}

func equalDependencies(left, right []Dependency) bool {
	left = normalizeDependencies(left)
	right = normalizeDependencies(right)
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

func equalRequirements(left, right []Requirement) bool {
	left = normalizeRequirements(left)
	right = normalizeRequirements(right)
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

func equalTarget(left, right BootstrapTarget) bool {
	left.Applications = stableStrings(left.Applications)
	left.Modules = stableStrings(left.Modules)
	left.ExternalOperations = stableStrings(left.ExternalOperations)
	left.InternalOperations = stableStrings(left.InternalOperations)
	left.Evidence = normalizeEvidence(left.Evidence)
	right.Applications = stableStrings(right.Applications)
	right.Modules = stableStrings(right.Modules)
	right.ExternalOperations = stableStrings(right.ExternalOperations)
	right.InternalOperations = stableStrings(right.InternalOperations)
	right.Evidence = normalizeEvidence(right.Evidence)
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

func stableStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func stringSetKeys[T any](values map[string]T) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
