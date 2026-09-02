package modulecatalog

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"gorm.io/gorm"
	"github.com/hvritual/yunka.io/framework/core/eventBus"
	"github.com/hvritual/yunka.io/pkg/logExt"
)

var moduleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)

type DatabaseRequirement struct {
	Name string `json:"name"`
}

type RPCRequirement struct {
	Name string `json:"name"`
}

type Requirements struct {
	ConfigKey string                `json:"configKey,omitempty"`
	Logger    bool                  `json:"logger,omitempty"`
	Databases []DatabaseRequirement `json:"databases,omitempty"`
	EventBus  bool                  `json:"eventBus,omitempty"`
	RPC       []RPCRequirement      `json:"rpc,omitempty"`
}

type ConfigRequirement struct {
	Module string `json:"module"`
	Key    string `json:"key"`
}

// RequirementSet is the deterministic aggregate of every sealed descriptor.
// A ContextFactory receives this set once before any module Build function is
// invoked, so shared platform resources can be validated and created once.
type RequirementSet struct {
	Modules   []string              `json:"modules"`
	Configs   []ConfigRequirement   `json:"configs,omitempty"`
	Logger    bool                  `json:"logger,omitempty"`
	Databases []DatabaseRequirement `json:"databases,omitempty"`
	EventBus  bool                  `json:"eventBus,omitempty"`
	RPC       []RPCRequirement      `json:"rpc,omitempty"`
}

type Instance interface {
	Name() string
}

type BuildFunc func(BuildContext) (Instance, error)

type Descriptor struct {
	Name         string       `json:"name"`
	Version      string       `json:"version,omitempty"`
	DependsOn    []string     `json:"dependsOn,omitempty"`
	Requirements Requirements `json:"requirements,omitempty"`
	Build        BuildFunc    `json:"-"`
}

type ConfigProvider interface {
	Decode(moduleName, key string, target any) error
}

type DatabaseProvider interface {
	GORM(name string) (*gorm.DB, error)
}

type RPCProvider interface {
	Connection(name string) (grpc.ClientConnInterface, error)
}

type BuildContext interface {
	Descriptor() Descriptor
	Config() ConfigProvider
	Logger() logExt.Logger
	Databases() DatabaseProvider
	EventBus() eventBus.EventBus
	RPC() RPCProvider
}

type ContextFactory interface {
	// Prepare is called exactly once per App construction before module Build
	// functions run. Implementations aggregate and own shared capabilities.
	Prepare(RequirementSet) error
	ForModule(Descriptor) (BuildContext, error)
}

type ContextFactoryFunc func(Descriptor) (BuildContext, error)

func (factory ContextFactoryFunc) Prepare(RequirementSet) error { return nil }

func (factory ContextFactoryFunc) ForModule(descriptor Descriptor) (BuildContext, error) {
	if factory == nil {
		return nil, fmt.Errorf("modulecatalog: nil context factory")
	}
	return factory(descriptor)
}

type Capabilities struct {
	Config    ConfigProvider
	Logger    logExt.Logger
	Databases DatabaseProvider
	EventBus  eventBus.EventBus
	RPC       RPCProvider
}

type StaticContextFactory struct {
	mu           sync.RWMutex
	capabilities Capabilities
	prepared     bool
	requirements RequirementSet
}

func NewStaticContextFactory(capabilities Capabilities) *StaticContextFactory {
	return &StaticContextFactory{capabilities: capabilities}
}

func (factory *StaticContextFactory) Prepare(requirements RequirementSet) error {
	if factory == nil {
		return fmt.Errorf("modulecatalog: nil static context factory")
	}
	requirements = cloneRequirementSet(requirements)
	factory.mu.Lock()
	defer factory.mu.Unlock()
	if factory.prepared {
		if !equalRequirementSets(factory.requirements, requirements) {
			return fmt.Errorf("modulecatalog: static context factory already prepared for a different requirement set")
		}
		return nil
	}
	if len(requirements.Configs) > 0 && factory.capabilities.Config == nil {
		return fmt.Errorf("modulecatalog: configuration provider is required")
	}
	if requirements.Logger && factory.capabilities.Logger == nil {
		return fmt.Errorf("modulecatalog: logger is required")
	}
	if len(requirements.Databases) > 0 && factory.capabilities.Databases == nil {
		return fmt.Errorf("modulecatalog: database provider is required")
	}
	if requirements.EventBus && factory.capabilities.EventBus == nil {
		return fmt.Errorf("modulecatalog: event bus is required")
	}
	if len(requirements.RPC) > 0 && factory.capabilities.RPC == nil {
		return fmt.Errorf("modulecatalog: RPC provider is required")
	}

	preparedCapabilities := factory.capabilities
	if len(requirements.Databases) > 0 {
		resolved := make(databaseMap, len(requirements.Databases))
		for _, requirement := range requirements.Databases {
			database, err := factory.capabilities.Databases.GORM(requirement.Name)
			if err != nil {
				return fmt.Errorf("modulecatalog: prepare database %q: %w", requirement.Name, err)
			}
			if database == nil {
				return fmt.Errorf("modulecatalog: database provider returned nil for %q", requirement.Name)
			}
			resolved[requirement.Name] = database
		}
		preparedCapabilities.Databases = resolved
	}
	if len(requirements.RPC) > 0 {
		resolved := make(rpcMap, len(requirements.RPC))
		for _, requirement := range requirements.RPC {
			connection, err := factory.capabilities.RPC.Connection(requirement.Name)
			if err != nil {
				return fmt.Errorf("modulecatalog: prepare RPC target %q: %w", requirement.Name, err)
			}
			if connection == nil {
				return fmt.Errorf("modulecatalog: RPC provider returned nil for %q", requirement.Name)
			}
			resolved[requirement.Name] = connection
		}
		preparedCapabilities.RPC = resolved
	}
	factory.capabilities = preparedCapabilities
	factory.requirements = requirements
	factory.prepared = true
	return nil
}

func (factory *StaticContextFactory) ForModule(descriptor Descriptor) (BuildContext, error) {
	if factory == nil {
		return nil, fmt.Errorf("modulecatalog: nil static context factory")
	}
	descriptor, err := normalizeDescriptor(descriptor)
	if err != nil {
		return nil, err
	}
	factory.mu.RLock()
	defer factory.mu.RUnlock()
	if !factory.prepared {
		return nil, fmt.Errorf("modulecatalog: static context factory is not prepared")
	}
	if !containsName(factory.requirements.Modules, descriptor.Name) {
		return nil, fmt.Errorf("modulecatalog: module %q was not part of the prepared plan", descriptor.Name)
	}
	capabilities := Capabilities{}
	if descriptor.Requirements.ConfigKey != "" {
		capabilities.Config = restrictedConfigProvider{
			delegate: factory.capabilities.Config,
			module:   descriptor.Name,
			key:      descriptor.Requirements.ConfigKey,
		}
	}
	if descriptor.Requirements.Logger {
		capabilities.Logger = factory.capabilities.Logger
	}
	if descriptor.Requirements.EventBus {
		capabilities.EventBus = factory.capabilities.EventBus
	}
	if len(descriptor.Requirements.Databases) > 0 {
		resolved := make(databaseMap, len(descriptor.Requirements.Databases))
		for _, requirement := range descriptor.Requirements.Databases {
			database, err := factory.capabilities.Databases.GORM(requirement.Name)
			if err != nil {
				return nil, err
			}
			resolved[requirement.Name] = database
		}
		capabilities.Databases = resolved
	}
	if len(descriptor.Requirements.RPC) > 0 {
		resolved := make(rpcMap, len(descriptor.Requirements.RPC))
		for _, requirement := range descriptor.Requirements.RPC {
			connection, err := factory.capabilities.RPC.Connection(requirement.Name)
			if err != nil {
				return nil, err
			}
			resolved[requirement.Name] = connection
		}
		capabilities.RPC = resolved
	}
	return staticBuildContext{descriptor: descriptor, capabilities: capabilities}, nil
}

type restrictedConfigProvider struct {
	delegate ConfigProvider
	module   string
	key      string
}

func (provider restrictedConfigProvider) Decode(moduleName, key string, target any) error {
	if strings.TrimSpace(moduleName) != provider.module || strings.TrimSpace(key) != provider.key {
		return fmt.Errorf("modulecatalog: module %q may only decode configuration key %q", provider.module, provider.key)
	}
	return provider.delegate.Decode(provider.module, provider.key, target)
}

type databaseMap map[string]*gorm.DB

func (databases databaseMap) GORM(name string) (*gorm.DB, error) {
	database, ok := databases[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("modulecatalog: database %q was not prepared", name)
	}
	return database, nil
}

type rpcMap map[string]grpc.ClientConnInterface

func (connections rpcMap) Connection(name string) (grpc.ClientConnInterface, error) {
	connection, ok := connections[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("modulecatalog: RPC target %q was not prepared", name)
	}
	return connection, nil
}

type staticBuildContext struct {
	descriptor   Descriptor
	capabilities Capabilities
}

func (context staticBuildContext) Descriptor() Descriptor      { return cloneDescriptor(context.descriptor) }
func (context staticBuildContext) Config() ConfigProvider      { return context.capabilities.Config }
func (context staticBuildContext) Logger() logExt.Logger       { return context.capabilities.Logger }
func (context staticBuildContext) Databases() DatabaseProvider { return context.capabilities.Databases }
func (context staticBuildContext) EventBus() eventBus.EventBus { return context.capabilities.EventBus }
func (context staticBuildContext) RPC() RPCProvider            { return context.capabilities.RPC }

func normalizeDescriptor(descriptor Descriptor) (Descriptor, error) {
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	descriptor.Version = strings.TrimSpace(descriptor.Version)
	if !moduleNamePattern.MatchString(descriptor.Name) {
		return Descriptor{}, fmt.Errorf("modulecatalog: invalid module name %q", descriptor.Name)
	}
	var err error
	descriptor.DependsOn, err = normalizeNames(descriptor.DependsOn, descriptor.Name, "dependency")
	if err != nil {
		return Descriptor{}, err
	}
	descriptor.Requirements.ConfigKey = strings.TrimSpace(descriptor.Requirements.ConfigKey)
	descriptor.Requirements.Databases, err = normalizeDatabaseRequirements(descriptor.Name, descriptor.Requirements.Databases)
	if err != nil {
		return Descriptor{}, err
	}
	descriptor.Requirements.RPC, err = normalizeRPCRequirements(descriptor.Name, descriptor.Requirements.RPC)
	if err != nil {
		return Descriptor{}, err
	}
	return descriptor, nil
}

func normalizeNames(values []string, owner, kind string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if !moduleNamePattern.MatchString(value) {
			return nil, fmt.Errorf("modulecatalog: module %q invalid %s %q", owner, kind, value)
		}
		if value == owner {
			return nil, fmt.Errorf("modulecatalog: module %q depends on itself", owner)
		}
		if _, duplicate := seen[value]; duplicate {
			return nil, fmt.Errorf("modulecatalog: module %q duplicate %s %q", owner, kind, value)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}

func normalizeDatabaseRequirements(owner string, values []DatabaseRequirement) ([]DatabaseRequirement, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]DatabaseRequirement, 0, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		if !moduleNamePattern.MatchString(value.Name) {
			return nil, fmt.Errorf("modulecatalog: module %q invalid database requirement %q", owner, value.Name)
		}
		if _, duplicate := seen[value.Name]; duplicate {
			return nil, fmt.Errorf("modulecatalog: module %q duplicate database requirement %q", owner, value.Name)
		}
		seen[value.Name] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result, nil
}

func normalizeRPCRequirements(owner string, values []RPCRequirement) ([]RPCRequirement, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]RPCRequirement, 0, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		if !moduleNamePattern.MatchString(value.Name) {
			return nil, fmt.Errorf("modulecatalog: module %q invalid RPC requirement %q", owner, value.Name)
		}
		if _, duplicate := seen[value.Name]; duplicate {
			return nil, fmt.Errorf("modulecatalog: module %q duplicate RPC requirement %q", owner, value.Name)
		}
		seen[value.Name] = struct{}{}
		result = append(result, value)
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Name < result[right].Name })
	return result, nil
}

func containsName(values []string, name string) bool {
	for _, value := range values {
		if value == name {
			return true
		}
	}
	return false
}

func cloneRequirementSet(requirements RequirementSet) RequirementSet {
	clone := requirements
	clone.Modules = append([]string(nil), requirements.Modules...)
	clone.Configs = append([]ConfigRequirement(nil), requirements.Configs...)
	clone.Databases = append([]DatabaseRequirement(nil), requirements.Databases...)
	clone.RPC = append([]RPCRequirement(nil), requirements.RPC...)
	return clone
}

func equalRequirementSets(left, right RequirementSet) bool {
	if left.Logger != right.Logger || left.EventBus != right.EventBus ||
		len(left.Modules) != len(right.Modules) || len(left.Configs) != len(right.Configs) ||
		len(left.Databases) != len(right.Databases) || len(left.RPC) != len(right.RPC) {
		return false
	}
	for index := range left.Modules {
		if left.Modules[index] != right.Modules[index] {
			return false
		}
	}
	for index := range left.Configs {
		if left.Configs[index] != right.Configs[index] {
			return false
		}
	}
	for index := range left.Databases {
		if left.Databases[index] != right.Databases[index] {
			return false
		}
	}
	for index := range left.RPC {
		if left.RPC[index] != right.RPC[index] {
			return false
		}
	}
	return true
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	clone := descriptor
	clone.DependsOn = append([]string(nil), descriptor.DependsOn...)
	clone.Requirements.Databases = append([]DatabaseRequirement(nil), descriptor.Requirements.Databases...)
	clone.Requirements.RPC = append([]RPCRequirement(nil), descriptor.Requirements.RPC...)
	return clone
}
