package modulecatalog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	capabilityNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)
	capabilityTypePattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9_]*$`)
)

// CapabilityContract is the stable typed identity of a process/App-scoped
// capability exported by a composed module. Name is the logical binding key;
// Package and Type identify the Go contract used by the consumer.
type CapabilityContract struct {
	Name    string `json:"name"`
	Package string `json:"package"`
	Type    string `json:"type"`
}

// CapabilityKey is the compiler-visible handle used by bootstrap code to
// resolve one capability without exposing an untyped service locator.
type CapabilityKey[T any] struct {
	contract CapabilityContract
}

func NewCapabilityKey[T any](name, packagePath, typeName string) (CapabilityKey[T], error) {
	contract, err := normalizeCapabilityContract(CapabilityContract{Name: name, Package: packagePath, Type: typeName})
	if err != nil {
		return CapabilityKey[T]{}, err
	}
	return CapabilityKey[T]{contract: contract}, nil
}

func MustCapabilityKey[T any](name, packagePath, typeName string) CapabilityKey[T] {
	key, err := NewCapabilityKey[T](name, packagePath, typeName)
	if err != nil {
		panic(err)
	}
	return key
}

func (key CapabilityKey[T]) Contract() CapabilityContract { return key.contract }

// CapabilityExport is deliberately opaque. Modules can create an export only
// through ExportCapability, which couples a runtime value to a typed key.
type CapabilityExport struct {
	contract CapabilityContract
	value    any
}

func ExportCapability[T any](key CapabilityKey[T], value T) CapabilityExport {
	return CapabilityExport{contract: key.contract, value: value}
}

// CapabilityExporter is an optional module-instance contract. Exported values
// remain owned by the module/App lifecycle; the framework only snapshots the
// bindings for application construction.
type CapabilityExporter interface {
	ExportCapabilities() []CapabilityExport
}

type capabilityValue struct {
	module   string
	contract CapabilityContract
	value    any
}

// CapabilitySet is an immutable bootstrap-time capability snapshot. It has no
// string-based value lookup method; consumers resolve through CapabilityKey[T].
type CapabilitySet struct {
	values map[string]capabilityValue
}

func EmptyCapabilitySet() CapabilitySet { return CapabilitySet{} }
func (set CapabilitySet) Len() int      { return len(set.values) }

func (set CapabilitySet) Names() []string {
	result := make([]string, 0, len(set.values))
	for name := range set.values {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func ResolveCapability[T any](set CapabilitySet, key CapabilityKey[T]) (T, error) {
	var zero T
	contract, err := normalizeCapabilityContract(key.contract)
	if err != nil {
		return zero, err
	}
	binding, ok := set.values[contract.Name]
	if !ok {
		return zero, fmt.Errorf("modulecatalog: capability %q is not provided", contract.Name)
	}
	if binding.contract != contract {
		return zero, fmt.Errorf(
			"modulecatalog: capability %q contract mismatch: provider=%s.%s consumer=%s.%s",
			contract.Name,
			binding.contract.Package,
			binding.contract.Type,
			contract.Package,
			contract.Type,
		)
	}
	value, ok := binding.value.(T)
	if !ok {
		return zero, fmt.Errorf("modulecatalog: capability %q exported by module %q is not assignable to %s.%s", contract.Name, binding.module, contract.Package, contract.Type)
	}
	return value, nil
}

// CollectCapabilities validates already-built module exports against the sealed
// descriptor declarations and returns a bootstrap-only immutable snapshot.
// A descriptor promise and runtime export must match exactly; undeclared,
// missing, duplicate, or contract-mismatched exports fail before App start.
func CollectCapabilities(descriptors []Descriptor, instances []Instance) (CapabilitySet, error) {
	declared := make(map[string]map[string]CapabilityContract, len(descriptors))
	for _, descriptor := range descriptors {
		normalized, err := normalizeDescriptor(descriptor)
		if err != nil {
			return CapabilitySet{}, err
		}
		contracts := make(map[string]CapabilityContract, len(normalized.Provides))
		for _, contract := range normalized.Provides {
			contracts[contract.Name] = contract
		}
		declared[normalized.Name] = contracts
	}

	byModule := make(map[string]Instance, len(instances))
	for _, instance := range instances {
		if instance == nil {
			continue
		}
		name := strings.TrimSpace(instance.Name())
		if _, duplicate := byModule[name]; duplicate {
			return CapabilitySet{}, fmt.Errorf("modulecatalog: duplicate built module instance %q", name)
		}
		byModule[name] = instance
	}

	values := make(map[string]capabilityValue)
	for moduleName, contracts := range declared {
		instance := byModule[moduleName]
		exporter, exportsCapabilities := instance.(CapabilityExporter)
		if len(contracts) == 0 {
			if exportsCapabilities && len(exporter.ExportCapabilities()) > 0 {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q exports capabilities but descriptor declares none", moduleName)
			}
			continue
		}
		if instance == nil {
			return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q declares capabilities but has no built instance", moduleName)
		}
		if !exportsCapabilities {
			return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q declares capabilities but instance does not export them", moduleName)
		}
		seen := make(map[string]struct{}, len(contracts))
		for _, export := range exporter.ExportCapabilities() {
			contract, err := normalizeCapabilityContract(export.contract)
			if err != nil {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q capability export: %w", moduleName, err)
			}
			declaredContract, ok := contracts[contract.Name]
			if !ok {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q exports undeclared capability %q", moduleName, contract.Name)
			}
			if declaredContract != contract {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q capability %q contract differs from descriptor", moduleName, contract.Name)
			}
			if _, duplicate := seen[contract.Name]; duplicate {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q exports capability %q more than once", moduleName, contract.Name)
			}
			seen[contract.Name] = struct{}{}
			if previous, duplicate := values[contract.Name]; duplicate {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: capability %q has multiple providers: %q and %q", contract.Name, previous.module, moduleName)
			}
			values[contract.Name] = capabilityValue{module: moduleName, contract: contract, value: export.value}
		}
		for name := range contracts {
			if _, ok := seen[name]; !ok {
				return CapabilitySet{}, fmt.Errorf("modulecatalog: module %q did not export declared capability %q", moduleName, name)
			}
		}
	}
	for moduleName, instance := range byModule {
		if _, known := declared[moduleName]; known {
			continue
		}
		if exporter, ok := instance.(CapabilityExporter); ok && len(exporter.ExportCapabilities()) > 0 {
			return CapabilitySet{}, fmt.Errorf("modulecatalog: built module %q is not present in sealed descriptor plan", moduleName)
		}
	}
	if len(values) == 0 {
		return EmptyCapabilitySet(), nil
	}
	return CapabilitySet{values: values}, nil
}

func normalizeCapabilityContracts(owner string, values []CapabilityContract) ([]CapabilityContract, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]CapabilityContract, 0, len(values))
	for _, value := range values {
		normalized, err := normalizeCapabilityContract(value)
		if err != nil {
			return nil, fmt.Errorf("modulecatalog: module %q capability declaration: %w", owner, err)
		}
		if _, duplicate := seen[normalized.Name]; duplicate {
			return nil, fmt.Errorf("modulecatalog: module %q duplicate capability declaration %q", owner, normalized.Name)
		}
		seen[normalized.Name] = struct{}{}
		result = append(result, normalized)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name != result[j].Name {
			return result[i].Name < result[j].Name
		}
		if result[i].Package != result[j].Package {
			return result[i].Package < result[j].Package
		}
		return result[i].Type < result[j].Type
	})
	return result, nil
}

func normalizeCapabilityContract(contract CapabilityContract) (CapabilityContract, error) {
	contract.Name = strings.TrimSpace(contract.Name)
	contract.Package = strings.TrimSpace(contract.Package)
	contract.Type = strings.TrimSpace(contract.Type)
	if !capabilityNamePattern.MatchString(contract.Name) {
		return CapabilityContract{}, fmt.Errorf("modulecatalog: invalid capability name %q", contract.Name)
	}
	if contract.Package == "" || strings.ContainsAny(contract.Package, " \t\r\n") || strings.HasPrefix(contract.Package, ".") || strings.HasSuffix(contract.Package, "/") {
		return CapabilityContract{}, fmt.Errorf("modulecatalog: invalid capability package %q", contract.Package)
	}
	if !capabilityTypePattern.MatchString(contract.Type) {
		return CapabilityContract{}, fmt.Errorf("modulecatalog: invalid capability type %q", contract.Type)
	}
	return contract, nil
}
