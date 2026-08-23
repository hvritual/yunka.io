package platform

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"gorm.io/gorm"
	"yunka.io/framework/core/eventBus"
	"yunka.io/framework/core/modulecatalog"
	"yunka.io/pkg/logExt"
)

const DefaultPrepareTimeout = 30 * time.Second

var capabilityNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)

type providerState uint8

const (
	providerStateNew providerState = iota
	providerStatePrepared
	providerStateStarting
	providerStateStarted
	providerStateStopping
	providerStateStopped
	providerStateFailed
)

// Options defines the process-level capability sources owned by one App.
// Modules never receive Options or the factory maps; they receive only the
// restricted BuildContext generated from their Descriptor requirements.
type Options struct {
	Config         modulecatalog.ConfigProvider
	Logger         logExt.Logger
	EventBus       eventBus.EventBus
	Databases      map[string]DatabaseFactory
	RPC            map[string]RPCFactory
	PrepareTimeout time.Duration
}

// Provider is an App-owned modulecatalog.ContextFactory. It opens every named
// required database/RPC capability exactly once, supplies module-scoped views,
// and participates in the existing App Start/Health/Shutdown spine.
type Provider struct {
	mu sync.RWMutex

	config         modulecatalog.ConfigProvider
	logger         logExt.Logger
	eventBus       eventBus.EventBus
	databaseSource map[string]DatabaseFactory
	rpcSource      map[string]RPCFactory
	prepareTimeout time.Duration

	state        providerState
	requirements modulecatalog.RequirementSet
	delegate     modulecatalog.ContextFactory
	resources    []ownedResource
	shutdownErr  error
}

type ownedResource struct {
	key      string
	start    func(context.Context) error
	health   func(context.Context) error
	shutdown func(context.Context) error
}

func New(options Options) (*Provider, error) {
	timeout := options.PrepareTimeout
	if timeout == 0 {
		timeout = DefaultPrepareTimeout
	}
	if timeout < 0 {
		return nil, errors.New("platform: prepare timeout cannot be negative")
	}
	databases, err := copyDatabaseFactories(options.Databases)
	if err != nil {
		return nil, err
	}
	rpc, err := copyRPCFactories(options.RPC)
	if err != nil {
		return nil, err
	}
	return &Provider{
		config:         options.Config,
		logger:         options.Logger,
		eventBus:       options.EventBus,
		databaseSource: databases,
		rpcSource:      rpc,
		prepareTimeout: timeout,
		state:          providerStateNew,
	}, nil
}

func copyDatabaseFactories(source map[string]DatabaseFactory) (map[string]DatabaseFactory, error) {
	result := make(map[string]DatabaseFactory, len(source))
	for name, factory := range source {
		name = strings.TrimSpace(name)
		if !capabilityNamePattern.MatchString(name) {
			return nil, fmt.Errorf("platform: invalid database capability name %q", name)
		}
		if factory == nil {
			return nil, fmt.Errorf("platform: database factory %q is nil", name)
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("platform: duplicate database capability %q", name)
		}
		result[name] = factory
	}
	return result, nil
}

func copyRPCFactories(source map[string]RPCFactory) (map[string]RPCFactory, error) {
	result := make(map[string]RPCFactory, len(source))
	for name, factory := range source {
		name = strings.TrimSpace(name)
		if !capabilityNamePattern.MatchString(name) {
			return nil, fmt.Errorf("platform: invalid RPC capability name %q", name)
		}
		if factory == nil {
			return nil, fmt.Errorf("platform: RPC factory %q is nil", name)
		}
		if _, duplicate := result[name]; duplicate {
			return nil, fmt.Errorf("platform: duplicate RPC capability %q", name)
		}
		result[name] = factory
	}
	return result, nil
}

func (provider *Provider) Config() modulecatalog.ConfigProvider {
	if provider == nil {
		return nil
	}
	return provider.config
}

func (provider *Provider) Logger() logExt.Logger {
	if provider == nil {
		return nil
	}
	return provider.logger
}

func (provider *Provider) EventBus() eventBus.EventBus {
	if provider == nil {
		return nil
	}
	return provider.eventBus
}

func (provider *Provider) Prepare(requirements modulecatalog.RequirementSet) error {
	if provider == nil {
		return errors.New("platform: provider is nil")
	}
	requirements = canonicalRequirements(requirements)

	provider.mu.Lock()
	defer provider.mu.Unlock()

	switch provider.state {
	case providerStatePrepared, providerStateStarting, providerStateStarted:
		if equalRequirements(provider.requirements, requirements) {
			return nil
		}
		return errors.New("platform: provider already prepared for a different requirement set")
	case providerStateStopping, providerStateStopped, providerStateFailed:
		return errors.New("platform: provider cannot prepare after shutdown or failure")
	}
	if len(requirements.Configs) > 0 && provider.config == nil {
		return errors.New("platform: configuration provider is required")
	}
	if requirements.Logger && provider.logger == nil {
		return errors.New("platform: logger is required")
	}
	if requirements.EventBus && provider.eventBus == nil {
		return errors.New("platform: event bus is required")
	}

	prepareContext, cancel := context.WithTimeout(context.Background(), provider.prepareTimeout)
	defer cancel()

	resources := make([]ownedResource, 0, len(requirements.Databases)+len(requirements.RPC))
	databases := make(preparedDatabases, len(requirements.Databases))
	connections := make(preparedRPC, len(requirements.RPC))

	cleanup := func(cause error) error {
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), provider.prepareTimeout)
		defer shutdownCancel()
		return errors.Join(cause, shutdownResources(shutdownContext, resources))
	}

	for _, requirement := range requirements.Databases {
		factory, ok := provider.databaseSource[requirement.Name]
		if !ok {
			return cleanup(fmt.Errorf("platform: required database %q is not configured", requirement.Name))
		}
		resource, err := factory.Open(prepareContext, requirement.Name)
		if err != nil {
			return cleanup(fmt.Errorf("platform: open database %q: %w", requirement.Name, err))
		}
		if resource.Database == nil {
			return cleanup(fmt.Errorf("platform: database factory %q returned nil", requirement.Name))
		}
		databases[requirement.Name] = resource.Database
		resources = append(resources, ownedResource{
			key:      "database:" + requirement.Name,
			start:    resource.StartFunc,
			health:   resource.HealthFunc,
			shutdown: resource.ShutdownFunc,
		})
	}

	for _, requirement := range requirements.RPC {
		factory, ok := provider.rpcSource[requirement.Name]
		if !ok {
			return cleanup(fmt.Errorf("platform: required RPC target %q is not configured", requirement.Name))
		}
		resource, err := factory.Open(prepareContext, requirement.Name)
		if err != nil {
			return cleanup(fmt.Errorf("platform: open RPC target %q: %w", requirement.Name, err))
		}
		if resource.Connection == nil {
			return cleanup(fmt.Errorf("platform: RPC factory %q returned nil", requirement.Name))
		}
		connections[requirement.Name] = resource.Connection
		resources = append(resources, ownedResource{
			key:      "rpc:" + requirement.Name,
			start:    resource.StartFunc,
			health:   resource.HealthFunc,
			shutdown: resource.ShutdownFunc,
		})
	}

	sort.Slice(resources, func(left, right int) bool { return resources[left].key < resources[right].key })
	delegate := modulecatalog.NewStaticContextFactory(modulecatalog.Capabilities{
		Config:    provider.config,
		Logger:    provider.logger,
		Databases: databases,
		EventBus:  provider.eventBus,
		RPC:       connections,
	})
	if err := delegate.Prepare(requirements); err != nil {
		return cleanup(fmt.Errorf("platform: prepare restricted module capabilities: %w", err))
	}

	provider.requirements = requirements
	provider.resources = resources
	provider.delegate = delegate
	provider.state = providerStatePrepared
	return nil
}

func (provider *Provider) ForModule(descriptor modulecatalog.Descriptor) (modulecatalog.BuildContext, error) {
	if provider == nil {
		return nil, errors.New("platform: provider is nil")
	}
	provider.mu.RLock()
	delegate := provider.delegate
	state := provider.state
	provider.mu.RUnlock()
	if state != providerStatePrepared && state != providerStateStarting && state != providerStateStarted {
		return nil, errors.New("platform: provider is not prepared")
	}
	if delegate == nil {
		return nil, errors.New("platform: module capability delegate is unavailable")
	}
	buildContext, err := delegate.ForModule(descriptor)
	if err != nil {
		return nil, err
	}
	if logger := buildContext.Logger(); logger != nil {
		return moduleBuildContext{BuildContext: buildContext, logger: newModuleLogger(logger, descriptor.Name)}, nil
	}
	return buildContext, nil
}

func (provider *Provider) Start(ctx context.Context) error {
	if provider == nil {
		return errors.New("platform: provider is nil")
	}
	ctx = normalizeContext(ctx)
	provider.mu.Lock()
	switch provider.state {
	case providerStateStarted:
		provider.mu.Unlock()
		return nil
	case providerStateNew:
		provider.mu.Unlock()
		return errors.New("platform: provider is not prepared")
	case providerStateStopping, providerStateStopped, providerStateFailed:
		provider.mu.Unlock()
		return errors.New("platform: provider cannot start after shutdown or failure")
	case providerStateStarting:
		provider.mu.Unlock()
		return errors.New("platform: provider start is already in progress")
	}
	provider.state = providerStateStarting
	resources := append([]ownedResource(nil), provider.resources...)
	provider.mu.Unlock()

	for _, resource := range resources {
		if resource.start == nil {
			continue
		}
		if err := safeResourceCall("start "+resource.key, func() error { return resource.start(ctx) }); err != nil {
			cleanupContext, cancel := context.WithTimeout(context.Background(), provider.prepareTimeout)
			cleanupErr := shutdownResources(cleanupContext, resources)
			cancel()
			provider.mu.Lock()
			// Resources have already been closed. Mark the provider stopped so
			// App failure cleanup is idempotent and cannot close them twice.
			provider.state = providerStateStopped
			provider.shutdownErr = cleanupErr
			provider.mu.Unlock()
			return errors.Join(fmt.Errorf("platform: start %s: %w", resource.key, err), cleanupErr)
		}
	}

	provider.mu.Lock()
	provider.state = providerStateStarted
	provider.mu.Unlock()
	return nil
}

func (provider *Provider) Health(ctx context.Context) error {
	if provider == nil {
		return errors.New("platform: provider is nil")
	}
	ctx = normalizeContext(ctx)
	provider.mu.RLock()
	state := provider.state
	resources := append([]ownedResource(nil), provider.resources...)
	provider.mu.RUnlock()
	if state == providerStateNew {
		return errors.New("platform: provider is not prepared")
	}
	if state == providerStateStopping || state == providerStateStopped || state == providerStateFailed {
		return errors.New("platform: provider is not healthy after shutdown or failure")
	}
	var failures []error
	for _, resource := range resources {
		if resource.health == nil {
			continue
		}
		if err := safeResourceCall("health "+resource.key, func() error { return resource.health(ctx) }); err != nil {
			failures = append(failures, fmt.Errorf("platform: health %s: %w", resource.key, err))
		}
	}
	return errors.Join(failures...)
}

func (provider *Provider) Shutdown(ctx context.Context) error {
	if provider == nil {
		return nil
	}
	ctx = normalizeContext(ctx)
	provider.mu.Lock()
	if provider.state == providerStateStopped {
		err := provider.shutdownErr
		provider.mu.Unlock()
		return err
	}
	if provider.state == providerStateStarting {
		provider.mu.Unlock()
		return errors.New("platform: provider start is in progress")
	}
	if provider.state == providerStateStopping {
		provider.mu.Unlock()
		return errors.New("platform: provider shutdown is already in progress")
	}
	provider.state = providerStateStopping
	resources := append([]ownedResource(nil), provider.resources...)
	provider.mu.Unlock()

	err := shutdownResources(ctx, resources)
	provider.mu.Lock()
	provider.state = providerStateStopped
	provider.shutdownErr = err
	provider.mu.Unlock()
	return err
}

func (provider *Provider) Close() error {
	return provider.Shutdown(context.Background())
}

func (provider *Provider) ResourceKeys() []string {
	if provider == nil {
		return nil
	}
	provider.mu.RLock()
	defer provider.mu.RUnlock()
	keys := make([]string, len(provider.resources))
	for index, resource := range provider.resources {
		keys[index] = resource.key
	}
	return keys
}

func shutdownResources(ctx context.Context, resources []ownedResource) error {
	var failures []error
	for index := len(resources) - 1; index >= 0; index-- {
		resource := resources[index]
		if resource.shutdown == nil {
			continue
		}
		if err := safeResourceCall("shutdown "+resource.key, func() error { return resource.shutdown(ctx) }); err != nil {
			failures = append(failures, fmt.Errorf("platform: shutdown %s: %w", resource.key, err))
		}
	}
	return errors.Join(failures...)
}

func safeResourceCall(name string, call func() error) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("%s panicked: %v", name, recovered)
		}
	}()
	return call()
}

type preparedDatabases map[string]*gorm.DB

func (databases preparedDatabases) GORM(name string) (*gorm.DB, error) {
	database, ok := databases[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("platform: database %q was not prepared", name)
	}
	return database, nil
}

type preparedRPC map[string]grpc.ClientConnInterface

func (connections preparedRPC) Connection(name string) (grpc.ClientConnInterface, error) {
	connection, ok := connections[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("platform: RPC target %q was not prepared", name)
	}
	return connection, nil
}

type moduleBuildContext struct {
	modulecatalog.BuildContext
	logger logExt.Logger
}

func (buildContext moduleBuildContext) Logger() logExt.Logger { return buildContext.logger }

func canonicalRequirements(requirements modulecatalog.RequirementSet) modulecatalog.RequirementSet {
	result := requirements
	result.Modules = append([]string(nil), requirements.Modules...)
	result.Configs = append([]modulecatalog.ConfigRequirement(nil), requirements.Configs...)
	result.Databases = append([]modulecatalog.DatabaseRequirement(nil), requirements.Databases...)
	result.RPC = append([]modulecatalog.RPCRequirement(nil), requirements.RPC...)
	sort.Strings(result.Modules)
	sort.Slice(result.Configs, func(left, right int) bool {
		if result.Configs[left].Module != result.Configs[right].Module {
			return result.Configs[left].Module < result.Configs[right].Module
		}
		return result.Configs[left].Key < result.Configs[right].Key
	})
	sort.Slice(result.Databases, func(left, right int) bool { return result.Databases[left].Name < result.Databases[right].Name })
	sort.Slice(result.RPC, func(left, right int) bool { return result.RPC[left].Name < result.RPC[right].Name })
	return result
}

func equalRequirements(left, right modulecatalog.RequirementSet) bool {
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
